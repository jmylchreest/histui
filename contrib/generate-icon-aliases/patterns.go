package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// PatternConfig represents the kb-patterns.toml file structure.
type PatternConfig struct {
	Meta  PatternMeta               `toml:"meta"`
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
	CanonicalName string   // e.g., "discord", "email"
	Type          string   // "app" or "category"
	Description   string
	GlyphName     string   // e.g., "md-discord"
	GlyphCode     string   // e.g., "F066F"
	GlyphChar     string   // The actual character
	SearchTerms   []string // Terms from upstream metadata
	ForceApps     []string // Force these apps (skips AI generation)
	ExtraApps     []string // Add these apps to AI-generated list
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

	// Merge: manual entries completely replace auto entries
	for name, icon := range manual.Icons {
		auto.Icons[name] = icon
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

				// Check if pattern matches
				var matches bool
				if strings.HasSuffix(pattern, "*") {
					// Prefix match
					prefix := strings.TrimSuffix(patternLower, "*")
					matches = strings.HasPrefix(glyphLower, prefix)
				} else {
					// Substring match
					matches = strings.Contains(glyphLower, patternLower)
				}

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

// GetIconsForAppGeneration returns a list of icons suitable for AI app generation.
// Sorted by canonical name for deterministic output.
func GetIconsForAppGeneration(matched map[string]*MatchedIcon) []struct{ Name, Type, Description string } {
	var icons []struct{ Name, Type, Description string }

	for name, icon := range matched {
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
