// generate-icon-aliases parses symbol font glyph data and generates icon mappings.
//
// Usage:
//
//	go run ./contrib/generate-icon-aliases [--fetch] [--output aliases.toml]
//
// This tool:
//   - Downloads/reads glyphnames.json from Nerd Fonts (symbol font source)
//   - Filters for app-related icons (discord, firefox, etc.)
//   - Maps common Linux application names to canonical icon names
//   - Generates icon-aliases.toml with [aliases], [symbols], [gtk-icons] sections
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
	"strings"
	"time"
)

const (
	glyphNamesURL     = "https://raw.githubusercontent.com/ryanoasis/nerd-fonts/master/glyphnames.json"
	cacheFile         = "glyphnames.json"
	nerdFontURL       = "https://github.com/ryanoasis/nerd-fonts/raw/master/patched-fonts/NerdFontsSymbolsOnly/SymbolsNerdFont-Regular.ttf"
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
	Category   string   // Category for the icon (messaging, browsers, etc.)
}

// explicitIconOverrides maps icon keys to specific glyph names when the auto-match fails or is wrong
var explicitIconOverrides = map[string]string{
	"tor-browser":        "linux-tor",             // Use Linux Tor icon instead of fuzzy match
	"message":            "md-chat",               // Chat icon for Matrix clients (Element, Fractal, etc.)
	"monitor-screenshot": "md-monitor_screenshot", // Correct screenshot icon
	"min":                "cod-browser",           // Generic browser icon
	"desktop-classic":    "cod-desktop_download",  // VM/desktop download icon
	"image":              "md-file_image",         // Simple file image icon
	"brave":              "md-shield_half_full",   // Shield for Brave browser
}

