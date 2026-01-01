package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// PatternConfig represents the kb-patterns.toml file structure.
type PatternConfig struct {
	Meta  PatternMeta            `toml:"meta"`
	Icons map[string]IconPattern `toml:"icons"`
}

// PatternMeta contains metadata about the patterns file.
type PatternMeta struct {
	Generated   string   `toml:"generated"`
	Sources     []string `toml:"sources"`
	Description string   `toml:"description"`
}

// IconPattern defines patterns for matching glyphs to a canonical icon.
type IconPattern struct {
	Patterns    []string `toml:"patterns"`
	SearchTerms []string `toml:"search_terms,omitempty"` // Terms from upstream metadata
	Type        string   `toml:"type"`                   // "app" or "category"
	Description string   `toml:"description"`            // Human-readable description
	Upstream    string   `toml:"upstream,omitempty"`     // Source: "fa", "md", "dev", "cod", "manual"
	Priority    int      `toml:"priority"`               // Higher = prefer this glyph (default: 5)
	// Manual override fields
	ForceApps []string `toml:"force_apps,omitempty"` // Force these apps (skips AI)
	ExtraApps []string `toml:"extra_apps,omitempty"` // Add these apps to AI list
}

// MatchedIcon represents a glyph matched to a canonical icon.
type MatchedIcon struct {
	CanonicalName string // e.g., "discord", "email"
	Type          string // "app" or "category"
	Description   string
	GlyphName     string   // e.g., "md-discord"
	GlyphCode     string   // e.g., "F066F"
	GlyphChar     string   // The actual character
	SearchTerms   []string // Terms from upstream metadata
	ForceApps     []string // Force these apps (skips AI generation)
	ExtraApps     []string // Add these apps to AI-generated list
}

// ExtraApp represents a detailed app entry with context for AI research.
type ExtraApp struct {
	Name        string `toml:"name"`
	Description string `toml:"description,omitempty"`
	URL         string `toml:"url,omitempty"`
}

// ExtraAppsConfig represents the extra-apps.toml file structure.
type ExtraAppsConfig struct {
	Apps struct {
		Include  []string   `toml:"include"`
		Detailed []ExtraApp `toml:"detailed,omitempty"`
	} `toml:"apps"`
}

// FormatForPrompt returns a formatted string for inclusion in the AI prompt.
func (c *ExtraAppsConfig) FormatForPrompt() []string {
	result := make([]string, 0, len(c.Apps.Include)+len(c.Apps.Detailed))

	// Simple includes
	for _, app := range c.Apps.Include {
		if app != "" {
			result = append(result, app)
		}
	}

	// Detailed entries with context
	for _, app := range c.Apps.Detailed {
		entry := app.Name
		if app.Description != "" {
			entry += " - " + app.Description
		}
		if app.URL != "" {
			entry += " (see: " + app.URL + ")"
		}
		result = append(result, entry)
	}

	return result
}

// LoadExtraApps loads the extra apps configuration from a TOML file.
func LoadExtraApps(path string) (*ExtraAppsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No extra apps file - return empty config
			return &ExtraAppsConfig{}, nil
		}
		return nil, fmt.Errorf("read extra apps file: %w", err)
	}

	var config ExtraAppsConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse extra apps file: %w", err)
	}

	return &config, nil
}

// LoadPatterns loads the pattern configuration from a TOML file.
func LoadPatterns(path string) (*PatternConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read patterns file: %w", err)
	}

	var config PatternConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse patterns file: %w", err)
	}

	if config.Icons == nil {
		config.Icons = make(map[string]IconPattern)
	}

	// Set default priority for icons that don't specify one
	for name, icon := range config.Icons {
		if icon.Priority == 0 {
			icon.Priority = 5
		}
		config.Icons[name] = icon
	}

	return &config, nil
}

