package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	godbus "github.com/godbus/dbus/v5"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/dbus"
	"github.com/jmylchreest/histui/internal/theme"
)

var (
	themesListOpts struct {
		json bool
	}
	themesExtractOpts struct {
		dest  string
		force bool
	}
	themesPreviewOpts struct {
		duration time.Duration
	}
)

// themesCmd is the parent command for theme management.
var themesCmd = &cobra.Command{
	Use:   "themes",
	Short: "Manage notification popup themes",
	Long: `Manage the themes used to render notification popups (histuid).

Themes can be bundled (embedded in histui) or user-installed under
~/.config/histui/themes/. A user theme with the same name as a bundled
theme overrides it.

Run 'histui themes' (or 'histui themes list') to see what's available.`,
	RunE: themesListRun,
}

var themesListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List available themes",
	Long:    `List all available themes (bundled and user-installed). The active theme is marked with '*'.`,
	RunE:    themesListRun,
}

var themesCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Print the active theme name",
	Long:  `Print the name of the theme currently configured for histuid.`,
	RunE:  themesCurrentRun,
}

var themesApplyCmd = &cobra.Command{
	Use:     "apply <name>",
	Aliases: []string{"set", "use"},
	Short:   "Set the active theme",
	Long: `Set the active theme by writing it to the daemon config (histuid.toml).

If histuid is running it hot-reloads the theme immediately. Existing config
values are preserved, but TOML comments in the file are not.`,
	Args: cobra.ExactArgs(1),
	RunE: themesApplyRun,
}

var themesShowCmd = &cobra.Command{
	Use:     "show <name>",
	Aliases: []string{"info"},
	Short:   "Show details about a theme",
	Long:    `Show metadata and bundled assets for a theme.`,
	Args:    cobra.ExactArgs(1),
	RunE:    themesShowRun,
}

var themesPreviewCmd = &cobra.Command{
	Use:   "preview <name>",
	Short: "Temporarily apply a theme and fire sample notifications",
	Long: `Temporarily switch to a theme, fire low/normal/critical sample
notifications so you can see how it looks, then revert to the previous theme.

Requires the histuid daemon to be running.`,
	Args: cobra.ExactArgs(1),
	RunE: themesPreviewRun,
}

var themesExtractCmd = &cobra.Command{
	Use:     "extract <name>",
	Aliases: []string{"eject"},
	Short:   "Copy a bundled theme into your themes directory for customization",
	Long: `Copy a bundled theme (and all its assets) into ~/.config/histui/themes/<name>
so you can customize it. The extracted copy overrides the bundled theme.`,
	Args: cobra.ExactArgs(1),
	RunE: themesExtractRun,
}

var themesValidateCmd = &cobra.Command{
	Use:   "validate <name>",
	Short: "Validate a theme's CSS",
	Long:  `Check that a theme's CSS defines the required selectors and is structurally valid.`,
	Args:  cobra.ExactArgs(1),
	RunE:  themesValidateRun,
}

var themesPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the user themes directory",
	Long:  `Print the path to the user themes directory (~/.config/histui/themes).`,
	RunE:  themesPathRun,
}

var themesEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Open a user theme in your editor",
	Long: `Open a user theme's theme.css in $VISUAL/$EDITOR.

Bundled themes are read-only; run 'histui themes extract <name>' first to
create an editable copy.`,
	Args: cobra.ExactArgs(1),
	RunE: themesEditRun,
}

func init() {
	themesListCmd.Flags().BoolVar(&themesListOpts.json, "json", false, "Output as JSON")
	themesExtractCmd.Flags().StringVar(&themesExtractOpts.dest, "dest", "", "Destination directory (default: ~/.config/histui/themes/<name>)")
	themesExtractCmd.Flags().BoolVar(&themesExtractOpts.force, "force", false, "Overwrite the destination if it already exists")
	themesPreviewCmd.Flags().DurationVar(&themesPreviewOpts.duration, "duration", 8*time.Second, "How long to display the preview before reverting")

	themesCmd.AddCommand(
		themesListCmd,
		themesCurrentCmd,
		themesApplyCmd,
		themesShowCmd,
		themesPreviewCmd,
		themesExtractCmd,
		themesValidateCmd,
		themesPathCmd,
		themesEditCmd,
	)

	rootCmd.AddCommand(themesCmd)
}

