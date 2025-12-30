package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// UpstreamIcon represents a parsed icon from any upstream source.
type UpstreamIcon struct {
	Name        string   // Canonical name (e.g., "discord")
	Patterns    []string // Glyph patterns to match (e.g., ["fa-discord", "md-discord"])
	SearchTerms []string // Search terms from upstream metadata
	Type        string   // "app" or "category"
	Description string   // Human-readable description
	Upstream    string   // Source: "fa", "md", "dev", "cod"
}

// KBPatterns represents the kb-patterns.toml structure.
type KBPatterns struct {
	Meta  KBMeta                  `toml:"meta"`
	Icons map[string]KBIconEntry `toml:"icons"`
}

// KBMeta contains metadata about the patterns file.
type KBMeta struct {
	Generated   string   `toml:"generated"`
	Sources     []string `toml:"sources"`
	Description string   `toml:"description"`
}

// KBIconEntry represents an icon entry in kb-patterns.toml.
type KBIconEntry struct {
	Patterns    []string `toml:"patterns"`
	SearchTerms []string `toml:"search_terms,omitempty"`
	Type        string   `toml:"type"`
	Description string   `toml:"description,omitempty"`
	Upstream    string   `toml:"upstream"`
	// Manual-only fields (loaded from kb-patterns-manual.toml)
	ForceApps []string `toml:"force_apps,omitempty"`
	ExtraApps []string `toml:"extra_apps,omitempty"`
}

// FontAwesome JSON structure
type FAMetadata map[string]FAIcon

type FAIcon struct {
	Styles   []string           `json:"styles"`
	Label    string             `json:"label"`
	Search   FASearch           `json:"search"`
	Unicode  string             `json:"unicode"`
	Aliases  FAAliases          `json:"aliases,omitempty"`
	Changes  []string           `json:"changes,omitempty"`
	Ligature []string           `json:"ligatures,omitempty"`
	SVG      map[string]FASVG   `json:"svg,omitempty"`
}

type FASearch struct {
	Terms []string `json:"terms"`
}

type FAAliases struct {
	Names    []string `json:"names,omitempty"`
	Unicodes FAUnicodes `json:"unicodes,omitempty"`
}

type FAUnicodes struct {
	Composite []string `json:"composite,omitempty"`
	Primary   []string `json:"primary,omitempty"`
	Secondary []string `json:"secondary,omitempty"`
}

type FASVG struct {
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Material Design Icons JSON structure (array format)
type MDIMetadata []MDIIcon

type MDIIcon struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Codepoint    string   `json:"codepoint"`
	Aliases      []string `json:"aliases"`
	Tags         []string `json:"tags"`
	Author       string   `json:"author"`
	Version      string   `json:"version"`
	Deprecated   bool     `json:"deprecated,omitempty"`
}

// Devicons JSON structure (array format)
type DeviconsMetadata []DeviconIcon

type DeviconIcon struct {
	Name     string   `json:"name"`
	Altnames []string `json:"altnames"`
	Tags     []string `json:"tags"`
	Versions struct {
		SVG  []string `json:"svg"`
		Font []string `json:"font"`
	} `json:"versions"`
	Color string `json:"color"`
}

// Codicons JSON structure (object mapping codepoint -> array of names)
type CodiconsMetadata map[string][]string

// FetchUpstream fetches metadata from all configured upstream sources.
func FetchUpstream(config *Config, verbose bool) ([]UpstreamIcon, error) {
	var allIcons []UpstreamIcon

	// Fetch Font Awesome
	if config.Upstream.FontAwesome != "" {
		if verbose {
			fmt.Println("Fetching Font Awesome metadata...")
		}
		icons, err := fetchFontAwesome(config.Upstream.FontAwesome, verbose)
		if err != nil {
			return nil, fmt.Errorf("font awesome: %w", err)
		}
		allIcons = append(allIcons, icons...)
		if verbose {
			fmt.Printf("  Found %d icons (%d app, %d category)\n",
				len(icons), countByType(icons, "app"), countByType(icons, "category"))
		}
	}

	// Fetch Material Design Icons
	if config.Upstream.MaterialDesign != "" {
		if verbose {
			fmt.Println("Fetching Material Design Icons metadata...")
		}
		icons, err := fetchMaterialDesign(config.Upstream.MaterialDesign, verbose)
		if err != nil {
			return nil, fmt.Errorf("material design: %w", err)
		}
		allIcons = append(allIcons, icons...)
		if verbose {
			fmt.Printf("  Found %d icons (%d app, %d category)\n",
				len(icons), countByType(icons, "app"), countByType(icons, "category"))
		}
	}

	// Fetch Devicons
	if config.Upstream.Devicons != "" {
		if verbose {
			fmt.Println("Fetching Devicons metadata...")
		}
		icons, err := fetchDevicons(config.Upstream.Devicons, verbose)
		if err != nil {
			return nil, fmt.Errorf("devicons: %w", err)
		}
		allIcons = append(allIcons, icons...)
		if verbose {
			fmt.Printf("  Found %d icons (all app type)\n", len(icons))
		}
	}

	// Fetch Codicons
	if config.Upstream.Codicons != "" {
		if verbose {
			fmt.Println("Fetching Codicons metadata...")
		}
		icons, err := fetchCodicons(config.Upstream.Codicons, verbose)
		if err != nil {
			return nil, fmt.Errorf("codicons: %w", err)
		}
		allIcons = append(allIcons, icons...)
		if verbose {
			fmt.Printf("  Found %d icons (%d app, %d category)\n",
				len(icons), countByType(icons, "app"), countByType(icons, "category"))
		}
	}

	return allIcons, nil
}