// LoadPatternsWithManual loads kb-patterns.toml and merges kb-patterns-manual.toml on top.
// Manual entries completely override auto-generated ones for the same name.
func LoadPatternsWithManual(autoPath, manualPath string) (*PatternConfig, error) {
	// Load auto-generated patterns
	auto, err := LoadPatterns(autoPath)
	if err != nil {
		// If auto file doesn't exist, start with empty
		if os.IsNotExist(err) {
			auto = &PatternConfig{Icons: make(map[string]IconPattern)}
		} else {
			return nil, fmt.Errorf("load auto patterns: %w", err)
		}
	}

	// Load manual overrides (optional)
	manual, err := LoadPatterns(manualPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No manual file - just return auto
			return auto, nil
		}
		return nil, fmt.Errorf("load manual patterns: %w", err)
	}

	// Merge: manual entries override specific fields, not entire entries
	for name, manualIcon := range manual.Icons {
		if autoIcon, exists := auto.Icons[name]; exists {
			// Merge fields: manual overrides auto for non-empty fields
			if len(manualIcon.Patterns) > 0 {
				autoIcon.Patterns = manualIcon.Patterns
			}
			if len(manualIcon.SearchTerms) > 0 {
				autoIcon.SearchTerms = manualIcon.SearchTerms
			}
			if manualIcon.Type != "" {
				autoIcon.Type = manualIcon.Type
			}
			if manualIcon.Description != "" {
				autoIcon.Description = manualIcon.Description
			}
			if manualIcon.Upstream != "" {
				autoIcon.Upstream = manualIcon.Upstream
			}
			if manualIcon.Priority != 0 {
				autoIcon.Priority = manualIcon.Priority
			}
			// ForceApps replaces entirely (intentional override)
			if len(manualIcon.ForceApps) > 0 {
				autoIcon.ForceApps = manualIcon.ForceApps
			}
			// ExtraApps appends to existing
			if len(manualIcon.ExtraApps) > 0 {
				autoIcon.ExtraApps = append(autoIcon.ExtraApps, manualIcon.ExtraApps...)
			}
			auto.Icons[name] = autoIcon
		} else {
			// New entry from manual file
			auto.Icons[name] = manualIcon
		}
	}

	return auto, nil
}

// MatchGlyphsToIcons matches glyphs to canonical icons using patterns.
// Returns a map of canonical icon name -> best matched glyph.
func MatchGlyphsToIcons(patterns *PatternConfig, glyphs map[string]GlyphInfo, verbose bool) map[string]*MatchedIcon {
	result := make(map[string]*MatchedIcon)

	// For each canonical icon, find the best matching glyph
	for iconName, iconPattern := range patterns.Icons {
		var bestMatch *MatchedIcon
		var bestPriority int

		// Try each pattern for this icon
		for _, pattern := range iconPattern.Patterns {
			patternLower := strings.ToLower(pattern)

			// Find glyphs matching this pattern
			for glyphName, glyphInfo := range glyphs {
				glyphLower := strings.ToLower(glyphName)

				// Check if pattern matches using separator-agnostic matching
				matches := matchGlyphPattern(patternLower, glyphLower)

				if matches {
					// Calculate priority based on match quality
					priority := iconPattern.Priority

					// Prefer exact matches over substring matches
					if glyphLower == patternLower || strings.HasSuffix(glyphLower, "-"+patternLower) {
						priority += 10
					}

					// Prefer shorter glyph names (less specific variants)
					priority -= len(glyphName) / 20

					// Prefer certain prefixes (md- is usually best)
					if strings.HasPrefix(glyphLower, "md-") {
						priority += 3
					} else if strings.HasPrefix(glyphLower, "fa-") {
						priority += 2
					} else if strings.HasPrefix(glyphLower, "dev-") {
						priority += 1
					}

					if bestMatch == nil || priority > bestPriority {
						bestMatch = &MatchedIcon{
							CanonicalName: iconName,
							Type:          iconPattern.Type,
							Description:   iconPattern.Description,
							GlyphName:     glyphName,
							GlyphCode:     glyphInfo.Code,
							GlyphChar:     glyphInfo.Char,
							SearchTerms:   iconPattern.SearchTerms,
							ForceApps:     iconPattern.ForceApps,
							ExtraApps:     iconPattern.ExtraApps,
						}
						bestPriority = priority
					}
				}
			}
		}

		if bestMatch != nil {
			result[iconName] = bestMatch
			if verbose {
				fmt.Printf("  %s -> %s (%s)\n", iconName, bestMatch.GlyphName, bestMatch.GlyphCode)
			}
		} else if verbose {
			fmt.Printf("  %s -> NO MATCH\n", iconName)
		}
	}

	return result
}

