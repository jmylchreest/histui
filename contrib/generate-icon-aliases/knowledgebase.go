package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	kbDefaultFile      = "kb-default.toml"       // TOML format for urgency/category defaults
	kbAIAppsFile       = "kb-ai-apps.json"       // AI-generated app mappings for brand icons
	kbAICategoriesFile = "kb-ai-categories.json" // AI-generated app mappings for categories
	minConfidence      = 0.7                     // Minimum confidence to include an app in brand icon mappings
)

// DefaultsConfig represents the kb-default.toml file structure.
type DefaultsConfig struct {
	Meta     DefaultsMeta      `toml:"meta"`
	Symbols  map[string]string `toml:"symbols"`
	GtkIcons map[string]string `toml:"gtk-icons"`
}

// DefaultsMeta contains metadata about the defaults file.
type DefaultsMeta struct {
	Description string `toml:"description"`
	Version     int    `toml:"version"`
}

// LoadDefaultsConfig loads the kb-default.toml file.
func LoadDefaultsConfig(path string) (*DefaultsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not an error, just doesn't exist
		}
		return nil, fmt.Errorf("read defaults file: %w", err)
	}

	var config DefaultsConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse defaults file: %w", err)
	}

	return &config, nil
}

// KnowledgeBase represents the AI-generated knowledge base.
type KnowledgeBase struct {
	Version     int               `json:"version"`
	GeneratedAt string            `json:"generated_at"`
	Model       string            `json:"model"`
	Icons       map[string]KBIcon `json:"icons"`
}

// KBIcon represents a single icon mapping in the knowledge base.
type KBIcon struct {
	Type  string  `json:"type"`  // "app" or "category"
	Glyph string  `json:"glyph"` // e.g., "md-discord"
	Apps  []KBApp `json:"apps"`
}

// KBApp represents an app entry with confidence.
type KBApp struct {
	ID         string  `json:"id"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"` // "package", "desktop", "flatpak", "snap", "inferred"
}

// MergedIcon represents the final merged icon mapping.
type MergedIcon struct {
	Name       string   // canonical name (discord, email, etc.)
	Type       string   // "app" or "category"
	Glyph      string   // matched glyph name (md-discord, etc.)
	Apps       []string // deduplicated app names
	Source     string   // "override", "ai", "manual"
	Confidence float64  // average confidence (for AI-sourced)
}

// LoadKnowledgeBase loads the AI-generated knowledge base if it exists.
func LoadKnowledgeBase(path string) (*KnowledgeBase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not an error, just doesn't exist
		}
		return nil, fmt.Errorf("read KB: %w", err)
	}

	var kb KnowledgeBase
	if err := json.Unmarshal(data, &kb); err != nil {
		return nil, fmt.Errorf("parse KB: %w", err)
	}

	return &kb, nil
}

// SaveKnowledgeBase saves the knowledge base to a file with sorted keys.
func SaveKnowledgeBase(kb *KnowledgeBase, path string) error {
	// Sort icon names for deterministic output
	iconNames := make([]string, 0, len(kb.Icons))
	for name := range kb.Icons {
		iconNames = append(iconNames, name)
	}
	sort.Strings(iconNames)

	// Build ordered JSON manually for deterministic output
	var buf strings.Builder
	buf.WriteString("{\n")
	buf.WriteString(fmt.Sprintf("  \"version\": %d,\n", kb.Version))
	buf.WriteString(fmt.Sprintf("  \"generated_at\": %q,\n", kb.GeneratedAt))
	buf.WriteString(fmt.Sprintf("  \"model\": %q,\n", kb.Model))
	buf.WriteString("  \"icons\": {\n")

	for i, name := range iconNames {
		icon := kb.Icons[name]

		// Sort apps by ID for deterministic output
		sortedApps := make([]KBApp, len(icon.Apps))
		copy(sortedApps, icon.Apps)
		sort.Slice(sortedApps, func(a, b int) bool {
			return sortedApps[a].ID < sortedApps[b].ID
		})

		appsJSON, err := json.Marshal(sortedApps)
		if err != nil {
			return fmt.Errorf("marshal apps for %s: %w", name, err)
		}

		buf.WriteString(fmt.Sprintf("    %q: {\n", name))
		buf.WriteString(fmt.Sprintf("      \"type\": %q,\n", icon.Type))
		buf.WriteString(fmt.Sprintf("      \"glyph\": %q,\n", icon.Glyph))
		buf.WriteString(fmt.Sprintf("      \"apps\": %s\n", appsJSON))
		if i < len(iconNames)-1 {
			buf.WriteString("    },\n")
		} else {
			buf.WriteString("    }\n")
		}
	}

	buf.WriteString("  }\n")
	buf.WriteString("}\n")

	if err := os.WriteFile(path, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("write KB: %w", err)
	}

	return nil
}

// appAssignment tracks which icon an app is assigned to during deduplication.
type appAssignment struct {
	iconName   string
	confidence float64
	isAI       bool // AI KB takes priority over default (default is seed data)
	isManual   bool // Manual overrides take highest priority
}

