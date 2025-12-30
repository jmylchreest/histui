// generate-icon-aliases parses Nerd Fonts glyph data and generates icon mappings.
//
// Usage:
//
//	go run ./contrib/generate-icon-aliases [--fetch] [--output aliases.toml]
//
// This tool:
//   - Downloads/reads glyphnames.json from Nerd Fonts
//   - Filters for app-related icons (discord, firefox, etc.)
//   - Maps common Linux application names to Nerd Font symbols
//   - Generates icon-aliases.toml for embedding
//   - Logs all matches for review
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	glyphNamesURL    = "https://raw.githubusercontent.com/ryanoasis/nerd-fonts/master/glyphnames.json"
	cacheFile        = "glyphnames.json"
	nerdFontURL      = "https://github.com/ryanoasis/nerd-fonts/raw/master/patched-fonts/NerdFontsSymbolsOnly/SymbolsNerdFont-Regular.ttf"
	nerdFontCacheFile = "SymbolsNerdFont-Regular.ttf"
)

// iconPreference is the preferred icon set (md, fa, dev)
var iconPreference = "md"

// getPrefixOrder returns icon set prefixes in order of preference.
func getPrefixOrder() []string {
	switch iconPreference {
	case "fa":
		return []string{"fa-", "md-", "dev-", "custom-", "linux-", "cod-"}
	case "dev":
		return []string{"dev-", "md-", "fa-", "custom-", "linux-", "cod-"}
	default: // "md" or anything else
		return []string{"md-", "fa-", "dev-", "custom-", "linux-", "cod-"}
	}
}

// GlyphInfo represents a single glyph entry from Nerd Fonts
type GlyphInfo struct {
	Char string `json:"char"`
	Code string `json:"code"`
}

// AppMapping represents a mapping from Linux app names to icon info
type AppMapping struct {
	AppNames   []string // Linux application names (package names, desktop file names)
	IconName   string   // Nerd Font icon name (e.g., "nf-md-discord")
	GlyphCode  string   // Unicode codepoint (e.g., "f066f")
	GlyphChar  string   // Actual unicode character
	Confidence string   // "exact", "likely", "possible"
}

// explicitIconOverrides maps icon keys to specific glyph names when the auto-match fails or is wrong
var explicitIconOverrides = map[string]string{
	"tor-browser":        "linux-tor",              // Use Linux Tor icon instead of fuzzy match
	"message":            "md-chat",                // Chat icon for Matrix clients (Element, Fractal, etc.)
	"monitor-screenshot": "md-monitor_screenshot",  // Correct screenshot icon
	"min":                "cod-browser",            // Generic browser icon
	"desktop-classic":    "cod-desktop_download",   // VM/desktop download icon
	"image":              "md-file_image",          // Simple file image icon
	"brave":              "md-shield_half_full",    // Shield for Brave browser
}