// themeEntry describes a single available theme.
type themeEntry struct {
	name        string
	source      string // "bundled" or "user"
	overrides   bool   // user theme shadowing a bundled one of the same name
	path        string // user theme path (dir or .css file); empty for bundled
	description string
	author      string
	version     string
}

// bundledThemeSet returns the set of bundled theme names.
func bundledThemeSet() map[string]bool {
	m := make(map[string]bool, len(theme.BundledThemes))
	for _, n := range theme.BundledThemes {
		m[n] = true
	}
	return m
}

// userThemes scans the user themes directory and returns a map of theme name to
// its path (a directory for directory-based themes, or a .css file for
// single-file themes). Missing directory is not an error.
func userThemes() (map[string]string, error) {
	dir, err := theme.ThemesDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}

	out := map[string]string{}
	// Directory-based themes take precedence over single-file ones.
	for _, e := range entries {
		if e.IsDir() {
			if fileExists(filepath.Join(dir, e.Name(), "theme.css")) {
				out[e.Name()] = filepath.Join(dir, e.Name())
			}
		}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".css") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".css")
		if _, ok := out[base]; !ok {
			out[base] = filepath.Join(dir, e.Name())
		}
	}
	return out, nil
}

// collectThemes returns all available themes, sorted by name.
func collectThemes() ([]themeEntry, error) {
	bundled := bundledThemeSet()
	users, err := userThemes()
	if err != nil {
		return nil, err
	}

	names := map[string]bool{}
	for n := range bundled {
		names[n] = true
	}
	for n := range users {
		names[n] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)

	out := make([]themeEntry, 0, len(sorted))
	for _, n := range sorted {
		e := themeEntry{name: n}
		if p, ok := users[n]; ok {
			e.source = "user"
			e.path = p
			e.overrides = bundled[n]
			loadUserMeta(&e)
		} else {
			e.source = "bundled"
			loadBundledMeta(&e)
		}
		out = append(out, e)
	}
	return out, nil
}

// resolveTheme finds a theme by name. User themes take precedence over bundled
// themes of the same name.
func resolveTheme(name string) (*themeEntry, error) {
	users, err := userThemes()
	if err != nil {
		return nil, err
	}
	if p, ok := users[name]; ok {
		e := &themeEntry{name: name, source: "user", path: p, overrides: bundledThemeSet()[name]}
		loadUserMeta(e)
		return e, nil
	}
	if bundledThemeSet()[name] {
		e := &themeEntry{name: name, source: "bundled"}
		loadBundledMeta(e)
		return e, nil
	}
	return nil, fmt.Errorf("theme %q not found (run 'histui themes list' to see available themes)", name)
}

func loadBundledMeta(e *themeEntry) {
	data, ok := theme.GetEmbeddedManifest(e.name)
	if !ok {
		return
	}
	m, err := theme.ParseManifest([]byte(data), "")
	if err != nil {
		return
	}
	e.description, e.author, e.version = m.Description, m.Author, m.Version
}

func loadUserMeta(e *themeEntry) {
	info, err := os.Stat(e.path)
	if err != nil || !info.IsDir() {
		return
	}
	mp, ok := theme.FindManifest(e.path)
	if !ok {
		return
	}
	m, err := theme.LoadManifest(mp)
	if err != nil {
		return
	}
	e.description, e.author, e.version = m.Description, m.Author, m.Version
}

