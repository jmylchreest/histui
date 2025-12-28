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

// iconEntry defines a single icon to app names mapping.
// Using a slice allows duplicate icon keys that get merged at init time.
type iconEntry struct {
	Icon string
	Apps []string
}

// knownAppIconEntries defines icon mappings. Duplicate icon keys are merged
// automatically at init time with a warning logged.
var knownAppIconEntries = []iconEntry{
	// Messaging & Social
	{"discord", []string{"discord", "discord-canary", "discord-ptb", "vesktop", "webcord", "armcord", "betterdiscord"}},
	{"slack", []string{"slack", "slack-desktop", "slack-wayland"}},
	{"telegram", []string{"telegram", "telegram-desktop", "telegramdesktop", "org.telegram.desktop", "64gram", "kotatogram"}},
	{"whatsapp", []string{"whatsapp", "whatsapp-desktop", "zapzap", "whatsapp-for-linux", "whatsie", "elecwhat"}},
	{"message", []string{"signal", "signal-desktop", "signal-desktop-beta"}},       // Signal messenger
	{"message", []string{"element", "element-desktop", "fractal", "nheko", "neochat", "org.kde.neochat", "fluffychat"}}, // Matrix clients
	{"skype", []string{"skype", "skypeforlinux", "skype-electron"}},
	{"facebook", []string{"facebook", "caprine", "messenger", "facebookmessenger"}},
	{"twitter", []string{"twitter", "cawbird", "tweetdeck"}},
	{"mastodon", []string{"mastodon", "tootle", "whalebird", "sengi", "hyperspace"}},
	{"reddit", []string{"reddit", "giara"}},
	{"linkedin", []string{"linkedin"}},
	{"wechat", []string{"wechat", "electronic-wechat", "wechat-uos"}},

	// Browsers
	{"firefox", []string{"firefox", "firefox-esr", "firefox-developer-edition", "firefox-nightly", "librewolf", "floorp", "waterfox", "org.mozilla.firefox"}},
	{"chrome", []string{"google-chrome", "google-chrome-stable", "google-chrome-beta", "google-chrome-unstable", "chromium", "chromium-browser", "ungoogled-chromium"}},
	{"microsoft-edge", []string{"microsoft-edge", "microsoft-edge-stable", "microsoft-edge-beta", "microsoft-edge-dev"}},
	{"brave", []string{"brave", "brave-browser", "brave-browser-stable"}},
	{"opera", []string{"opera", "opera-stable", "opera-beta", "opera-developer"}},
	{"vivaldi", []string{"vivaldi", "vivaldi-stable", "vivaldi-snapshot"}},
	{"tor-browser", []string{"tor-browser", "torbrowser-launcher"}},
	{"epiphany", []string{"epiphany", "org.gnome.Epiphany", "gnome-web"}},
	{"qutebrowser", []string{"qutebrowser"}},
	{"min", []string{"min-browser", "min"}},
	{"safari", []string{"safari"}}, // rare on Linux but included

	// Email
	{"email", []string{"thunderbird", "thunderbird-daily", "thunderbird-beta", "evolution", "evolution-mail", "geary", "org.gnome.Geary", "mailspring", "tutanota", "protonmail-bridge", "kmail", "org.kde.kmail2", "claws-mail", "sylpheed", "mutt", "neomutt", "aerc"}},
	{"gmail", []string{"gmail", "gmail-desktop"}},
	{"outlook", []string{"outlook"}},

	// Media Players
	{"spotify", []string{"spotify", "spotify-client", "spotifyd", "spot", "psst"}},
	{"youtube", []string{"youtube", "freetube", "gtk-youtube-viewer", "minitube", "youtube-music"}},
	{"vlc", []string{"vlc", "org.videolan.VLC", "vlc-player"}},
	{"music", []string{"rhythmbox", "lollypop", "org.gnome.Lollypop", "elisa", "org.kde.elisa", "gnome-music", "org.gnome.Music", "clementine", "strawberry", "audacious", "quodlibet", "deadbeef", "cmus", "mpd", "ncmpcpp"}},
	{"video", []string{"mpv", "io.mpv.Mpv", "celluloid", "io.github.celluloid_player.Celluloid", "totem", "org.gnome.Totem", "gnome-videos", "haruna", "org.kde.haruna", "smplayer", "mplayer"}},
	{"podcast", []string{"gnome-podcasts", "org.gnome.Podcasts", "vocal", "gpodder"}},

	// Development
	{"visual-studio-code", []string{"code", "code-oss", "vscodium", "code-insiders", "codium", "visual-studio-code"}},
	{"git", []string{"git", "gitg", "gitk", "git-gui", "lazygit", "tig"}},
	{"github", []string{"github", "github-desktop", "gittyup"}},
	{"gitlab", []string{"gitlab"}},
	{"bitbucket", []string{"bitbucket"}},
	{"docker", []string{"docker", "docker-desktop", "podman", "podman-desktop"}},
	{"kubernetes", []string{"kubernetes", "kubectl", "k9s", "lens"}},
	{"database", []string{"dbeaver", "mysql-workbench", "pgadmin4", "mongodb-compass", "beekeeper-studio"}},
	{"terminal", []string{"gnome-terminal", "org.gnome.Terminal", "konsole", "org.kde.konsole", "kitty", "alacritty", "wezterm", "foot", "tilix", "terminator", "xfce4-terminal", "lxterminal", "xterm", "urxvt", "st", "hyper", "tabby", "contour", "rio", "ghostty"}},
	{"vim", []string{"vim", "neovim", "nvim", "gvim"}},
	{"emacs", []string{"emacs", "emacs-gtk", "doom-emacs", "spacemacs"}},

	// IDEs
	{"intellij", []string{"idea", "intellij-idea", "intellij-idea-ultimate", "intellij-idea-community", "jetbrains-idea"}},
	{"pycharm", []string{"pycharm", "pycharm-professional", "pycharm-community", "jetbrains-pycharm"}},
	{"webstorm", []string{"webstorm", "jetbrains-webstorm"}},
	{"goland", []string{"goland", "jetbrains-goland"}},
	{"clion", []string{"clion", "jetbrains-clion"}},
	{"rider", []string{"rider", "jetbrains-rider"}},
	{"android-studio", []string{"android-studio"}},
	{"sublime-text", []string{"sublime-text", "sublime_text", "subl"}},
	{"atom", []string{"atom"}},

	// File Managers
	{"folder", []string{"nautilus", "org.gnome.Nautilus", "org.gnome.Files", "nemo", "thunar", "dolphin", "org.kde.dolphin", "pcmanfm", "pcmanfm-qt", "spacefm", "ranger", "lf", "nnn", "vifm", "mc", "doublecmd"}},

	// Office & Productivity
	{"libreoffice", []string{"libreoffice", "libreoffice-writer", "libreoffice-calc", "libreoffice-impress", "libreoffice-draw", "libreoffice-base"}},
	{"onlyoffice", []string{"onlyoffice", "onlyoffice-desktopeditors"}},
	{"office", []string{"wps-office", "freeoffice"}},
	{"note", []string{"obsidian", "logseq", "notion", "notion-app", "joplin", "standard-notes", "simplenote", "zettlr", "marktext", "typora", "ghostwriter"}},
	{"calendar", []string{"gnome-calendar", "org.gnome.Calendar", "korganizer"}},

	// Graphics & Design
	{"image", []string{"gimp", "org.gimp.GIMP", "inkscape", "org.inkscape.Inkscape", "krita", "org.kde.krita", "blender", "darktable", "rawtherapee", "digikam", "shotwell", "eog", "org.gnome.eog", "gthumb", "org.gnome.gThumb", "gwenview", "org.kde.gwenview", "feh", "sxiv", "imv", "nomacs"}},

	// System & Utilities
	{"cog", []string{"gnome-control-center", "gnome-settings", "systemsettings", "org.kde.systemsettings", "xfce4-settings-manager", "lxappearance"}},
	{"lock", []string{"gnome-keyring", "keepassxc", "org.keepassxc.KeePassXC", "bitwarden", "1password"}},
	{"security-shield", []string{"firewall", "gufw", "firewalld"}},
	{"package", []string{"gnome-software", "org.gnome.Software", "discover", "org.kde.discover", "pamac", "octopi", "synaptic", "gdebi"}},
	{"update", []string{"software-update", "gnome-software", "update-manager"}},
	{"backup", []string{"deja-dup", "org.gnome.DejaDup", "timeshift", "borgbackup", "restic", "vorta"}},

	// Gaming
	{"steam", []string{"steam", "steam-runtime", "steam-native"}},
	{"gamepad", []string{"lutris", "heroic", "heroic-games-launcher", "bottles", "protonup-qt", "gamehub"}},

	// Cloud & Sync
	{"cloud", []string{"nextcloud", "owncloud", "dropbox", "insync", "megasync", "pcloud"}},
	{"google-drive", []string{"google-drive-ocamlfuse", "grive", "insync"}},

	// Network
	{"wifi", []string{"nm-applet", "network-manager-applet", "connman", "wicd"}},
	{"bluetooth", []string{"blueman", "blueman-manager", "blueberry", "gnome-bluetooth"}},
	{"vpn", []string{"openvpn", "wireguard", "protonvpn", "mullvad-vpn", "nordvpn"}},

	// Communication & Conferencing
	{"video-account", []string{"zoom", "zoom-client", "teams", "microsoft-teams", "webex", "jitsi", "jitsi-meet"}},

	// Torrent & Download
	{"download", []string{"transmission", "transmission-gtk", "transmission-qt", "qbittorrent", "deluge", "aria2", "uget", "jdownloader"}},

	// Screenshot & Recording
	{"monitor-screenshot", []string{"flameshot", "gnome-screenshot", "org.gnome.Screenshot", "spectacle", "org.kde.spectacle", "shutter", "scrot", "maim"}},
	{"video-vintage", []string{"obs", "obs-studio", "com.obsproject.Studio", "simplescreenrecorder", "kazam", "peek"}},

	// Virtualization
	{"desktop-classic", []string{"virt-manager", "gnome-boxes", "org.gnome.Boxes", "virtualbox", "vmware-workstation"}},
}

