package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/BurntSushi/toml"
)

const (
	kbAIFile       = "kb-ai.json"
	kbOverridesFile = "kb-overrides.toml"
	minConfidence  = 0.3 // Minimum confidence to include an app
)

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

// Overrides represents user-defined overrides.
type Overrides struct {
	// Icons maps icon name to list of app names (replaces AI/manual list entirely)
	Icons map[string][]string `toml:"icons"`
	// Additions maps icon name to additional app names (merged with existing)
	Additions map[string][]string `toml:"additions"`
	// Exclusions maps icon name to app names to exclude
	Exclusions map[string][]string `toml:"exclusions"`
	// GlyphOverrides maps icon name to specific glyph (overrides auto-match)
	GlyphOverrides map[string]string `toml:"glyph_overrides"`
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

// SaveKnowledgeBase saves the knowledge base to a file.
func SaveKnowledgeBase(kb *KnowledgeBase, path string) error {
	data, err := json.MarshalIndent(kb, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KB: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write KB: %w", err)
	}

	return nil
}

// LoadOverrides loads user-defined overrides if the file exists.
func LoadOverrides(path string) (*Overrides, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not an error, just doesn't exist
		}
		return nil, fmt.Errorf("read overrides: %w", err)
	}

	var overrides Overrides
	if _, err := toml.Decode(string(data), &overrides); err != nil {
		return nil, fmt.Errorf("parse overrides: %w", err)
	}

	return &overrides, nil
}

// MergeIconSources merges icons from all three sources with priority: override > AI > manual.
// Returns a map of icon name -> MergedIcon.
func MergeIconSources(kb *KnowledgeBase, manual map[string][]string, overrides *Overrides, verbose bool) map[string]*MergedIcon {
	result := make(map[string]*MergedIcon)

	// Step 1: Start with manual mappings (lowest priority)
	for iconName, apps := range manual {
		result[iconName] = &MergedIcon{
			Name:   iconName,
			Type:   "app", // default, can be overridden
			Apps:   dedupe(apps),
			Source: "manual",
		}
	}

	// Step 2: Layer AI knowledge base (medium priority)
	if kb != nil {
		for iconName, kbIcon := range kb.Icons {
			// Filter apps by confidence
			var apps []string
			var totalConf float64
			for _, app := range kbIcon.Apps {
				if app.Confidence >= minConfidence {
					apps = append(apps, app.ID)
					totalConf += app.Confidence
				}
			}

			if len(apps) == 0 {
				continue
			}

			avgConf := totalConf / float64(len(apps))

			if existing, ok := result[iconName]; ok {
				// Merge with manual: AI takes precedence but we keep manual apps not in AI
				existing.Apps = mergeApps(apps, existing.Apps)
				existing.Type = kbIcon.Type
				existing.Source = "ai+manual"
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

	// Step 3: Apply overrides (highest priority)
	if overrides != nil {
		// Full replacements
		for iconName, apps := range overrides.Icons {
			if existing, ok := result[iconName]; ok {
				existing.Apps = dedupe(apps)
				existing.Source = "override"
			} else {
				result[iconName] = &MergedIcon{
					Name:   iconName,
					Type:   "app",
					Apps:   dedupe(apps),
					Source: "override",
				}
			}
		}

		// Additions (merge with existing)
		for iconName, apps := range overrides.Additions {
			if existing, ok := result[iconName]; ok {
				existing.Apps = mergeApps(existing.Apps, apps)
				if existing.Source != "override" {
					existing.Source = existing.Source + "+override"
				}
			} else {
				result[iconName] = &MergedIcon{
					Name:   iconName,
					Type:   "app",
					Apps:   dedupe(apps),
					Source: "override",
				}
			}
		}

		// Exclusions (remove specific apps)
		for iconName, excludeApps := range overrides.Exclusions {
			if existing, ok := result[iconName]; ok {
				existing.Apps = excludeFrom(existing.Apps, excludeApps)
			}
		}

		// Glyph overrides
		for iconName, glyph := range overrides.GlyphOverrides {
			if existing, ok := result[iconName]; ok {
				existing.Glyph = glyph
			}
		}
	}

	if verbose {
		// Print merge stats
		var manualCount, aiCount, overrideCount, mixedCount int
		for _, icon := range result {
			switch icon.Source {
			case "manual":
				manualCount++
			case "ai":
				aiCount++
			case "override":
				overrideCount++
			default:
				mixedCount++
			}
		}
		fmt.Printf("Merge stats: manual=%d, ai=%d, override=%d, mixed=%d\n",
			manualCount, aiCount, overrideCount, mixedCount)
	}

	return result
}

// ConvertMergedToAppMapping converts merged icons to AppMapping for the existing generator.
func ConvertMergedToAppMapping(merged map[string]*MergedIcon, glyphs map[string]GlyphInfo, verbose bool) []AppMapping {
	var mappings []AppMapping

	for iconName, icon := range merged {
		if len(icon.Apps) == 0 {
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

func dedupe(apps []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, app := range apps {
		if !seen[app] {
			seen[app] = true
			result = append(result, app)
		}
	}
	return result
}

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

func excludeFrom(apps, exclusions []string) []string {
	exclude := make(map[string]bool)
	for _, e := range exclusions {
		exclude[e] = true
	}

	var result []string
	for _, app := range apps {
		if !exclude[app] {
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

// WriteExampleOverrides writes an example overrides file.
func WriteExampleOverrides(path string) error {
	example := `# Icon Aliases Overrides
# This file allows you to customize icon mappings.
# Priority: overrides > AI knowledge base > manual mappings

# Full replacements - completely replace the app list for an icon
[icons]
# discord = ["my-custom-discord-client"]

# Additions - add apps to existing mappings
[additions]
# email = ["my-email-client", "another-mailer"]

# Exclusions - remove specific apps from mappings
[exclusions]
# email = ["thunderbird"]  # Don't map thunderbird to email icon

# Glyph overrides - force a specific Nerd Font glyph
[glyph_overrides]
# discord = "fa-discord"  # Use Font Awesome instead of Material Design
`
	return os.WriteFile(path, []byte(example), 0644)
}