// DeduplicateApps ensures each app only appears under one icon.
// Priority: AI KB > default KB (AI augments/overrides seed data), then by confidence.
func DeduplicateApps(merged map[string]*MergedIcon, defaultKB, aiKB *KnowledgeBase, verbose bool) {
	// Build a map of app -> best assignment
	bestAssignment := make(map[string]appAssignment)

	// First pass: determine best icon for each app
	for iconName, icon := range merged {
		for _, app := range icon.Apps {
			appLower := strings.ToLower(app)

			// Determine source priority: manual > AI > default
			// For confidence, check defaultKB (categories) first since it has accurate
			// per-app confidences, then fall back to aiKB (brand icons)
			isManual := icon.Source == "manual"
			isAI := false
			confidence := icon.Confidence
			foundConfidence := false

			// Check defaultKB (categories) first for per-app confidence
			if !isManual && defaultKB != nil {
				if kbIcon, ok := defaultKB.Icons[iconName]; ok {
					for _, kbApp := range kbIcon.Apps {
						if strings.ToLower(kbApp.ID) == appLower {
							confidence = kbApp.Confidence
							foundConfidence = true
							break
						}
					}
				}
			}

			// Then check aiKB (brand icons) - these take priority for isAI flag
			if !isManual && aiKB != nil {
				if kbIcon, ok := aiKB.Icons[iconName]; ok {
					for _, kbApp := range kbIcon.Apps {
						if strings.ToLower(kbApp.ID) == appLower {
							isAI = true
							// Only use aiKB confidence if not found in defaultKB
							if !foundConfidence {
								confidence = kbApp.Confidence
							}
							break
						}
					}
				}
			}

			current, exists := bestAssignment[appLower]
			if !exists {
				bestAssignment[appLower] = appAssignment{
					iconName:   iconName,
					confidence: confidence,
					isAI:       isAI,
					isManual:   isManual,
				}
			} else {
				// Compare: manual beats all, then AI beats default, then higher confidence wins
				shouldReplace := false
				if isManual && !current.isManual {
					shouldReplace = true
				} else if !isManual && !current.isManual && isAI && !current.isAI {
					shouldReplace = true
				} else if isManual == current.isManual && isAI == current.isAI && confidence > current.confidence {
					shouldReplace = true
				}
				if shouldReplace {
					bestAssignment[appLower] = appAssignment{
						iconName:   iconName,
						confidence: confidence,
						isAI:       isAI,
						isManual:   isManual,
					}
				}
			}
		}
	}

	// Second pass: remove apps that should be assigned to a different icon
	removedCount := 0
	for iconName, icon := range merged {
		var filteredApps []string
		for _, app := range icon.Apps {
			appLower := strings.ToLower(app)
			if best, ok := bestAssignment[appLower]; ok && best.iconName == iconName {
				filteredApps = append(filteredApps, app)
			} else {
				removedCount++
				if verbose {
					fmt.Printf("  Dedup: %s removed from %s (assigned to %s)\n",
						app, iconName, bestAssignment[appLower].iconName)
				}
			}
		}
		icon.Apps = filteredApps
	}

	if verbose && removedCount > 0 {
		fmt.Printf("Deduplication: removed %d duplicate app assignments\n", removedCount)
	}
}

// MergeIconSources merges icons from all sources with priority: AI > default.
// Returns a map of icon name -> MergedIcon.
func MergeIconSources(defaultKB, aiKB *KnowledgeBase, verbose bool) map[string]*MergedIcon {
	result := make(map[string]*MergedIcon)

	// Step 1: Start with default KB (lowest priority)
	if defaultKB != nil {
		for iconName, kbIcon := range defaultKB.Icons {
			apps := extractApps(kbIcon, minConfidence)
			if len(apps) == 0 {
				continue
			}

			result[iconName] = &MergedIcon{
				Name:   iconName,
				Type:   kbIcon.Type,
				Glyph:  kbIcon.Glyph,
				Apps:   apps,
				Source: "default",
			}
		}
	}

	// Step 2: Layer AI knowledge base (higher priority)
	if aiKB != nil {
		for iconName, kbIcon := range aiKB.Icons {
			apps := extractApps(kbIcon, minConfidence)
			if len(apps) == 0 {
				continue
			}

			avgConf := avgConfidence(kbIcon, minConfidence)

			if existing, ok := result[iconName]; ok {
				// Merge with default: AI takes precedence but we keep default apps not in AI
				existing.Apps = mergeApps(apps, existing.Apps)
				existing.Type = kbIcon.Type
				existing.Source = "ai+default"
				existing.Confidence = avgConf
				if kbIcon.Glyph != "" {
					existing.Glyph = kbIcon.Glyph
				}
			} else {
				result[iconName] = &MergedIcon{
					Name:       iconName,
					Type:       kbIcon.Type,
					Glyph:      kbIcon.Glyph,
					Apps:       apps,
					Source:     "ai",
					Confidence: avgConf,
				}
			}
		}
	}

	if verbose {
		// Print merge stats
		var defaultCount, aiCount, mixedCount int
		for _, icon := range result {
			switch icon.Source {
			case "default":
				defaultCount++
			case "ai":
				aiCount++
			default:
				mixedCount++
			}
		}
		fmt.Printf("Merge stats: default=%d, ai=%d, mixed=%d\n",
			defaultCount, aiCount, mixedCount)
	}

	return result
}