// GetIconsForAppGeneration returns a list of app-type icons suitable for AI app generation.
// Only includes icons with Type == "app" (brand icons, not category icons).
// Sorted by canonical name for deterministic output.
func GetIconsForAppGeneration(matched map[string]*MatchedIcon) []struct{ Name, Type, Description string } {
	icons := make([]struct{ Name, Type, Description string }, 0, len(matched))

	for name, icon := range matched {
		// Only include app-type icons (brand icons), not category icons
		if icon.Type != "app" {
			continue
		}
		icons = append(icons, struct{ Name, Type, Description string }{
			Name:        name,
			Type:        icon.Type,
			Description: icon.Description,
		})
	}

	// Sort by name for deterministic batching
	sort.Slice(icons, func(i, j int) bool {
		return icons[i].Name < icons[j].Name
	})

	return icons
}

// BuildSymbolsMap creates a map of canonical icon name -> glyph info for output.
func BuildSymbolsMap(matched map[string]*MatchedIcon) map[string]GlyphInfo {
	result := make(map[string]GlyphInfo)

	for iconName, icon := range matched {
		result[iconName] = GlyphInfo{
			Code: icon.GlyphCode,
			Char: icon.GlyphChar,
		}
	}

	return result
}

// UnmatchedIcon represents an icon pattern that couldn't be matched to a glyph.
type UnmatchedIcon struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Patterns    []string `json:"patterns"`
}

// FindUnmatchedIcons finds icons defined in patterns that have no matching glyph.
func FindUnmatchedIcons(patterns *PatternConfig, matched map[string]*MatchedIcon) []UnmatchedIcon {
	var unmatched []UnmatchedIcon

	for name, pattern := range patterns.Icons {
		if _, found := matched[name]; !found {
			unmatched = append(unmatched, UnmatchedIcon{
				Name:        name,
				Type:        pattern.Type,
				Description: pattern.Description,
				Patterns:    pattern.Patterns,
			})
		}
	}

	// Sort for deterministic output
	sort.Slice(unmatched, func(i, j int) bool {
		return unmatched[i].Name < unmatched[j].Name
	})

	return unmatched
}