func main() {
	// Primary workflow flags
	fetchFlag := flag.Bool("fetch", false, "Fetch upstream metadata and regenerate kb-patterns.toml (also refreshes Nerd Font data)")
	outputFlag := flag.String("output", "icon-aliases.toml", "Output TOML file path")
	fontOutputFlag := flag.String("font-output", "", "Output path for Nerd Font symbols TTF (optional)")
	verboseFlag := flag.Bool("verbose", false, "Verbose logging")
	preferFlag := flag.String("prefer", "md", "Preferred icon set: md (Material Design), fa (Font Awesome), dev (Devicons)")

	// Knowledge base flags
	aiAppsFlag := flag.Bool("ai-apps", false, "Generate AI app mappings for brand icons (requires OPENROUTER_API_KEY)")
	aiCategoriesFlag := flag.Bool("ai-categories", false, "Generate AI app mappings for categories (requires OPENROUTER_API_KEY)")
	suggestCategoryIconsFlag := flag.Bool("suggest-category-icons", false, "Generate AI icon suggestions for categories (requires OPENROUTER_API_KEY)")
	openrouterModelFlag := flag.String("openrouter-model", "", "OpenRouter model to use (from config or anthropic/claude-sonnet-4)")
	webSearchFlag := flag.Bool("web-search", false, "Enable web search for real-time data (adds :online suffix)")
	noCacheFlag := flag.Bool("no-cache", false, "Disable caching of API responses")
	clearCacheFlag := flag.Bool("clear-cache", false, "Clear the API response cache before generating")
	patternsFileFlag := flag.String("patterns-file", kbPatternsFile, "Path to auto-generated patterns file")
	manualPatternsFileFlag := flag.String("manual-patterns-file", kbPatternsManualFile, "Path to manual patterns override file")
	defaultFileFlag := flag.String("default-file", kbDefaultFile, "Path to default knowledge base file")
	aiAppsFileFlag := flag.String("ai-apps-file", kbAIAppsFile, "Path to AI apps knowledge base file")
	aiCategoriesFileFlag := flag.String("ai-categories-file", kbAICategoriesFile, "Path to AI categories knowledge base file")
	categorySuggestionsFileFlag := flag.String("category-suggestions-file", "kb-category-suggestions.toml", "Path to output category icon suggestions")
	applySuggestionsFlag := flag.Bool("apply-suggestions", false, "Apply top suggestions from kb-category-suggestions.toml to kb-categories.toml")
	categoriesFileFlag := flag.String("categories-file", kbCategoriesFile, "Path to categories file")
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
		fmt.Println("Next: Run --ai-apps to create AI app mappings for brand icons")
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

	// Handle --ai-apps: generate app mappings for brand icons
	if *aiAppsFlag {
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

		// Load extra apps to research
		extraAppsConfig, err := LoadExtraApps("extra-apps.toml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading extra apps: %v\n", err)
			os.Exit(1)
		}
		extraApps := extraAppsConfig.FormatForPrompt()
		if len(extraApps) > 0 {
			fmt.Printf("Loaded %d extra apps to research\n", len(extraApps))
			if *verboseFlag {
				for _, app := range extraApps {
					fmt.Printf("  - %s\n", app)
				}
			}
		}

		// Match glyphs to canonical icons using patterns
		fmt.Println("Matching glyphs to canonical icons...")
		matchedIcons := MatchGlyphsToIcons(patterns, glyphs, *verboseFlag)
		fmt.Printf("Matched %d canonical icons to glyphs\n", len(matchedIcons))

		// Find and report unmatched icons
		unmatched := FindUnmatchedIcons(patterns, matchedIcons)
		if len(unmatched) > 0 {
			fmt.Printf("Warning: %d icons have no matching glyphs\n", len(unmatched))
			if *verboseFlag {
				for _, u := range unmatched {
					fmt.Printf("  NO MATCH: %s (patterns: %v)\n", u.Name, u.Patterns)
				}
			}
		}

		// Show cache stats
		useCache := !*noCacheFlag
		if useCache {
			appGenCount, catSuggestCount, totalSize := CacheStats()
			if appGenCount > 0 || catSuggestCount > 0 {
				fmt.Printf("Cache: %d app-gen, %d category-suggest entries (%.1f KB)\n",
					appGenCount, catSuggestCount, float64(totalSize)/1024)
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
		kb, err := client.GenerateAppMappings(iconsForAppGen, matchedIcons, extraApps)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating app mappings: %v\n", err)
			os.Exit(1)
		}

		if err := SaveKnowledgeBase(kb, *aiAppsFileFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving knowledge base: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Saved knowledge base to %s\n", *aiAppsFileFlag)
		fmt.Printf("Generated %d icon mappings\n", len(kb.Icons))
		fmt.Println("Next: Run --ai-categories to generate app mappings for categories")
		return
	}

	// Handle --ai-categories: generate app mappings for category icons
	if *aiCategoriesFlag {
		fmt.Println("=== Generating Category App Mappings ===")

		// Handle cache clearing
		if *clearCacheFlag {
			if err := ClearCache(); err != nil {
				fmt.Fprintf(os.Stderr, "Error clearing cache: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Cleared API response cache")
		}

		// Load categories
		categories, err := LoadCategories(*categoriesFileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading categories: %v\n", err)
			fmt.Fprintf(os.Stderr, "Make sure %s exists\n", *categoriesFileFlag)
			os.Exit(1)
		}
		fmt.Printf("Loaded %d categories from %s\n", len(categories.Categories), *categoriesFileFlag)

		// Show cache stats
		useCache := !*noCacheFlag
		if useCache {
			appGenCount, catSuggestCount, totalSize := CacheStats()
			if appGenCount > 0 || catSuggestCount > 0 {
				fmt.Printf("Cache: %d app-gen, %d category-suggest entries (%.1f KB)\n",
					appGenCount, catSuggestCount, float64(totalSize)/1024)
			}
		}

		client, err := NewOpenRouterClient(*openrouterModelFlag, *webSearchFlag, useCache, config, *verboseFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating OpenRouter client: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Generating category app mappings using model: %s (web search: %v, cache: %v)\n",
			client.Model, client.WebSearch, client.UseCache)

		// Generate app mappings for categories
		kb, err := client.GenerateCategoryAppMappings(categories)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating category app mappings: %v\n", err)
			os.Exit(1)
		}

		if err := SaveKnowledgeBase(kb, *aiCategoriesFileFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving knowledge base: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Saved category knowledge base to %s\n", *aiCategoriesFileFlag)
		fmt.Printf("Generated %d category mappings\n", len(kb.Icons))
		fmt.Println("Next: Run without flags to generate final icon-aliases.toml")
		return
	}

	// Handle --suggest-category-icons: generate icon suggestions for categories
	if *suggestCategoryIconsFlag {
		fmt.Println("=== Generating Category Icon Suggestions ===")

		// Load categories
		categories, err := LoadCategories(kbCategoriesFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading categories: %v\n", err)
			fmt.Fprintf(os.Stderr, "Make sure %s exists\n", kbCategoriesFile)
			os.Exit(1)
		}
		fmt.Printf("Loaded %d categories from %s\n", len(categories.Categories), kbCategoriesFile)

		// Build glyph metadata for the AI prompt
		glyphMetadata := BuildAllGlyphMetadata(glyphs, config.IconPreferences.PreferredSets)
		fmt.Printf("Prepared %d glyphs for AI (filtered non-brand icons)\n", len(glyphMetadata))

		// Fetch GTK icons from Adwaita theme
		gtkIcons, err := FetchGtkIcons(config.Upstream.AdwaitaIcons, *fetchFlag, *verboseFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not fetch GTK icons: %v\n", err)
			fmt.Println("Continuing without GTK icon suggestions...")
			gtkIcons = nil
		} else {
			// Filter to relevant categories
			gtkIcons = FilterGtkIconsForCategories(gtkIcons)
			fmt.Printf("Prepared %d GTK icons for AI\n", len(gtkIcons))
		}

		// Create OpenRouter client
		useCache := !*noCacheFlag
		client, err := NewOpenRouterClient(*openrouterModelFlag, *webSearchFlag, useCache, config, *verboseFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating OpenRouter client: %v\n", err)
			os.Exit(1)
		}

		// Generate prompt (with GTK icons if available)
		var prompt string
		if len(gtkIcons) > 0 {
			prompt, err = config.RenderCategorySuggestPromptWithGtk(categories, glyphMetadata, gtkIcons)
		} else {
			prompt, err = config.RenderCategorySuggestPrompt(categories, glyphMetadata)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error rendering prompt: %v\n", err)
			os.Exit(1)
		}

		if *verboseFlag {
			fmt.Printf("\n--- Prompt Preview (first 2000 chars) ---\n")
			if len(prompt) > 2000 {
				fmt.Println(prompt[:2000] + "...")
			} else {
				fmt.Println(prompt)
			}
			fmt.Println("--- End Preview ---")
		}

		// Generate cache key from inputs
		cacheKey := CategorySuggestCacheKey(categories, glyphMetadata)
		fmt.Printf("Using model: %s (cache: %v)\n", client.Model, useCache)

		// Call AI (with caching)
		suggestions, err := client.GenerateCategorySuggestions(prompt, cacheKey, useCache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating suggestions: %v\n", err)
			os.Exit(1)
		}

		// Write suggestions to TOML
		if err := WriteCategorySuggestions(suggestions, glyphs, *categorySuggestionsFileFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing suggestions: %v\n", err)
			os.Exit(1)
		}

		// Summary
		fmt.Println("\n=== Summary ===")
		fmt.Printf("Generated suggestions for %d categories\n", len(suggestions))
		fmt.Println("\nOutput files:")
		fmt.Printf("  %s\n", *categorySuggestionsFileFlag)
		fmt.Println("\nNext step:")
		fmt.Println("  task generate:icons:apply-suggestions")
		return
	}

	// Handle --apply-suggestions: apply top suggestions to kb-categories.toml
	if *applySuggestionsFlag {
		fmt.Println("=== Applying Category Icon Suggestions ===")

		// Load suggestions
		suggestions, err := LoadCategorySuggestions(*categorySuggestionsFileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading suggestions: %v\n", err)
			fmt.Fprintf(os.Stderr, "Run --suggest-category-icons first to generate suggestions.\n")
			os.Exit(1)
		}
		fmt.Printf("Loaded suggestions for %d categories from %s\n", len(suggestions), *categorySuggestionsFileFlag)

		// Load current categories
		categories, err := LoadCategories(*categoriesFileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading categories: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Loaded %d categories from %s\n", len(categories.Categories), *categoriesFileFlag)

		// Apply top suggestions
		applied := 0
		for catName, catSuggestions := range suggestions {
			if len(catSuggestions) == 0 {
				continue
			}
			top := catSuggestions[0]

			// Find the category and update it
			if cat, ok := categories.Categories[catName]; ok {
				// Look up the symbol from the glyph name using normalization
				symbol, foundGlyphName := lookupGlyphWithNormalization(top.Glyph, glyphs)

				if symbol == "" {
					fmt.Printf("  Warning: could not find glyph %q for category %q\n", top.Glyph, catName)
					continue
				}

				cat.Symbol = symbol
				cat.Glyph = foundGlyphName

				// Apply GTK icon if provided
				if top.GtkIcon != "" {
					cat.GtkIcon = top.GtkIcon
				}

				categories.Categories[catName] = cat
				applied++
				if *verboseFlag {
					gtkInfo := ""
					if top.GtkIcon != "" {
						gtkInfo = fmt.Sprintf(", gtk: %s", top.GtkIcon)
					}
					fmt.Printf("  %s -> %s (%s%s) (confidence: %.2f)\n", catName, symbol, foundGlyphName, gtkInfo, top.Confidence)
				}
			} else {
				fmt.Printf("  Warning: category %q not found in %s\n", catName, *categoriesFileFlag)
			}
		}

		// Write updated categories
		if err := SaveCategories(categories, *categoriesFileFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving categories: %v\n", err)
			os.Exit(1)
		}

		// Summary
		fmt.Println("\n=== Summary ===")
		fmt.Printf("Applied %d category icon suggestions\n", applied)
		fmt.Println("\nOutput files:")
		fmt.Printf("  %s\n", *categoriesFileFlag)
		fmt.Println("\nNext step:")
		fmt.Println("  task generate:icons:output")
		return
	}

	// Fetch font if requested
	if *fontOutputFlag != "" {
		if err := fetchFont(*fetchFlag, *fontOutputFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching font: %v\n", err)
			os.Exit(1)
		}
	}

	// Load urgency/category defaults from TOML (kb-default.toml)
	defaultsConfig, err := LoadDefaultsConfig(*defaultFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error loading defaults config: %v\n", err)
	} else if defaultsConfig != nil {
		fmt.Printf("Loaded defaults config (%d symbols, %d gtk-icons)\n",
			len(defaultsConfig.Symbols), len(defaultsConfig.GtkIcons))
	}

	// Load categories for category fallback icons
	categories, err := LoadCategories(*categoriesFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error loading categories: %v\n", err)
	} else if categories != nil {
		// Count categories with symbols
		withSymbols := 0
		for _, cat := range categories.Categories {
			if cat.Symbol != "" {
				withSymbols++
			}
		}
		fmt.Printf("Loaded categories (%d total, %d with symbols)\n",
			len(categories.Categories), withSymbols)
	}

	// Load AI knowledge bases (brand icons and categories)
	aiAppsKB, err := LoadKnowledgeBase(*aiAppsFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error loading AI apps KB: %v\n", err)
	} else if aiAppsKB != nil {
		fmt.Printf("Loaded AI apps KB (%d brand icons, generated %s)\n", len(aiAppsKB.Icons), aiAppsKB.GeneratedAt)
	}

	aiCategoriesKB, err := LoadKnowledgeBase(*aiCategoriesFileFlag)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: error loading AI categories KB: %v\n", err)
		}
	} else if aiCategoriesKB != nil {
		fmt.Printf("Loaded AI categories KB (%d categories, generated %s)\n", len(aiCategoriesKB.Icons), aiCategoriesKB.GeneratedAt)
	}

	// Generate mappings from AI KBs
	var mappings []AppMapping
	var merged map[string]*MergedIcon

	if aiAppsKB != nil {
		// Use AI KBs as the source of app mappings
		fmt.Println("Using AI knowledge bases for app mappings")

		// Merge brand icons KB with categories KB
		// Brand icons take priority over categories for the same app
		merged = MergeIconSources(aiCategoriesKB, aiAppsKB, *verboseFlag)

		// Apply manual force_apps overrides (highest priority)
		manualPatterns, err := LoadPatternsWithManual(*patternsFileFlag, *manualPatternsFileFlag)
		if err == nil && manualPatterns != nil {
			ApplyManualForceApps(merged, manualPatterns, glyphs, *verboseFlag)
		}

		// Deduplicate: ensure each app only maps to one icon (best match wins)
		fmt.Println("Deduplicating app assignments...")
		DeduplicateApps(merged, aiCategoriesKB, aiAppsKB, *verboseFlag)

		// Use full glyphs map for lookup since KB stores full glyph names
		mappings = ConvertMergedToAppMapping(merged, glyphs, *verboseFlag)
	} else {
		// No AI KB found - error
		fmt.Fprintf(os.Stderr, "Error: No AI apps knowledge base found.\n")
		fmt.Fprintf(os.Stderr, "Expected: %s (AI-generated brand icon mappings)\n", *aiAppsFileFlag)
		fmt.Fprintf(os.Stderr, "\nRun with --ai-apps to create the AI knowledge base.\n")
		os.Exit(1)
	}

	fmt.Printf("Generated %d app mappings\n", len(mappings))

	// Write TOML output (with defaults and categories merged in)
	if err := writeTOML(*outputFlag, mappings, defaultsConfig, categories, aiAppsKB, merged, aiCategoriesKB); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing TOML: %v\n", err)
		os.Exit(1)
	}
	// Print summary
	printSummary(mappings, *outputFlag, *fontOutputFlag, *verboseFlag)
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
		defer func() { _ = resp.Body.Close() }()

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
		defer func() { _ = resp.Body.Close() }()

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

// gtkIconForCanonical maps a canonical icon name to an appropriate GTK icon name.
// For branded apps, we use the app name directly (e.g., "discord" -> "discord").
// For generic/category icons, we map to freedesktop standard icons.
func gtkIconForCanonical(canonical string, _ string) string {
	// Canonical to GTK icon name mappings for generic icons
	gtkMappings := map[string]string{
		// Urgency/status icons (freedesktop standard)
		"urgency-low":      "dialog-information-symbolic",
		"urgency-normal":   "dialog-information-symbolic",
		"urgency-critical": "dialog-warning-symbolic",

		// Generic categories
		"message":            "mail-message-new-symbolic",
		"chat":               "user-available-symbolic",
		"email":              "mail-unread-symbolic",
		"calendar":           "x-office-calendar-symbolic",
		"alarm":              "alarm-symbolic",
		"reminder":           "task-due-symbolic",
		"download":           "folder-download-symbolic",
		"upload":             "folder-upload-symbolic",
		"sync":               "emblem-synchronizing-symbolic",
		"update":             "software-update-available-symbolic",
		"error":              "dialog-error-symbolic",
		"warning":            "dialog-warning-symbolic",
		"info":               "dialog-information-symbolic",
		"question":           "dialog-question-symbolic",
		"security":           "security-high-symbolic",
		"network":            "network-wireless-symbolic",
		"bluetooth":          "bluetooth-symbolic",
		"battery":            "battery-symbolic",
		"volume":             "audio-volume-high-symbolic",
		"brightness":         "display-brightness-symbolic",
		"printer":            "printer-symbolic",
		"usb":                "drive-removable-media-symbolic",
		"file":               "text-x-generic-symbolic",
		"folder":             "folder-symbolic",
		"image":              "image-x-generic-symbolic",
		"video":              "video-x-generic-symbolic",
		"audio":              "audio-x-generic-symbolic",
		"document":           "x-office-document-symbolic",
		"spreadsheet":        "x-office-spreadsheet-symbolic",
		"archive":            "package-x-generic-symbolic",
		"code":               "text-x-script-symbolic",
		"terminal":           "utilities-terminal-symbolic",
		"settings":           "preferences-system-symbolic",
		"system":             "computer-symbolic",
		"user":               "avatar-default-symbolic",
		"trash":              "user-trash-symbolic",
		"search":             "edit-find-symbolic",
		"lock":               "system-lock-screen-symbolic",
		"power":              "system-shutdown-symbolic",
		"screenshot":         "applets-screenshooter-symbolic",
		"monitor_screenshot": "applets-screenshooter-symbolic",
		"clipboard":          "edit-paste-symbolic",
	}

	// Check for explicit mapping first
	if gtkIcon, ok := gtkMappings[canonical]; ok {
		return gtkIcon
	}

	// For branded apps, use the canonical name directly
	// GTK icon themes typically have icons named after popular apps
	return canonical
}

// appEntry represents an app or category icon entry
type appEntry struct {
	symbol    string
	gtkIcon   string
	glyphName string // Nerd Font glyph name for comment
}

// aliasEntry tracks an alias with its confidence score
type aliasEntry struct {
	target     string
	confidence float64
	isCategory bool // true if this is a category fallback assignment
}

func writeTOML(path string, mappings []AppMapping, defaults *DefaultsConfig, categories *CategoriesConfig, aiAppsKB *KnowledgeBase, merged map[string]*MergedIcon, aiCategoriesKB *KnowledgeBase) error {
	// Build data structures
	aliases := make(map[string]aliasEntry) // app name → alias entry with confidence
	apps := make(map[string]appEntry)      // canonical name → icon entry
	written := make(map[string]string)     // for duplicate detection

	// Build confidence lookup from AI KBs
	appConfidence := make(map[string]float64)
	if aiAppsKB != nil {
		for _, icon := range aiAppsKB.Icons {
			for _, app := range icon.Apps {
				appConfidence[strings.ToLower(app.ID)] = app.Confidence
			}
		}
	}
	if aiCategoriesKB != nil {
		for _, icon := range aiCategoriesKB.Icons {
			for _, app := range icon.Apps {
				// Only add if not already present (brand icons take precedence)
				if _, exists := appConfidence[strings.ToLower(app.ID)]; !exists {
					appConfidence[strings.ToLower(app.ID)] = app.Confidence
				}
			}
		}
	}

	// First, collect all canonical icon names to avoid aliasing them to different icons
	canonicalIcons := make(map[string]bool)
	for _, m := range mappings {
		targetIcon := m.IconName
		if idx := strings.Index(m.IconName, "-"); idx != -1 {
			targetIcon = m.IconName[idx+1:]
		}
		canonicalIcons[strings.ToLower(targetIcon)] = true
	}

	// Build set of category names for detecting category-based aliases
	categoryNames := make(map[string]bool)
	if categories != nil {
		for name := range categories.Categories {
			categoryNames[name] = true
		}
	}

	// Build aliases map from app names to canonical icon names
	for _, m := range mappings {
		// Extract the base icon name for the target (e.g., "discord" from "md-discord")
		targetIcon := m.IconName
		if idx := strings.Index(m.IconName, "-"); idx != -1 {
			targetIcon = m.IconName[idx+1:]
		}

		for _, appName := range m.AppNames {
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

			// Get confidence from AI KB
			conf := appConfidence[normalizedApp]
			isCategory := categoryNames[targetIcon]
			aliases[normalizedApp] = aliasEntry{
				target:     targetIcon,
				confidence: conf,
				isCategory: isCategory,
			}
		}
	}

	// Add category apps as aliases (from merged data, which is deduplicated)
	// These apps don't have brand icons, so they get assigned to their category
	if merged != nil {
		// Build a lookup map for per-app confidence from aiCategoriesKB
		categoryAppConfidence := make(map[string]map[string]float64) // categoryName -> appName -> confidence
		if aiCategoriesKB != nil {
			for catName, icon := range aiCategoriesKB.Icons {
				categoryAppConfidence[catName] = make(map[string]float64)
				for _, app := range icon.Apps {
					categoryAppConfidence[catName][strings.ToLower(app.ID)] = app.Confidence
				}
			}
		}

		for iconName, icon := range merged {
			// Only process category-type icons
			if icon.Type != "category" {
				continue
			}

			// Only process if this is a valid category
			if !categoryNames[iconName] {
				continue
			}

			for _, appName := range icon.Apps {
				normalizedApp := strings.ToLower(appName)

				// Skip if app name equals category name
				if normalizedApp == iconName {
					continue
				}

				// Skip if already mapped (brand icons take precedence)
				if _, exists := written[normalizedApp]; exists {
					continue
				}

				// Skip if app is a canonical icon name
				if canonicalIcons[normalizedApp] {
					continue
				}

				// Look up confidence from original KB
				confidence := 0.7 // default minimum confidence
				if catConf, ok := categoryAppConfidence[iconName]; ok {
					if appConf, ok := catConf[normalizedApp]; ok {
						confidence = appConf
					}
				}

				written[normalizedApp] = iconName
				aliases[normalizedApp] = aliasEntry{
					target:     iconName,
					confidence: confidence,
					isCategory: true,
				}
			}
		}
	}

	// Build apps map with symbol + gtk_icon + glyph name
	for _, m := range mappings {
		targetIcon := m.IconName
		if idx := strings.Index(m.IconName, "-"); idx != -1 {
			targetIcon = m.IconName[idx+1:]
		}
		if m.GlyphChar != "" && len(m.GlyphChar) > 0 {
			apps[targetIcon] = appEntry{
				symbol:    m.GlyphChar,
				gtkIcon:   gtkIconForCanonical(targetIcon, ""),
				glyphName: m.IconName, // Full Nerd Font name for comment
			}
		}
	}

	// Merge defaults from kb-default.toml (urgency icons, notification fallback)
	// These are special system entries, add them to apps section
	if defaults != nil {
		for name, symbol := range defaults.Symbols {
			gtkIcon := ""
			if icon, ok := defaults.GtkIcons[name]; ok {
				gtkIcon = icon
			}
			apps[name] = appEntry{
				symbol:    symbol,
				gtkIcon:   gtkIcon,
				glyphName: "", // No Nerd Font name for defaults
			}
		}
	}

	// Build categories map from kb-categories.toml
	// These are fallback icons when an app doesn't have a brand icon
	categoryEntries := make(map[string]appEntry)
	if categories != nil {
		for catName, cat := range categories.Categories {
			categoryEntries[catName] = appEntry{
				symbol:    cat.Symbol,
				gtkIcon:   cat.GtkIcon,
				glyphName: cat.Glyph, // Glyph name from category config
			}
		}
	}

	// Write file manually to include glyph name comments
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	generatedAt := time.Now().UTC().Format(time.RFC3339)

	// Helper to write with error tracking
	var writeErr error
	w := func(format string, a ...any) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintf(f, format, a...)
	}

	// Write header
	w("# Icon Aliases for histui\n")
	w("#\n")
	w("# DO NOT EDIT THIS FILE DIRECTLY!\n")
	w("#\n")
	w("# This file is auto-generated by: contrib/generate-icon-aliases\n")
	w("# Generated on: %s\n", generatedAt)
	w("# To regenerate:\n")
	w("#   cd contrib/generate-icon-aliases\n")
	w("#   ./generate-icon-aliases\n")
	w("#   task generate:icons  # copies output to embed/aliases_default.toml\n")
	w("#\n")
	w("# Sources:\n")
	w("#   - kb-default.toml (urgency and notification defaults)\n")
	w("#   - kb-categories.toml (category fallback icons)\n")
	w("#   - kb-patterns.toml + kb-ai-apps.json (app brand icons)\n")
	w("#   - kb-ai-categories.json (category fallback icons)\n")
	w("#\n")
	w("# Sections:\n")
	w("# [meta]       - File metadata (version, generation date)\n")
	w("# [aliases]    - Maps app names to canonical icon names\n")
	w("# [apps]       - Brand/app icons with symbol + gtk_icon\n")
	w("# [categories] - Category fallback icons (when no brand icon exists)\n")
	w("\n")

	// Write [meta] section
	w("[meta]\n")
	w("version = 1\n")
	w("generated_at = %s\n", tomlString(generatedAt))
	w("generator = \"contrib/generate-icon-aliases\"\n")
	w("\n")

	// Write [aliases] section
	w("[aliases]\n")
	aliasKeys := sortedAliasKeys(aliases)
	for _, key := range aliasKeys {
		entry := aliases[key]
		// Build confidence comment
		comment := ""
		if entry.confidence > 0 {
			if entry.isCategory {
				comment = fmt.Sprintf(" # %.2f (category)", entry.confidence)
			} else {
				comment = fmt.Sprintf(" # %.2f", entry.confidence)
			}
		}
		w("%s = %s%s\n", tomlKey(key), tomlString(entry.target), comment)
	}
	w("\n")

	// Write [apps] section - brand/app icons
	w("[apps]\n")
	appKeys := sortedAppKeys(apps)
	for _, key := range appKeys {
		entry := apps[key]
		comment := ""
		if entry.glyphName != "" {
			comment = " # " + entry.glyphName
		}
		w("%s = { symbol = %s, gtk_icon = %s }%s\n",
			tomlKey(key), tomlString(entry.symbol), tomlString(entry.gtkIcon), comment)
	}
	w("\n")

	// Write [categories] section - fallback icons
	w("[categories]\n")
	catKeys := sortedCatKeys(categoryEntries)
	for _, key := range catKeys {
		entry := categoryEntries[key]
		comment := ""
		if entry.glyphName != "" {
			comment = " # " + entry.glyphName
		}
		w("%s = { symbol = %s, gtk_icon = %s }%s\n",
			tomlKey(key), tomlString(entry.symbol), tomlString(entry.gtkIcon), comment)
	}

	return writeErr
}