// extractApps extracts app IDs from a KBIcon, filtering by minimum confidence.
func extractApps(icon KBIcon, minConf float64) []string {
	var apps []string
	for _, app := range icon.Apps {
		if app.Confidence >= minConf {
			apps = append(apps, app.ID)
		}
	}
	return apps
}

// avgConfidence calculates average confidence for apps above the threshold.
func avgConfidence(icon KBIcon, minConf float64) float64 {
	var total float64
	var count int
	for _, app := range icon.Apps {
		if app.Confidence >= minConf {
			total += app.Confidence
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

// ConvertMergedToAppMapping converts merged icons to AppMapping for the existing generator.
// Only includes app-type icons (brand icons), not category icons.
func ConvertMergedToAppMapping(merged map[string]*MergedIcon, glyphs map[string]GlyphInfo, verbose bool) []AppMapping {
	mappings := make([]AppMapping, 0, len(merged))

	for iconName, icon := range merged {
		if len(icon.Apps) == 0 {
			continue
		}

		// Only include app-type icons (brand icons), not category icons
		// Category icons are handled separately in writeTOML
		if icon.Type == "category" {
			continue
		}

		// Find glyph (use pre-set glyph or find via existing logic)
		glyphName := icon.Glyph
		var glyphInfo GlyphInfo
		var found bool

		if glyphName != "" {
			glyphInfo, found = glyphs[glyphName]
		}

		// If no pre-set glyph or not found, use existing matching logic
		if !found {
			// Check explicit overrides first
			if override, ok := explicitIconOverrides[iconName]; ok {
				if g, ok := glyphs[override]; ok {
					glyphName = override
					glyphInfo = g
					found = true
				}
			}
		}

		// Try prefix matching
		if !found {
			prefixes := getPrefixOrder()
			for _, prefix := range prefixes {
				testName := prefix + iconName
				if g, ok := glyphs[testName]; ok {
					glyphName = testName
					glyphInfo = g
					found = true
					break
				}
				// Try with underscores
				testName = prefix + replaceHyphensWithUnderscores(iconName)
				if g, ok := glyphs[testName]; ok {
					glyphName = testName
					glyphInfo = g
					found = true
					break
				}
			}
		}

		if !found {
			if verbose {
				fmt.Printf("  %s -> NO MATCH\n", iconName)
			}
			continue
		}

		mapping := AppMapping{
			AppNames:   icon.Apps,
			IconName:   glyphName,
			GlyphCode:  glyphInfo.Code,
			GlyphChar:  glyphInfo.Char,
			Confidence: icon.Source,
		}
		mappings = append(mappings, mapping)

		if verbose {
			fmt.Printf("  %s -> %s (U+%s) [%s]\n", iconName, glyphName, glyphInfo.Code, icon.Source)
		}
	}

	// Sort by icon name for consistent output
	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].IconName < mappings[j].IconName
	})

	return mappings
}

// Helper functions

func mergeApps(primary, secondary []string) []string {
	seen := make(map[string]bool)
	var result []string

	// Primary apps first
	for _, app := range primary {
		if !seen[app] {
			seen[app] = true
			result = append(result, app)
		}
	}

	// Add secondary apps not in primary
	for _, app := range secondary {
		if !seen[app] {
			seen[app] = true
			result = append(result, app)
		}
	}

	return result
}

func replaceHyphensWithUnderscores(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			result[i] = '_'
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}

// ApplyManualForceApps applies force_apps from manual patterns to the merged icons.
// This ensures manual overrides take highest priority over AI-generated mappings.
func ApplyManualForceApps(merged map[string]*MergedIcon, patterns *PatternConfig, glyphs map[string]GlyphInfo, verbose bool) {
	if patterns == nil {
		return
	}

	appliedCount := 0
	for iconName, pattern := range patterns.Icons {
		if len(pattern.ForceApps) == 0 {
			continue
		}

		// Find or create the merged icon
		icon, exists := merged[iconName]
		if !exists {
			// Create new merged icon for manual-only entries
			// Find glyph from pattern
			glyphName := ""
			for _, p := range pattern.Patterns {
				if _, ok := glyphs[p]; ok {
					glyphName = p
					break
				}
			}

			icon = &MergedIcon{
				Name:   iconName,
				Type:   pattern.Type,
				Glyph:  glyphName,
				Apps:   pattern.ForceApps,
				Source: "manual",
			}
			merged[iconName] = icon
		} else {
			// Override existing apps with force_apps
			icon.Apps = pattern.ForceApps
			icon.Source = "manual"
		}

		appliedCount++
		if verbose {
			fmt.Printf("Applied manual force_apps for %s: %v\n", iconName, pattern.ForceApps)
		}
	}

	if appliedCount > 0 {
		fmt.Printf("Applied %d manual force_apps overrides\n", appliedCount)
	}
}