// BuildGlyphMetadataForSuggestions builds metadata for the category suggestion prompt.
// It filters to "category" type icons only (not brand-specific app icons) and includes
// the description and search terms from the patterns.
func BuildGlyphMetadataForSuggestions(patterns *PatternConfig, glyphs map[string]GlyphInfo) []GlyphMetadata {
	result := make([]GlyphMetadata, 0, len(patterns.Icons))

	// Collect all category-type icons with their matched glyphs
	for iconName, pattern := range patterns.Icons {
		// Only include category-type icons
		if pattern.Type != "category" {
			continue
		}

		// Find the best matching glyph for this pattern
		var bestGlyph string
		var bestGlyphInfo GlyphInfo
		found := false

		for _, p := range pattern.Patterns {
			pLower := strings.ToLower(p)
			for glyphName, info := range glyphs {
				glyphLower := strings.ToLower(glyphName)

				var matches bool
				if strings.HasSuffix(p, "*") {
					prefix := strings.TrimSuffix(pLower, "*")
					matches = strings.HasPrefix(glyphLower, prefix)
				} else {
					matches = strings.Contains(glyphLower, pLower)
				}

				if matches {
					bestGlyph = glyphName
					bestGlyphInfo = info
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			continue
		}

		// Extract prefix from glyph name (e.g., "md-chat" -> "md")
		prefix := ""
		if idx := strings.Index(bestGlyph, "-"); idx > 0 {
			prefix = bestGlyph[:idx]
		}

		result = append(result, GlyphMetadata{
			Name:        iconName,
			Prefix:      prefix,
			Symbol:      bestGlyphInfo.Char,
			Description: pattern.Description,
			Tags:        pattern.SearchTerms,
		})
	}

	// Sort by name for consistent output
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// BuildAllGlyphMetadata builds metadata for ALL glyphs (not just those with patterns).
// This is useful for letting AI discover new category icons from the full glyph set.
func BuildAllGlyphMetadata(glyphs map[string]GlyphInfo, preferredPrefixes []string) []GlyphMetadata {
	result := make([]GlyphMetadata, 0, len(glyphs))

	// Create a set of preferred prefixes for quick lookup
	prefixOrder := make(map[string]int)
	for i, p := range preferredPrefixes {
		prefixOrder[p] = i
	}

	for glyphName, info := range glyphs {
		// Extract prefix
		prefix := ""
		name := glyphName
		if idx := strings.Index(glyphName, "-"); idx > 0 {
			prefix = glyphName[:idx]
			name = glyphName[idx+1:]
		}

		// Skip glyphs that look like brand/app icons (heuristic)
		// These usually have very specific names like "discord", "spotify", etc.
		// We want generic category icons like "chat", "email", "music"
		if isLikelyBrandIcon(name) {
			continue
		}

		result = append(result, GlyphMetadata{
			Name:        name,
			Prefix:      prefix,
			Symbol:      info.Char,
			Description: "", // No description available for raw glyphs
			Tags:        nil,
		})
	}

	// Sort by preferred prefix order, then by name
	sort.Slice(result, func(i, j int) bool {
		iOrder, iOk := prefixOrder[result[i].Prefix]
		jOrder, jOk := prefixOrder[result[j].Prefix]

		// Preferred prefixes come first
		if iOk && !jOk {
			return true
		}
		if !iOk && jOk {
			return false
		}
		if iOk && jOk && iOrder != jOrder {
			return iOrder < jOrder
		}

		// Then sort by name
		return result[i].Name < result[j].Name
	})

	return result
}

// isLikelyBrandIcon uses heuristics to detect brand-specific icons.
func isLikelyBrandIcon(name string) bool {
	// Known brand patterns (partial list - AI will filter further)
	brandPatterns := []string{
		"discord", "spotify", "slack", "twitter", "facebook", "instagram",
		"google", "apple", "microsoft", "amazon", "netflix", "youtube",
		"github", "gitlab", "bitbucket", "docker", "kubernetes",
		"firefox", "chrome", "safari", "edge", "opera",
		"android", "ios", "windows", "linux", "ubuntu", "fedora", "debian",
	}

	nameLower := strings.ToLower(name)
	for _, brand := range brandPatterns {
		if strings.Contains(nameLower, brand) {
			return true
		}
	}

	return false
}

// matchGlyphPattern matches a pattern against a glyph name, treating separators
// (hyphens, underscores) as equivalent. This handles naming differences between
// upstream icon sources (Font Awesome uses hyphens) and Nerd Fonts (uses underscores).
//
// Examples:
//   - "fa-shield-halved" matches "fa-shield_halved"
//   - "md-discord" matches "md_discord"
//   - "nf-fa-user" matches "nf_fa_user"
func matchGlyphPattern(pattern, glyphName string) bool {
	// Convert both to use a common separator for comparison
	patternNorm := normalizeSeparators(pattern)
	glyphNorm := normalizeSeparators(glyphName)

	// Handle wildcard patterns (e.g., "md-discord*")
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(patternNorm, "*")
		return strings.HasPrefix(glyphNorm, prefix)
	}

	// Check for exact match or substring match
	return glyphNorm == patternNorm || strings.Contains(glyphNorm, patternNorm)
}

// normalizeSeparators replaces all separator characters (hyphens, underscores)
// with a common separator for comparison purposes.
func normalizeSeparators(s string) string {
	// Replace both hyphens and underscores with underscores
	return strings.ReplaceAll(s, "-", "_")
}