// themeCSS returns the CSS content for a theme.
func themeCSS(e *themeEntry) (string, error) {
	if e.source == "bundled" {
		css, ok := theme.GetEmbeddedTheme(e.name)
		if !ok {
			return "", fmt.Errorf("bundled theme %q has no CSS", e.name)
		}
		return css, nil
	}
	cssPath := e.path
	if info, err := os.Stat(e.path); err == nil && info.IsDir() {
		cssPath = filepath.Join(e.path, "theme.css")
	}
	data, err := os.ReadFile(cssPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// activeThemeName returns the theme name configured for the daemon, falling back
// to the default theme.
func activeThemeName() string {
	cfg, err := config.LoadDaemonConfig()
	if err != nil || cfg == nil || cfg.Theme.Name == "" {
		return theme.DefaultThemeName
	}
	return cfg.Theme.Name
}

// setThemeInConfig writes the theme name into the daemon config, preserving any
// existing keys. It reports whether the config file had to be created.
// Note: TOML comments in an existing file are not preserved.
func setThemeInConfig(name string) (created bool, err error) {
	path, err := config.DaemonConfigPath()
	if err != nil {
		return false, err
	}
	dir, err := config.DaemonConfigDir()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}

	root := map[string]any{}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return false, readErr
		}
		created = true
	} else if err := toml.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("existing config is not valid TOML: %w", err)
	}

	section, _ := root["theme"].(map[string]any)
	if section == nil {
		section = map[string]any{}
	}
	section["name"] = name
	root["theme"] = section

	out, err := toml.Marshal(root)
	if err != nil {
		return created, err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return created, err
	}
	return created, nil
}