// sortedAliasKeys returns sorted keys from an aliasEntry map
func sortedAliasKeys(m map[string]aliasEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// sortedAppKeys returns sorted keys from an appEntry map
func sortedAppKeys(m map[string]appEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// sortedCatKeys returns sorted keys from a category map
func sortedCatKeys(m map[string]appEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// tomlKey returns a properly quoted TOML key
func tomlKey(key string) string {
	// Check if key needs quoting (contains special chars)
	needsQuote := false
	for _, c := range key {
		// Check if character is alphanumeric or underscore/hyphen (allowed unquoted)
		//nolint:staticcheck // QF1001: positive form is more readable than De Morgan's law
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-') {
			needsQuote = true
			break
		}
	}
	if needsQuote {
		return "'" + strings.ReplaceAll(key, "'", "''") + "'"
	}
	return key
}

// tomlString returns a properly quoted TOML string value
func tomlString(s string) string {
	// Use single quotes for strings with special chars, double for regular
	if strings.ContainsAny(s, "'\n\r\t\\") {
		// Use double quotes and escape
		escaped := strings.ReplaceAll(s, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		escaped = strings.ReplaceAll(escaped, "\n", "\\n")
		escaped = strings.ReplaceAll(escaped, "\r", "\\r")
		escaped = strings.ReplaceAll(escaped, "\t", "\\t")
		return "\"" + escaped + "\""
	}
	return "'" + s + "'"
}

func printSummary(mappings []AppMapping, outputPath, fontOutputPath string, verbose bool) {
	fmt.Println("\n=== Summary ===")

	// Count apps per category
	var totalApps int
	for _, m := range mappings {
		totalApps += len(m.AppNames)
	}

	fmt.Printf("Total icon types: %d\n", len(mappings))
	fmt.Printf("Total app aliases: %d\n", totalApps)

	// Show mapped icons only in verbose mode
	if verbose {
		fmt.Println("\nMapped icons:")
		for _, m := range mappings {
			fmt.Printf("  %s -> %d apps\n", m.IconName, len(m.AppNames))
		}
	}

	// Output files
	fmt.Println("\nOutput files:")
	fmt.Printf("  %s\n", outputPath)
	if fontOutputPath != "" {
		fmt.Printf("  %s\n", fontOutputPath)
	}
}

// WriteCategorySuggestions writes category icon suggestions to a TOML file.
func WriteCategorySuggestions(suggestions map[string][]CategorySuggestion, glyphs map[string]GlyphInfo, path string) error {
	var buf strings.Builder

	buf.WriteString("# Category Icon Suggestions\n")
	buf.WriteString("# =========================\n")
	buf.WriteString("#\n")
	buf.WriteString("# Generated by: contrib/generate-icon-aliases --suggest-category-icons\n")
	buf.WriteString(fmt.Sprintf("# Generated on: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	buf.WriteString("#\n")
	buf.WriteString("# Review these suggestions and update kb-categories.toml with your choices.\n")
	buf.WriteString("# Each category has multiple icon candidates ranked by confidence.\n")
	buf.WriteString("#\n")
	buf.WriteString("# To apply: copy the symbol and gtk_icon values you want to kb-categories.toml\n")
	buf.WriteString("#\n\n")

	// Sort categories for consistent output
	var categoryNames []string
	for name := range suggestions {
		categoryNames = append(categoryNames, name)
	}
	sortStrings(categoryNames)

	for _, catName := range categoryNames {
		candidates := suggestions[catName]
		if len(candidates) == 0 {
			continue
		}

		buf.WriteString(fmt.Sprintf("[suggestions.%s]\n", catName))
		buf.WriteString("# Candidates (ranked by confidence):\n")

		for i, candidate := range candidates {
			// Find the glyph info using normalization (handles hyphen/underscore differences)
			symbol, foundName := lookupGlyphWithNormalization(candidate.Glyph, glyphs)
			code := ""
			if foundName != "" {
				if info, ok := glyphs[foundName]; ok {
					code = info.Code
				}
			}

			if i == 0 {
				// Best candidate - uncommented
				buf.WriteString(fmt.Sprintf("glyph = %q\n", candidate.Glyph))
				if candidate.GtkIcon != "" {
					buf.WriteString(fmt.Sprintf("gtk_icon = %q\n", candidate.GtkIcon))
				}
				if symbol != "" {
					buf.WriteString(fmt.Sprintf("symbol = %q  # U+%s\n", symbol, code))
				}
				buf.WriteString(fmt.Sprintf("confidence = %.2f\n", candidate.Confidence))
				buf.WriteString(fmt.Sprintf("reason = %q\n", candidate.Reason))
			} else {
				// Alternatives - commented
				buf.WriteString(fmt.Sprintf("# alt%d_glyph = %q", i, candidate.Glyph))
				if candidate.GtkIcon != "" {
					buf.WriteString(fmt.Sprintf(" gtk_icon = %q", candidate.GtkIcon))
				}
				if symbol != "" {
					buf.WriteString(fmt.Sprintf("  # symbol: %s (U+%s)", symbol, code))
				}
				buf.WriteString(fmt.Sprintf("  # confidence: %.2f, reason: %s\n",
					candidate.Confidence, candidate.Reason))
			}
		}
		buf.WriteString("\n")
	}

	return os.WriteFile(path, []byte(buf.String()), 0644)
}

// sortStrings sorts a slice of strings in place.
func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// lookupGlyphWithNormalization tries to find a glyph by name, handling naming convention
// differences between Font Awesome (hyphens) and Nerd Fonts (underscores).
// Returns the symbol character and the actual glyph name found, or empty strings if not found.
func lookupGlyphWithNormalization(glyphName string, glyphs map[string]GlyphInfo) (symbol string, foundName string) {
	// Prefixes to try
	prefixes := []string{"", "md-", "fa-", "cod-", "dev-"}

	// First, try exact matches with prefixes
	for _, prefix := range prefixes {
		testName := prefix + glyphName
		if info, ok := glyphs[testName]; ok {
			return info.Char, testName
		}
		// Also try exact match of the name itself
		if info, ok := glyphs[glyphName]; ok {
			return info.Char, glyphName
		}
	}

	// Second, try with hyphens converted to underscores
	for _, prefix := range prefixes {
		// For names with a prefix (e.g., "fa-folder-open"), convert only the name part
		if strings.HasPrefix(glyphName, prefix) && prefix != "" {
			namePart := strings.TrimPrefix(glyphName, prefix)
			underscored := strings.ReplaceAll(namePart, "-", "_")
			if underscored != namePart {
				testName := prefix + underscored
				if info, ok := glyphs[testName]; ok {
					return info.Char, testName
				}
			}
		} else {
			// For unprefixed names, try full conversion with prefix
			underscored := strings.ReplaceAll(glyphName, "-", "_")
			testName := prefix + underscored
			if info, ok := glyphs[testName]; ok {
				return info.Char, testName
			}
		}
	}

	// Third, use regex matching: convert - and _ to . (any char) to find flexible matches
	// This handles edge cases where the naming varies in unexpected ways
	glyphPattern := glyphName
	// Extract the prefix if present
	actualPrefix := ""
	for _, prefix := range prefixes[1:] {
		if strings.HasPrefix(glyphName, prefix) {
			actualPrefix = prefix
			glyphPattern = strings.TrimPrefix(glyphName, prefix)
			break
		}
	}

	// Convert hyphens and underscores to regex wildcards in the name part
	regexPattern := strings.ReplaceAll(glyphPattern, "-", "[-_]")
	regexPattern = strings.ReplaceAll(regexPattern, "_", "[-_]")

	// Search through all glyphs for a match
	prefixesToSearch := prefixes
	if actualPrefix != "" {
		// If input had a prefix, prioritize that prefix
		prefixesToSearch = []string{actualPrefix}
	}

	for _, prefix := range prefixesToSearch {
		fullPattern := "^" + regexp.QuoteMeta(prefix) + regexPattern + "$"
		re, err := regexp.Compile(fullPattern)
		if err != nil {
			continue
		}

		for name, info := range glyphs {
			if re.MatchString(name) {
				return info.Char, name
			}
		}
	}

	return "", ""
}