func countByType(icons []UpstreamIcon, typ string) int {
	count := 0
	for _, icon := range icons {
		if icon.Type == typ {
			count++
		}
	}
	return count
}

func fetchFontAwesome(url string, verbose bool) ([]UpstreamIcon, error) {
	data, err := fetchJSON(url)
	if err != nil {
		return nil, err
	}

	var metadata FAMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	var icons []UpstreamIcon
	for name, icon := range metadata {
		// Determine type: "brands" style = app, otherwise category
		iconType := "category"
		for _, style := range icon.Styles {
			if style == "brands" {
				iconType = "app"
				break
			}
		}

		// Build patterns - FA icons use fa- prefix in Nerd Fonts
		patterns := []string{"fa-" + name}

		// Add aliases as additional patterns
		for _, alias := range icon.Aliases.Names {
			patterns = append(patterns, "fa-"+alias)
		}

		icons = append(icons, UpstreamIcon{
			Name:        name,
			Patterns:    patterns,
			SearchTerms: icon.Search.Terms,
			Type:        iconType,
			Description: icon.Label,
			Upstream:    "fa",
		})
	}

	return icons, nil
}

func fetchMaterialDesign(url string, verbose bool) ([]UpstreamIcon, error) {
	data, err := fetchJSON(url)
	if err != nil {
		return nil, err
	}

	var metadata MDIMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	// Brand-related tags that indicate an app icon
	brandTags := map[string]bool{
		"Brand / Logo": true,
		"brand":        true,
		"logo":         true,
	}

	var icons []UpstreamIcon
	for _, icon := range metadata {
		if icon.Deprecated {
			continue
		}

		// Determine type based on tags
		iconType := "category"
		for _, tag := range icon.Tags {
			if brandTags[tag] {
				iconType = "app"
				break
			}
		}

		// Build patterns - MDI icons use md- or mdi- prefix in Nerd Fonts
		patterns := []string{"md-" + icon.Name}

		// Add aliases as additional patterns
		for _, alias := range icon.Aliases {
			patterns = append(patterns, "md-"+alias)
		}

		icons = append(icons, UpstreamIcon{
			Name:        icon.Name,
			Patterns:    patterns,
			SearchTerms: icon.Tags,
			Type:        iconType,
			Description: strings.ReplaceAll(icon.Name, "-", " "),
			Upstream:    "md",
		})
	}

	return icons, nil
}

func fetchDevicons(url string, verbose bool) ([]UpstreamIcon, error) {
	data, err := fetchJSON(url)
	if err != nil {
		return nil, err
	}

	var metadata DeviconsMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	var icons []UpstreamIcon
	for _, icon := range metadata {
		// All Devicons are developer tool/language logos - they're all app type
		patterns := []string{"dev-" + icon.Name}

		// Add altnames as additional patterns
		for _, alt := range icon.Altnames {
			patterns = append(patterns, "dev-"+alt)
		}

		// Combine tags and altnames for search terms
		searchTerms := append([]string{}, icon.Tags...)
		searchTerms = append(searchTerms, icon.Altnames...)

		icons = append(icons, UpstreamIcon{
			Name:        icon.Name,
			Patterns:    patterns,
			SearchTerms: searchTerms,
			Type:        "app", // All Devicons are app logos
			Description: icon.Name + " (developer tool/language)",
			Upstream:    "dev",
		})
	}

	return icons, nil
}

func fetchCodicons(url string, verbose bool) ([]UpstreamIcon, error) {
	data, err := fetchJSON(url)
	if err != nil {
		return nil, err
	}

	var metadata CodiconsMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	// Known VS Code related icons that should be app type
	appIcons := map[string]bool{
		"vscode":           true,
		"visual-studio":    true,
		"github":           true,
		"github-alt":       true,
		"github-inverted":  true,
		"azure":            true,
		"azure-devops":     true,
	}

	var icons []UpstreamIcon
	// Codicons maps codepoint -> []names (aliases)
	for _, names := range metadata {
		if len(names) == 0 {
			continue
		}

		// Primary name is first in array
		primaryName := names[0]

		iconType := "category"
		for _, name := range names {
			if appIcons[name] {
				iconType = "app"
				break
			}
		}

		// Build patterns - all names are aliases for same glyph
		var patterns []string
		for _, name := range names {
			patterns = append(patterns, "cod-"+name)
		}

		// All names except primary are search terms
		var searchTerms []string
		if len(names) > 1 {
			searchTerms = names[1:]
		}

		icons = append(icons, UpstreamIcon{
			Name:        primaryName,
			Patterns:    patterns,
			SearchTerms: searchTerms,
			Type:        iconType,
			Description: strings.ReplaceAll(primaryName, "-", " "),
			Upstream:    "cod",
		})
	}

	return icons, nil
}