// knownAppIcons is the merged map built from knownAppIconEntries at init time.
var knownAppIcons map[string][]string

func init() {
	knownAppIcons = make(map[string][]string)
	for _, entry := range knownAppIconEntries {
		if existing, ok := knownAppIcons[entry.Icon]; ok {
			// Merge duplicate entries with a warning
			fmt.Fprintf(os.Stderr, "WARNING: merging duplicate icon key %q (%d + %d apps)\n",
				entry.Icon, len(existing), len(entry.Apps))
			knownAppIcons[entry.Icon] = append(existing, entry.Apps...)
		} else {
			knownAppIcons[entry.Icon] = entry.Apps
		}
	}
}

// additionalAppNames maps extra Linux app names that don't directly match icon names
var additionalAppNames = map[string]string{
	// Format: "linux-app-name": "icon-key-from-knownAppIcons"
	"org.telegram.desktop":     "telegram",
	"com.discordapp.Discord":   "discord",
	"com.slack.Slack":          "slack",
	"com.spotify.Client":       "spotify",
	"org.mozilla.firefox":      "firefox",
	"com.google.Chrome":        "chrome",
	"com.brave.Browser":        "brave",
	"org.videolan.VLC":         "vlc",
	"io.mpv.Mpv":               "video",
	"com.visualstudio.code":    "visual-studio-code",
	"com.jetbrains.IntelliJ-IDEA-Ultimate": "intellij",
	"org.keepassxc.KeePassXC":  "lock",
	"com.valvesoftware.Steam":  "steam",
	"com.obsproject.Studio":    "video-vintage",
	"md.obsidian.Obsidian":     "note",
	"com.bitwarden.desktop":    "lock",
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
	// Existing flags
	fetchFlag := flag.Bool("fetch", false, "Fetch fresh glyphnames.json and font from GitHub")
	outputFlag := flag.String("output", "icon-aliases.toml", "Output TOML file path")
	fontOutputFlag := flag.String("font-output", "", "Output path for Nerd Font symbols TTF (optional)")
	verboseFlag := flag.Bool("verbose", false, "Verbose logging")
	preferFlag := flag.String("prefer", "md", "Preferred icon set: md (Material Design), fa (Font Awesome), dev (Devicons)")

	// Knowledge base flags
	generateKBFlag := flag.Bool("generate-kb", false, "Generate AI knowledge base using OpenRouter (requires OPENROUTER_API_KEY)")
	openrouterModelFlag := flag.String("openrouter-model", "", "OpenRouter model to use (from config or anthropic/claude-sonnet-4)")
	webSearchFlag := flag.Bool("web-search", false, "Enable web search for real-time data (adds :online suffix)")
	noCacheFlag := flag.Bool("no-cache", false, "Disable caching of API responses")
	clearCacheFlag := flag.Bool("clear-cache", false, "Clear the API response cache before generating")
	defaultFileFlag := flag.String("default-file", kbDefaultFile, "Path to default knowledge base file")
	aiFileFlag := flag.String("ai-file", kbAIFile, "Path to AI knowledge base file")
	overridesFileFlag := flag.String("overrides-file", kbOverridesFile, "Path to overrides file")
	configFileFlag := flag.String("config", configFile, "Path to config file")
	writeExampleOverridesFlag := flag.Bool("write-example-overrides", false, "Write an example overrides file and exit")
	writeExampleConfigFlag := flag.Bool("write-example-config", false, "Write an example config file and exit")

	flag.Parse()

	// Set global preference
	iconPreference = *preferFlag

	// Handle example file generation
	if *writeExampleOverridesFlag {
		if err := WriteExampleOverrides(*overridesFileFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing example overrides: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote example overrides to %s\n", *overridesFileFlag)
		return
	}

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

	// Load or fetch glyph data
	glyphs, err := loadGlyphs(*fetchFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading glyphs: %v\n", err)
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

		// Show cache stats
		useCache := !*noCacheFlag
		if useCache {
			classifyCount, appGenCount, totalSize := CacheStats()
			if classifyCount > 0 || appGenCount > 0 {
				fmt.Printf("Cache: %d classify, %d app-gen entries (%.1f KB)\n",
					classifyCount, appGenCount, float64(totalSize)/1024)
			}
		}

		client, err := NewOpenRouterClient(*openrouterModelFlag, *webSearchFlag, useCache, config, *verboseFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating OpenRouter client: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Generating knowledge base using model: %s (web search: %v, cache: %v)\n",
			client.Model, client.WebSearch, client.UseCache)

		kb, err := client.GenerateKnowledgeBase(glyphs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating knowledge base: %v\n", err)
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

	// Find app-related glyphs for matching
	appGlyphs := filterAppGlyphs(glyphs, config.Filter)
	fmt.Printf("Found %d app-related glyphs\n", len(appGlyphs))

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

	// Load overrides (if exists)
	overrides, err := LoadOverrides(*overridesFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error loading overrides: %v\n", err)
	} else if overrides != nil {
		iconCount := len(overrides.Icons) + len(overrides.Additions)
		fmt.Printf("Loaded overrides (%d icon entries)\n", iconCount)
	}

	// Merge and generate mappings
	var mappings []AppMapping

	if defaultKB != nil || aiKB != nil || overrides != nil {
		// Merge all sources: override > AI > default
		fmt.Println("Using merged icon sources (override > AI > default)")
		merged := MergeIconSources(defaultKB, aiKB, overrides, *verboseFlag)
		mappings = ConvertMergedToAppMapping(merged, appGlyphs, *verboseFlag)
	} else {
		// Fallback: use hardcoded knownAppIcons (legacy)
		fmt.Println("Using legacy hardcoded mappings (no KB files found)")
		mappings = generateMappings(appGlyphs, *verboseFlag)
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

// filterAppGlyphs finds glyphs that are likely app icons using config filters
func filterAppGlyphs(glyphs map[string]GlyphInfo, filter FilterConfig) map[string]GlyphInfo {
	result := make(map[string]GlyphInfo)

	for name, glyph := range glyphs {
		// Check prefix
		hasAppPrefix := false
		for _, prefix := range filter.Prefixes {
			if strings.HasPrefix(name, prefix) {
				hasAppPrefix = true
				break
			}
		}
		if !hasAppPrefix {
			continue
		}

		// Check if name contains app-related keyword
		nameLower := strings.ToLower(name)
		for _, keyword := range filter.Keywords {
			if strings.Contains(nameLower, keyword) {
				result[name] = glyph
				break
			}
		}
	}

	return result
}

func generateMappings(appGlyphs map[string]GlyphInfo, verbose bool) []AppMapping {
	var mappings []AppMapping

	// Create a reverse lookup: keyword -> glyph names
	keywordGlyphs := make(map[string][]string)
	for glyphName := range appGlyphs {
		// Extract the icon name part (after prefix)
		parts := strings.SplitN(glyphName, "-", 3)
		if len(parts) < 3 {
			continue
		}
		iconName := parts[2] // e.g., "discord" from "nf-md-discord"
		keywordGlyphs[iconName] = append(keywordGlyphs[iconName], glyphName)
	}

	// For each known app icon, find matching glyphs
	for iconKey, appNames := range knownAppIcons {
		// Look for matching glyphs
		var bestMatch string
		var bestGlyph GlyphInfo

		// First check for explicit overrides
		if override, ok := explicitIconOverrides[iconKey]; ok {
			if glyph, ok := appGlyphs[override]; ok {
				bestMatch = override
				bestGlyph = glyph
			} else if verbose {
				fmt.Printf("  WARNING: explicit override %q not found in glyphs\n", override)
			}
		}

		// If no override, try auto-matching
		if bestMatch == "" {
			// Use configured prefix order (--prefer flag)
			prefixes := getPrefixOrder()

			for _, prefix := range prefixes {
				testName := prefix + iconKey
				if glyph, ok := appGlyphs[testName]; ok {
					bestMatch = testName
					bestGlyph = glyph
					break
				}
				// Try with underscores instead of hyphens
				testName = prefix + strings.ReplaceAll(iconKey, "-", "_")
				if glyph, ok := appGlyphs[testName]; ok {
					bestMatch = testName
					bestGlyph = glyph
					break
				}
			}
		}

		if bestMatch == "" {
			// Try fuzzy match on first word
			firstWord := strings.Split(iconKey, "-")[0]
			for glyphName, glyph := range appGlyphs {
				if strings.Contains(strings.ToLower(glyphName), firstWord) {
					bestMatch = glyphName
					bestGlyph = glyph
					break
				}
			}
		}

		if bestMatch != "" {
			mapping := AppMapping{
				AppNames:   appNames,
				IconName:   bestMatch,
				GlyphCode:  bestGlyph.Code,
				GlyphChar:  bestGlyph.Char,
				Confidence: "exact",
			}
			mappings = append(mappings, mapping)

			if verbose {
				fmt.Printf("  %s -> %s (U+%s)\n", iconKey, bestMatch, strings.ToUpper(bestGlyph.Code))
			}
		} else if verbose {
			fmt.Printf("  %s -> NO MATCH\n", iconKey)
		}
	}

	// Sort by icon name for consistent output
	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].IconName < mappings[j].IconName
	})

	return mappings
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

	for _, cat := range order {
		ms := categories[cat]
		if len(ms) == 0 {
			continue
		}

		fmt.Fprintf(f, "\n# %s\n", titles[cat])
		for _, m := range ms {
			// Extract the base icon name for the target (e.g., "discord" from "md-discord")
			// Nerd Font icons are prefix-name, e.g., "md-discord", "fa-telegram"
			targetIcon := m.IconName
			if idx := strings.Index(m.IconName, "-"); idx != -1 {
				targetIcon = m.IconName[idx+1:] // e.g., "discord" from "md-discord"
			}

			for _, appName := range m.AppNames {
				// Skip if app name equals target (no alias needed)
				if appName == targetIcon {
					continue
				}
				// Check for duplicates
				if existingTarget, exists := written[appName]; exists {
					fmt.Fprintf(os.Stderr, "WARNING: duplicate app name %q (already mapped to %q, skipping %q)\n",
						appName, existingTarget, targetIcon)
					continue
				}
				written[appName] = targetIcon
				// Quote keys that contain dots (TOML interprets dots as nested tables otherwise)
				if strings.Contains(appName, ".") {
					fmt.Fprintf(f, "%q = %q\n", appName, targetIcon)
				} else {
					fmt.Fprintf(f, "%s = %q\n", appName, targetIcon)
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