func themesListRun(cmd *cobra.Command, args []string) error {
	entries, err := collectThemes()
	if err != nil {
		return err
	}
	active := activeThemeName()

	if themesListOpts.json {
		type jsonEntry struct {
			Name        string `json:"name"`
			Source      string `json:"source"`
			Active      bool   `json:"active"`
			Overrides   bool   `json:"overrides_bundled"`
			Description string `json:"description,omitempty"`
			Author      string `json:"author,omitempty"`
			Version     string `json:"version,omitempty"`
		}
		out := make([]jsonEntry, 0, len(entries))
		for _, e := range entries {
			out = append(out, jsonEntry{
				Name: e.name, Source: e.source, Active: e.name == active,
				Overrides: e.overrides, Description: e.description,
				Author: e.author, Version: e.version,
			})
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	nameW := len("NAME")
	for _, e := range entries {
		if len(e.name) > nameW {
			nameW = len(e.name)
		}
	}

	anyOverride := false
	fmt.Printf("  %-*s  %-16s  %s\n", nameW, "NAME", "SOURCE", "DESCRIPTION")
	for _, e := range entries {
		marker := " "
		if e.name == active {
			marker = "*"
		}
		src := e.source
		if e.overrides {
			src = "user (override)"
			anyOverride = true
		}
		fmt.Printf("%s %-*s  %-16s  %s\n", marker, nameW, e.name, src, e.description)
	}

	fmt.Println()
	fmt.Println("* = active theme")
	if anyOverride {
		fmt.Println("(override) = user theme replacing a bundled theme of the same name")
	}
	return nil
}

func themesCurrentRun(cmd *cobra.Command, args []string) error {
	fmt.Println(activeThemeName())
	return nil
}

func themesApplyRun(cmd *cobra.Command, args []string) error {
	name := args[0]
	if _, err := resolveTheme(name); err != nil {
		return err
	}

	created, err := setThemeInConfig(name)
	if err != nil {
		return err
	}

	client := dbus.NewDaemonClient(nil)
	defer func() { _ = client.Close() }()

	if client.IsAvailable() {
		fmt.Printf("Applied theme %q (daemon will hot-reload)\n", name)
	} else {
		fmt.Printf("Applied theme %q (takes effect when histuid starts)\n", name)
	}
	if created {
		if path, err := config.DaemonConfigPath(); err == nil {
			fmt.Printf("Created %s\n", path)
		}
	}
	return nil
}

func themesShowRun(cmd *cobra.Command, args []string) error {
	e, err := resolveTheme(args[0])
	if err != nil {
		return err
	}
	active := activeThemeName()

	source := e.source
	if e.overrides {
		source += " (overrides bundled)"
	}

	fmt.Printf("Name:        %s\n", e.name)
	fmt.Printf("Source:      %s\n", source)
	if e.source == "user" {
		fmt.Printf("Path:        %s\n", e.path)
	}
	if e.description != "" {
		fmt.Printf("Description: %s\n", e.description)
	}
	if e.author != "" {
		fmt.Printf("Author:      %s\n", e.author)
	}
	if e.version != "" {
		fmt.Printf("Version:     %s\n", e.version)
	}
	if e.name == active {
		fmt.Printf("Active:      yes\n")
	} else {
		fmt.Printf("Active:      no\n")
	}
	fmt.Printf("Assets:      %s\n", strings.Join(themeAssets(e), ", "))
	return nil
}

// themeAssets reports which optional asset files/dirs a theme provides.
func themeAssets(e *themeEntry) []string {
	var assets []string
	add := func(label string, present bool) {
		if present {
			assets = append(assets, label)
		}
	}

	if e.source == "bundled" {
		_, css := theme.GetEmbeddedTheme(e.name)
		_, layout := theme.GetEmbeddedLayout(e.name)
		_, aliases := theme.GetEmbeddedAliases(e.name)
		_, manifest := theme.GetEmbeddedManifest(e.name)
		add("css", css)
		add("layout", layout)
		add("aliases", aliases)
		add("manifest", manifest)
		add("sounds", embedDirExists("themes/"+e.name+"/sounds"))
		add("icons", embedDirExists("themes/"+e.name+"/icons"))
	} else if info, err := os.Stat(e.path); err == nil && info.IsDir() {
		add("css", fileExists(filepath.Join(e.path, "theme.css")))
		add("layout", fileExists(filepath.Join(e.path, "layout.xml")))
		add("aliases", fileExists(filepath.Join(e.path, "aliases.toml")))
		add("manifest", fileExists(filepath.Join(e.path, "manifest.toml")))
		add("sounds", dirExists(filepath.Join(e.path, "sounds")))
		add("icons", dirExists(filepath.Join(e.path, "icons")))
	} else {
		add("css", true) // single-file theme
	}

	if len(assets) == 0 {
		return []string{"none"}
	}
	return assets
}

func themesPreviewRun(cmd *cobra.Command, args []string) error {
	entry, err := resolveTheme(args[0])
	if err != nil {
		return err
	}

	client := dbus.NewDaemonClient(nil)
	defer func() { _ = client.Close() }()
	if !client.IsAvailable() {
		return fmt.Errorf("histuid is not running; preview needs the daemon to render notifications")
	}

	path, err := config.DaemonConfigPath()
	if err != nil {
		return err
	}
	origBytes, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	existed := readErr == nil
	prevName := activeThemeName()

	reverted := false
	revert := func() {
		if reverted {
			return
		}
		reverted = true
		if existed {
			_ = os.WriteFile(path, origBytes, 0o644)
		} else {
			// No config existed before; pin the previous theme so the daemon
			// visually reverts (removing the file does not trigger a reload).
			_, _ = setThemeInConfig(prevName)
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	if _, err := setThemeInConfig(entry.name); err != nil {
		return err
	}
	fmt.Printf("Previewing theme %q (was %q) for %s...\n", entry.name, prevName, themesPreviewOpts.duration)

	// Give the daemon a moment to hot-reload the theme.
	time.Sleep(700 * time.Millisecond)

	conn, err := godbus.SessionBus()
	if err != nil {
		revert()
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}
	sendPreviewNotifications(conn)

	select {
	case <-time.After(themesPreviewOpts.duration):
	case <-sig:
		fmt.Println("\ninterrupted, reverting...")
	}

	revert()
	fmt.Printf("Reverted to theme %q\n", prevName)
	if !existed {
		fmt.Printf("Note: created %s pinning theme %q (you had no daemon config before)\n", path, prevName)
	}
	return nil
}

// sendPreviewNotifications fires one notification per urgency level so the user
// can judge how a theme renders.
func sendPreviewNotifications(conn *godbus.Conn) {
	send := func(summary, body, icon string, urgency byte) {
		hints := map[string]godbus.Variant{"urgency": godbus.MakeVariant(urgency)}
		obj := conn.Object("org.freedesktop.Notifications", godbus.ObjectPath("/org/freedesktop/Notifications"))
		var id uint32
		_ = obj.Call("org.freedesktop.Notifications.Notify", 0,
			"histui", uint32(0), icon, summary, body, []string{}, hints, int32(10000),
		).Store(&id)
	}

	send("Theme preview — normal", "Normal urgency notification rendered with this theme.", "dialog-information", 1)
	time.Sleep(600 * time.Millisecond)
	send("Theme preview — low", "Low urgency styling. Check contrast of dimmed text.", "dialog-information", 0)
	time.Sleep(600 * time.Millisecond)
	send("Theme preview — critical", "Critical urgency styling. Is the warning state distinct?", "dialog-warning", 2)
}

func themesExtractRun(cmd *cobra.Command, args []string) error {
	name := args[0]
	if !bundledThemeSet()[name] {
		return fmt.Errorf("theme %q is not a bundled theme; only bundled themes can be extracted", name)
	}

	dest := themesExtractOpts.dest
	if dest == "" {
		dir, err := theme.ThemesDir()
		if err != nil {
			return err
		}
		dest = filepath.Join(dir, name)
	}

	if _, err := os.Stat(dest); err == nil {
		if !themesExtractOpts.force {
			return fmt.Errorf("destination %s already exists (use --force to overwrite)", dest)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	count, err := copyEmbeddedThemeDir(name, dest)
	if err != nil {
		return err
	}
	fmt.Printf("Extracted bundled theme %q to %s (%d files)\n", name, dest, count)
	fmt.Printf("Customize it with: histui themes edit %s\n", name)
	return nil
}

// copyEmbeddedThemeDir copies all files of a bundled theme to dest.
func copyEmbeddedThemeDir(name, dest string) (int, error) {
	root := "themes/" + name
	count := 0
	err := fs.WalkDir(theme.EmbeddedThemes, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := theme.EmbeddedThemes.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func themesValidateRun(cmd *cobra.Command, args []string) error {
	e, err := resolveTheme(args[0])
	if err != nil {
		return err
	}
	css, err := themeCSS(e)
	if err != nil {
		return err
	}

	problems := theme.ValidateCSS(css)
	if len(problems) == 0 {
		fmt.Printf("Theme %q: OK\n", e.name)
		return nil
	}

	fmt.Printf("Theme %q: %d problem(s)\n", e.name, len(problems))
	for _, p := range problems {
		fmt.Printf("  - %s\n", p)
	}
	return fmt.Errorf("theme %q failed validation", e.name)
}

func themesPathRun(cmd *cobra.Command, args []string) error {
	dir, err := theme.ThemesDir()
	if err != nil {
		return err
	}
	fmt.Println(dir)
	return nil
}

func themesEditRun(cmd *cobra.Command, args []string) error {
	name := args[0]
	users, err := userThemes()
	if err != nil {
		return err
	}
	p, ok := users[name]
	if !ok {
		if bundledThemeSet()[name] {
			return fmt.Errorf("theme %q is bundled and read-only; run 'histui themes extract %s' first to create an editable copy", name, name)
		}
		return fmt.Errorf("theme %q not found", name)
	}

	target := p
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		target = filepath.Join(p, "theme.css")
	}

	editor := firstNonEmpty(os.Getenv("VISUAL"), os.Getenv("EDITOR"))
	if editor == "" {
		for _, candidate := range []string{"nvim", "vim", "nano", "vi"} {
			if _, err := exec.LookPath(candidate); err == nil {
				editor = candidate
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("no editor found; set $EDITOR or $VISUAL")
	}

	parts := append(strings.Fields(editor), target)
	c := exec.Command(parts[0], parts[1:]...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}

// --- small fs helpers ---

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func embedDirExists(path string) bool {
	entries, err := fs.ReadDir(theme.EmbeddedThemes, path)
	return err == nil && len(entries) > 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