func fetchJSON(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return data, nil
}

// GenerateKBPatterns creates kb-patterns.toml from upstream icons.
func GenerateKBPatterns(icons []UpstreamIcon, outputPath string, verbose bool) error {
	// Group icons by canonical name, merging patterns from multiple sources
	iconMap := make(map[string]*KBIconEntry)

	for _, icon := range icons {
		if existing, ok := iconMap[icon.Name]; ok {
			// Merge patterns from multiple sources
			existing.Patterns = append(existing.Patterns, icon.Patterns...)
			// Merge search terms
			termSet := make(map[string]bool)
			for _, t := range existing.SearchTerms {
				termSet[t] = true
			}
			for _, t := range icon.SearchTerms {
				if !termSet[t] {
					existing.SearchTerms = append(existing.SearchTerms, t)
				}
			}
			// Prefer "app" type if any source says it's an app
			if icon.Type == "app" {
				existing.Type = "app"
			}
			// Update upstream to show multiple sources
			if !strings.Contains(existing.Upstream, icon.Upstream) {
				existing.Upstream += "," + icon.Upstream
			}
		} else {
			entry := &KBIconEntry{
				Patterns:    icon.Patterns,
				SearchTerms: icon.SearchTerms,
				Type:        icon.Type,
				Description: icon.Description,
				Upstream:    icon.Upstream,
			}
			iconMap[icon.Name] = entry
		}
	}

	// Sort keys for deterministic output
	var names []string
	for name := range iconMap {
		names = append(names, name)
	}
	sort.Strings(names)

	// Build output structure
	patterns := &KBPatterns{
		Meta: KBMeta{
			Generated:   time.Now().UTC().Format(time.RFC3339),
			Sources:     []string{"font-awesome", "material-design-icons", "devicons", "codicons"},
			Description: "Auto-generated from upstream icon metadata. Do not edit - use kb-patterns-manual.toml for overrides.",
		},
		Icons: make(map[string]KBIconEntry),
	}

	for _, name := range names {
		entry := iconMap[name]
		// Sort patterns and search terms for deterministic output
		sort.Strings(entry.Patterns)
		sort.Strings(entry.SearchTerms)
		patterns.Icons[name] = *entry
	}

	// Write TOML
	var buf strings.Builder
	buf.WriteString("# Auto-generated icon patterns from upstream metadata\n")
	buf.WriteString("# DO NOT EDIT - this file is regenerated by --fetch\n")
	buf.WriteString("# Use kb-patterns-manual.toml for manual overrides\n\n")

	encoded, err := toml.Marshal(patterns)
	if err != nil {
		return fmt.Errorf("encode toml: %w", err)
	}
	buf.Write(encoded)

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	if verbose {
		fmt.Printf("Wrote %d icons to %s\n", len(patterns.Icons), outputPath)
	}

	return nil
}

// LoadKBPatterns loads kb-patterns.toml.
func LoadKBPatterns(path string) (*KBPatterns, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &KBPatterns{Icons: make(map[string]KBIconEntry)}, nil
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	var patterns KBPatterns
	if err := toml.Unmarshal(data, &patterns); err != nil {
		return nil, fmt.Errorf("parse toml: %w", err)
	}

	if patterns.Icons == nil {
		patterns.Icons = make(map[string]KBIconEntry)
	}

	return &patterns, nil
}

// MergePatterns merges manual patterns on top of auto-generated patterns.
// Manual entries completely override auto-generated ones for the same name.
func MergePatterns(auto, manual *KBPatterns) *KBPatterns {
	merged := &KBPatterns{
		Meta:  auto.Meta,
		Icons: make(map[string]KBIconEntry),
	}

	// Copy all auto-generated icons
	for name, entry := range auto.Icons {
		merged.Icons[name] = entry
	}

	// Override/add manual icons
	for name, entry := range manual.Icons {
		merged.Icons[name] = entry
	}

	return merged
}

const (
	kbPatternsFile       = "kb-patterns.toml"
	kbPatternsManualFile = "kb-patterns-manual.toml"
)

// RunFetch executes the --fetch workflow.
func RunFetch(config *Config, verbose bool) error {
	// Get working directory for output files
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Fetch upstream metadata
	icons, err := FetchUpstream(config, verbose)
	if err != nil {
		return fmt.Errorf("fetch upstream: %w", err)
	}

	if verbose {
		fmt.Printf("\nTotal: %d icons from upstream sources\n", len(icons))
	}

	// Generate kb-patterns.toml
	outputPath := filepath.Join(workDir, kbPatternsFile)
	if err := GenerateKBPatterns(icons, outputPath, verbose); err != nil {
		return fmt.Errorf("generate patterns: %w", err)
	}

	return nil
}