func main() {
	// Primary workflow flags
	fetchFlag := flag.Bool("fetch", false, "Fetch upstream metadata and regenerate kb-patterns.toml (also refreshes Nerd Font data)")
	outputFlag := flag.String("output", "icon-aliases.toml", "Output TOML file path")
	fontOutputFlag := flag.String("font-output", "", "Output path for Nerd Font symbols TTF (optional)")
	verboseFlag := flag.Bool("verbose", false, "Verbose logging")
	preferFlag := flag.String("prefer", "md", "Preferred icon set: md (Material Design), fa (Font Awesome), dev (Devicons)")

	// Knowledge base flags
	generateKBFlag := flag.Bool("generate-kb", false, "Generate AI app mappings using OpenRouter (requires OPENROUTER_API_KEY)")
	openrouterModelFlag := flag.String("openrouter-model", "", "OpenRouter model to use (from config or anthropic/claude-sonnet-4)")
	webSearchFlag := flag.Bool("web-search", false, "Enable web search for real-time data (adds :online suffix)")
	noCacheFlag := flag.Bool("no-cache", false, "Disable caching of API responses")
	clearCacheFlag := flag.Bool("clear-cache", false, "Clear the API response cache before generating")
	patternsFileFlag := flag.String("patterns-file", kbPatternsFile, "Path to auto-generated patterns file")
	manualPatternsFileFlag := flag.String("manual-patterns-file", kbPatternsManualFile, "Path to manual patterns override file")
	defaultFileFlag := flag.String("default-file", kbDefaultFile, "Path to default knowledge base file")
	aiFileFlag := flag.String("ai-file", kbAIFile, "Path to AI knowledge base file")
	configFileFlag := flag.String("config", configFile, "Path to config file")
	writeExampleConfigFlag := flag.Bool("write-example-config", false, "Write an example config file and exit")

	flag.Parse()

	// Set global preference
	iconPreference = *preferFlag

	// Handle example file generation
	if *writeExampleConfigFlag {
		examplePath := "config.example.toml"
		if err := WriteDefaultConfig(examplePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing example config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote example config to %s\n", examplePath)
		return
	}

	// Load config
	config, err := LoadConfig(*configFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Handle --fetch: fetch upstream metadata and regenerate kb-patterns.toml
	if *fetchFlag {
		fmt.Println("=== Fetching upstream metadata ===")

		// 1. Fetch Nerd Font glyphs (always refresh when --fetch is used)
		fmt.Println("Fetching Nerd Font glyphnames.json...")
		if _, err := loadGlyphs(true); err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching glyphs: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Cached glyphnames.json\n")

		// 2. Fetch Nerd Font symbols TTF
		fmt.Println("Fetching Nerd Font symbols TTF...")
		if err := fetchFont(true, nerdFontCacheFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching font: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Cached %s\n", nerdFontCacheFile)

		// 3. Fetch upstream icon metadata (FA, MDI, Devicons, Codicons)
		fmt.Println("\nFetching icon library metadata...")
		if err := RunFetch(config, *verboseFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching upstream: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n=== Fetch complete ===")
		fmt.Printf("Generated: %s\n", kbPatternsFile)
		fmt.Println("Next: Run --generate-kb to create AI app mappings")
		return
	}

	// Load glyph data (from cache)
	glyphs, err := loadGlyphs(false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading glyphs: %v\n", err)
		fmt.Fprintf(os.Stderr, "Run with --fetch to download Nerd Font data\n")
		os.Exit(1)
	}
	fmt.Printf("Loaded %d glyphs from Nerd Fonts\n", len(glyphs))

	// Handle KB generation
	if *generateKBFlag {
		// Handle cache clearing
		if *clearCacheFlag {
			if err := ClearCache(); err != nil {
				fmt.Fprintf(os.Stderr, "Error clearing cache: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Cleared API response cache")
		}

		// Load patterns for canonical icon names (auto + manual merged)
		patterns, err := LoadPatternsWithManual(*patternsFileFlag, *manualPatternsFileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading patterns: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Loaded %d icon patterns (auto: %s, manual: %s)\n",
			len(patterns.Icons), *patternsFileFlag, *manualPatternsFileFlag)

		// Match glyphs to canonical icons using patterns
		fmt.Println("Matching glyphs to canonical icons...")
		matchedIcons := MatchGlyphsToIcons(patterns, glyphs, *verboseFlag)
		fmt.Printf("Matched %d canonical icons to glyphs\n", len(matchedIcons))

		// Show cache stats
		useCache := !*noCacheFlag
		if useCache {
			_, appGenCount, totalSize := CacheStats()
			if appGenCount > 0 {
				fmt.Printf("Cache: %d app-gen entries (%.1f KB)\n",
					appGenCount, float64(totalSize)/1024)
			}
		}

		client, err := NewOpenRouterClient(*openrouterModelFlag, *webSearchFlag, useCache, config, *verboseFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating OpenRouter client: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Generating app mappings using model: %s (web search: %v, cache: %v)\n",
			client.Model, client.WebSearch, client.UseCache)

		// Get icons for app generation (from patterns, not AI classification)
		iconsForAppGen := GetIconsForAppGeneration(matchedIcons)

		// Generate app mappings using AI (only app generation, no classification)
		kb, err := client.GenerateAppMappings(iconsForAppGen, matchedIcons)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating app mappings: %v\n", err)
			os.Exit(1)
		}

		if err := SaveKnowledgeBase(kb, *aiFileFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving knowledge base: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Saved knowledge base to %s\n", *aiFileFlag)
		fmt.Printf("Generated %d icon mappings\n", len(kb.Icons))
		return
	}

	// Fetch font if requested
	if *fontOutputFlag != "" {
		if err := fetchFont(*fetchFlag, *fontOutputFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching font: %v\n", err)
			os.Exit(1)
		}
	}

	// Load default knowledge base
	defaultKB, err := LoadKnowledgeBase(*defaultFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error loading default KB: %v\n", err)
	} else if defaultKB != nil {
		fmt.Printf("Loaded default KB (%d icons)\n", len(defaultKB.Icons))
	}

	// Load AI knowledge base (if exists)
	aiKB, err := LoadKnowledgeBase(*aiFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error loading AI KB: %v\n", err)
	} else if aiKB != nil {
		fmt.Printf("Loaded AI KB (%d icons, generated %s)\n", len(aiKB.Icons), aiKB.GeneratedAt)
	}

	// Merge and generate mappings
	var mappings []AppMapping

	if defaultKB != nil || aiKB != nil {
		// Merge all sources: AI > default
		fmt.Println("Using merged icon sources (AI > default)")
		merged := MergeIconSources(defaultKB, aiKB, *verboseFlag)

		// Deduplicate: ensure each app only maps to one icon (best match wins)
		fmt.Println("Deduplicating app assignments...")
		DeduplicateApps(merged, defaultKB, aiKB, *verboseFlag)

		// Use full glyphs map for lookup since KB stores full glyph names
		mappings = ConvertMergedToAppMapping(merged, glyphs, *verboseFlag)
	} else {
		// No KB files found - this shouldn't happen in normal usage
		fmt.Fprintf(os.Stderr, "Error: No knowledge base files found.\n")
		fmt.Fprintf(os.Stderr, "Expected at least one of:\n")
		fmt.Fprintf(os.Stderr, "  - %s (default mappings)\n", *defaultFileFlag)
		fmt.Fprintf(os.Stderr, "  - %s (AI-generated mappings)\n", *aiFileFlag)
		fmt.Fprintf(os.Stderr, "\nRun with --generate-kb to create the AI knowledge base.\n")
		os.Exit(1)
	}

	fmt.Printf("Generated %d app mappings\n", len(mappings))

	// Write TOML output
	if err := writeTOML(*outputFlag, mappings); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing TOML: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s\n", *outputFlag)

	// Print summary
	printSummary(mappings)
}

func fetchFont(fetch bool, outputPath string) error {
	var data []byte
	var err error

	if fetch {
		fmt.Println("Fetching Nerd Font symbols from GitHub...")
		resp, err := http.Get(nerdFontURL)
		if err != nil {
			return fmt.Errorf("fetch failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("fetch failed: HTTP %d", resp.StatusCode)
		}

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}

		// Cache it locally
		if err := os.WriteFile(nerdFontCacheFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not cache font: %v\n", err)
		}
	} else {
		// Try cache first
		data, err = os.ReadFile(nerdFontCacheFile)
		if err != nil {
			fmt.Println("No cached font found, fetching from GitHub...")
			return fetchFont(true, outputPath)
		}
	}

	// Write to output path
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write font: %w", err)
	}

	fmt.Printf("Wrote font to %s (%d bytes)\n", outputPath, len(data))
	return nil
}

func loadGlyphs(fetch bool) (map[string]GlyphInfo, error) {
	var data []byte
	var err error

	if fetch {
		fmt.Println("Fetching glyphnames.json from GitHub...")
		resp, err := http.Get(glyphNamesURL)
		if err != nil {
			return nil, fmt.Errorf("fetch failed: %w", err)
		}
		defer resp.Body.Close()

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		// Cache it
		if err := os.WriteFile(cacheFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not cache: %v\n", err)
		}
	} else {
		// Try cache first
		data, err = os.ReadFile(cacheFile)
		if err != nil {
			fmt.Println("No cache found, fetching from GitHub...")
			return loadGlyphs(true)
		}
	}

	var glyphs map[string]GlyphInfo
	if err := json.Unmarshal(data, &glyphs); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return glyphs, nil
}

func writeTOML(path string, mappings []AppMapping) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# Icon Aliases for histui\n")
	fmt.Fprintf(f, "# Generated by generate-icon-aliases on %s\n", time.Now().Format("2006-01-02"))
	fmt.Fprintf(f, "# Maps Linux application names to Nerd Font icon names.\n")
	fmt.Fprintf(f, "#\n")
	fmt.Fprintf(f, "# Place this file at: ~/.config/histui/icon-aliases.toml\n")
	fmt.Fprintf(f, "#\n")
	fmt.Fprintf(f, "# The [aliases] section maps app names to icon names.\n")
	fmt.Fprintf(f, "# The [symbols] section maps icon names to Nerd Font glyphs.\n")
	fmt.Fprintf(f, "# To override an icon, set: app-name = \"icon-name\"\n\n")
	fmt.Fprintf(f, "[aliases]\n")

	// Group by category for readability
	categories := map[string][]AppMapping{
		"messaging": {},
		"browsers":  {},
		"email":     {},
		"media":     {},
		"dev":       {},
		"system":    {},
		"other":     {},
	}

	categorize := func(iconName string) string {
		name := strings.ToLower(iconName)
		switch {
		case strings.Contains(name, "discord") || strings.Contains(name, "slack") ||
			strings.Contains(name, "telegram") || strings.Contains(name, "whatsapp") ||
			strings.Contains(name, "signal") || strings.Contains(name, "matrix") ||
			strings.Contains(name, "skype") || strings.Contains(name, "facebook") ||
			strings.Contains(name, "twitter") || strings.Contains(name, "mastodon"):
			return "messaging"
		case strings.Contains(name, "firefox") || strings.Contains(name, "chrome") ||
			strings.Contains(name, "brave") || strings.Contains(name, "edge") ||
			strings.Contains(name, "opera") || strings.Contains(name, "vivaldi") ||
			strings.Contains(name, "tor") || strings.Contains(name, "safari"):
			return "browsers"
		case strings.Contains(name, "email") || strings.Contains(name, "gmail") ||
			strings.Contains(name, "outlook"):
			return "email"
		case strings.Contains(name, "spotify") || strings.Contains(name, "youtube") ||
			strings.Contains(name, "music") || strings.Contains(name, "video") ||
			strings.Contains(name, "vlc"):
			return "media"
		case strings.Contains(name, "code") || strings.Contains(name, "git") ||
			strings.Contains(name, "terminal") || strings.Contains(name, "docker") ||
			strings.Contains(name, "vim") || strings.Contains(name, "emacs") ||
			strings.Contains(name, "intellij") || strings.Contains(name, "pycharm"):
			return "dev"
		case strings.Contains(name, "folder") || strings.Contains(name, "cog") ||
			strings.Contains(name, "lock") || strings.Contains(name, "wifi") ||
			strings.Contains(name, "bluetooth") || strings.Contains(name, "cloud"):
			return "system"
		default:
			return "other"
		}
	}

	for _, m := range mappings {
		cat := categorize(m.IconName)
		categories[cat] = append(categories[cat], m)
	}

	order := []string{"messaging", "browsers", "email", "media", "dev", "system", "other"}
	titles := map[string]string{
		"messaging": "Messaging & Social",
		"browsers":  "Web Browsers",
		"email":     "Email Clients",
		"media":     "Media Players",
		"dev":       "Development Tools",
		"system":    "System & Utilities",
		"other":     "Other Applications",
	}

	// Track written app names to detect duplicates
	written := make(map[string]string) // appName -> targetIcon

	// First, collect all canonical icon names to avoid aliasing them to different icons
	canonicalIcons := make(map[string]bool)
	for _, ms := range categories {
		for _, m := range ms {
			targetIcon := m.IconName
			if idx := strings.Index(m.IconName, "-"); idx != -1 {
				targetIcon = m.IconName[idx+1:]
			}
			canonicalIcons[strings.ToLower(targetIcon)] = true
		}
	}

	for _, cat := range order {
		ms := categories[cat]
		if len(ms) == 0 {
			continue
		}

		// Sort mappings within category by icon name for deterministic output
		sort.Slice(ms, func(i, j int) bool {
			return ms[i].IconName < ms[j].IconName
		})

		fmt.Fprintf(f, "\n# %s\n", titles[cat])
		for _, m := range ms {
			// Extract the base icon name for the target (e.g., "discord" from "md-discord")
			// Nerd Font icons are prefix-name, e.g., "md-discord", "fa-telegram"
			targetIcon := m.IconName
			if idx := strings.Index(m.IconName, "-"); idx != -1 {
				targetIcon = m.IconName[idx+1:] // e.g., "discord" from "md-discord"
			}

			// Sort app names for deterministic output
			sortedApps := make([]string, len(m.AppNames))
			copy(sortedApps, m.AppNames)
			sort.Strings(sortedApps)

			for _, appName := range sortedApps {
				// Normalize app name to lowercase for consistency
				normalizedApp := strings.ToLower(appName)

				// Skip if app name equals target (no alias needed)
				if normalizedApp == strings.ToLower(targetIcon) {
					continue
				}

				// Skip if app name is itself a canonical icon name (should get its own icon)
				if canonicalIcons[normalizedApp] && normalizedApp != strings.ToLower(targetIcon) {
					fmt.Fprintf(os.Stderr, "WARNING: skipping %q -> %q (app name is a canonical icon)\n",
						appName, targetIcon)
					continue
				}

				// Check for duplicates
				if existingTarget, exists := written[normalizedApp]; exists {
					fmt.Fprintf(os.Stderr, "WARNING: duplicate app name %q (already mapped to %q, skipping %q)\n",
						appName, existingTarget, targetIcon)
					continue
				}
				written[normalizedApp] = targetIcon

				// Quote keys that contain dots (TOML interprets dots as nested tables otherwise)
				if strings.Contains(normalizedApp, ".") {
					fmt.Fprintf(f, "%q = %q\n", normalizedApp, targetIcon)
				} else {
					fmt.Fprintf(f, "%s = %q\n", normalizedApp, targetIcon)
				}
			}
		}
	}

	// Write symbols section - maps icon names to Nerd Font glyphs
	fmt.Fprintf(f, "\n# Nerd Font symbols (icon name -> glyph)\n")
	fmt.Fprintf(f, "# Generated from Nerd Fonts glyphnames.json\n")
	fmt.Fprintf(f, "[symbols]\n")

	// Collect unique symbols and sort by icon name
	symbols := make(map[string]string) // iconName -> glyphChar
	for _, m := range mappings {
		// Extract the base icon name (e.g., "discord" from "md-discord")
		targetIcon := m.IconName
		if idx := strings.Index(m.IconName, "-"); idx != -1 {
			targetIcon = m.IconName[idx+1:]
		}
		// Store the glyph character
		if m.GlyphChar != "" {
			symbols[targetIcon] = m.GlyphChar
		}
	}

	// Sort and write app symbols
	var iconNames []string
	for name := range symbols {
		iconNames = append(iconNames, name)
	}
	sort.Strings(iconNames)

	fmt.Fprintf(f, "\n# Application icons\n")
	for _, name := range iconNames {
		glyph := symbols[name]
		// Write as escaped unicode for readability
		if len(glyph) > 0 {
			r := []rune(glyph)[0]
			fmt.Fprintf(f, "%s = \"\\U%08X\"\n", name, r)
		}
	}

	// Note: Fallback symbols for urgencies (low, normal, critical, undefined) and
	// categories (notification, im, device, transfer, presence) are defined in
	// resolver.go. Users can override them by adding entries to their icon-aliases.toml.

	return nil
}

func printSummary(mappings []AppMapping) {
	fmt.Println("\n=== Summary ===")

	// Count apps per category
	var totalApps int
	for _, m := range mappings {
		totalApps += len(m.AppNames)
	}

	fmt.Printf("Total icon types: %d\n", len(mappings))
	fmt.Printf("Total app aliases: %d\n", totalApps)

	// List unmapped known icons
	fmt.Println("\nMapped icons:")
	for _, m := range mappings {
		fmt.Printf("  %s -> %d apps\n", m.IconName, len(m.AppNames))
	}
}

// sanitizeAppName converts an app name to a valid TOML key
func sanitizeAppName(name string) string {
	// Replace spaces and special chars with hyphens
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	return strings.ToLower(re.ReplaceAllString(name, "-"))
}
