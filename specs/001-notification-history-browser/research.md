# Phase 0 Research: histui - Notification History Browser

**Date**: 2025-12-26
**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

## Research Summary

All technology decisions have been validated. No NEEDS CLARIFICATION items remain.

---

## 1. Cobra CLI Framework

### Decision
Use **Cobra** for CLI framework with **subcommand** design.

### Rationale
- Industry standard for Go CLIs (kubectl, gh, docker use it)
- Constitution specifies Cobra
- Subcommands provide clear separation of concerns
- Default command (no args) runs TUI for quick access

### Command Structure

```bash
# Commands
histui                  # Default: TUI mode
histui get [flags]      # Query and output notifications
histui status           # Waybar JSON output
histui tui              # Interactive TUI (explicit)
histui prune [flags]    # Clean up old notifications

# Get command - output all notifications
histui get --format dmenu              # dmenu-style list
histui get --format json               # JSON array
histui get --app --title --body        # Specific fields
histui get --format dmenu --ulid       # Include ULID for piping

# Get command - lookup specific notification (stdin)
histui get --format dmenu --ulid | fuzzel -d | histui get --body

# Filtering
histui get --since 48h                 # Last 48h (default)
histui get --since 1h                  # Last hour
histui get --since 0                   # All time
histui get --app-filter firefox        # Filter by app
histui get --urgency critical          # Filter by urgency
histui get --limit 20                  # Limit count

# Sorting
histui get --sort timestamp:desc       # Newest first (default)
histui get --sort timestamp:asc        # Oldest first
histui get --sort app:asc              # Alphabetical

# Prune
histui prune                           # Remove older than 48h
histui prune --older-than 7d           # Custom threshold
histui prune --keep 100                # Keep at most N
histui prune --dry-run                 # Preview changes
```

### Subcommand Implementation

```go
// cmd/histui/root.go
var rootCmd = &cobra.Command{
    Use:   "histui",
    Short: "Notification history browser",
    Run: func(cmd *cobra.Command, args []string) {
        // Default to TUI mode
        tuiCmd.Run(cmd, args)
    },
}

// cmd/histui/get.go
var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Query and output notifications",
    RunE:  runGet,
}

func init() {
    // Field flags
    getCmd.Flags().BoolP("app", "a", false, "Include app name")
    getCmd.Flags().BoolP("title", "t", false, "Include title/summary")
    getCmd.Flags().BoolP("body", "b", false, "Include body")
    getCmd.Flags().BoolP("timestamp", "T", false, "Include timestamp")
    getCmd.Flags().Bool("time-relative", false, "Include relative time")
    getCmd.Flags().Bool("ulid", false, "Include ULID")
    getCmd.Flags().Bool("all", false, "Include all fields")

    // Format
    getCmd.Flags().String("format", "", "Output format: dmenu, json, or template")

    // Filtering
    getCmd.Flags().String("since", "48h", "Time filter (e.g., 1h, 48h, 7d, 0=all)")
    getCmd.Flags().String("app-filter", "", "Filter by app name")
    getCmd.Flags().String("urgency", "", "Filter by urgency: low, normal, critical")
    getCmd.Flags().Int("limit", 0, "Maximum notifications (0=unlimited)")

    // Sorting
    getCmd.Flags().String("sort", "timestamp:desc", "Sort: field:order")

    rootCmd.AddCommand(getCmd)
}
```

### Version Injection

```go
// internal/version/version.go
package version

var (
    Version   = "dev"
    GitCommit = "unknown"
    BuildDate = "unknown"
)

// Build command:
// go build -ldflags="-X 'histui/internal/version.Version=1.0.0' \
//                    -X 'histui/internal/version.GitCommit=$(git rev-parse --short HEAD)' \
//                    -X 'histui/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)' \
//                    -s -w"
```

### Error Handling Pattern

```go
// Return errors, don't os.Exit() in commands
func runList(cmd *cobra.Command, args []string) error {
    source, err := getSource(cmd)
    if err != nil {
        return fmt.Errorf("invalid source: %w", err)
    }
    // ...
}

// Exit in main only
func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
```

---

## 2. BubbleTea TUI Framework

### Decision
Use **BubbleTea + Bubbles list component + Lipgloss** for TUI mode.

### Rationale
- Elm architecture (Model-Update-View) is well-suited for reactive UIs
- Constitution specifies BubbleTea
- Bubbles provides production-ready list component with filtering

### Elm Architecture Pattern

```go
type model struct {
    list         list.Model
    notifications []notification.Notification
    store        *store.Store
    detailView   bool
    selected     *notification.Notification
    width        int
    height       int
}

func (m model) Init() tea.Cmd {
    return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "enter":
            return m, m.selectNotification()
        case "/":
            return m, m.startSearch()
        }
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case storeUpdateMsg:
        return m, m.refreshList()
    }

    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}

func (m model) View() string {
    if m.detailView {
        return m.renderDetailView()
    }
    return m.list.View()
}
```

### External Channel Subscription (for reactive updates)

```go
// Subscribe to store change notifications
func (m model) Init() tea.Cmd {
    return tea.Batch(
        m.list.Init(),
        m.waitForStoreUpdate(),
    )
}

type storeUpdateMsg struct{}

func (m model) waitForStoreUpdate() tea.Cmd {
    return func() tea.Msg {
        <-m.store.Changes() // Block on channel
        return storeUpdateMsg{}
    }
}
```

### List Component with Filtering

```go
import "github.com/charmbracelet/bubbles/list"

// Item implementation
type notificationItem struct {
    n notification.Notification
}

func (i notificationItem) Title() string {
    return fmt.Sprintf("%s: %s", i.n.AppName, i.n.Summary)
}

func (i notificationItem) Description() string {
    return i.n.RelativeTime()
}

func (i notificationItem) FilterValue() string {
    return i.n.AppName + " " + i.n.Summary + " " + i.n.Body
}

// Initialize list
items := make([]list.Item, len(notifications))
for i, n := range notifications {
    items[i] = notificationItem{n}
}

l := list.New(items, list.NewDefaultDelegate(), width, height)
l.Title = "Notification History"
l.SetFilteringEnabled(true)
```

### Keyboard Navigation

| Key | Action |
|-----|--------|
| `j/k` or `↓/↑` | Navigate list |
| `Enter` | View notification detail |
| `y` | Copy to clipboard |
| `/` | Start filter/search |
| `Esc` | Clear filter/back |
| `q` | Quit |

### Lipgloss Styling

```go
import "github.com/charmbracelet/lipgloss"

var (
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("212"))

    selectedStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("229")).
        Background(lipgloss.Color("57"))

    urgencyStyles = map[string]lipgloss.Style{
        "low":      lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
        "normal":   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
        "critical": lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
    }
)
```

---

## 3. Terminal Image Rendering

### Decision
Use **blacktop/go-termimg** for rendering notification icons in the TUI detail view.

### Rationale
- Modern, actively maintained library with comprehensive protocol support
- Automatic protocol detection and graceful fallback
- Supports all major terminal image protocols
- Simple API for image rendering within terminal applications

### Supported Protocols

| Protocol | Terminals | Features |
|----------|-----------|----------|
| **Kitty** | Kitty, Ghostty, WezTerm | Best quality, virtual images, z-index, compression, animated GIF/PNG/WebP (0.45+) |
| **Sixel** | mlterm, xterm, mintty, foot | High quality with palette optimization and dithering |
| **iTerm2** | iTerm2, VS Code | Native inline images with ECH clearing |
| **Halfblocks** | Everything | Unicode fallback using block characters |

### Usage Pattern

```go
import (
    "github.com/blacktop/go-termimg"
    "image"
    _ "image/png"
    "os"
)

// Render notification icon
func renderIcon(iconPath string, w io.Writer) error {
    // Open icon file
    f, err := os.Open(iconPath)
    if err != nil {
        return err // Gracefully skip icon
    }
    defer f.Close()

    // Decode image
    img, _, err := image.Decode(f)
    if err != nil {
        return err
    }

    // Create encoder with auto-detection
    enc, err := termimg.NewEncoder(w)
    if err != nil {
        return err // No supported protocol
    }

    // Render image (scaled to 64x64 cell area)
    return enc.Encode(img, termimg.WithSize(64, 64))
}
```

### Protocol Detection

```go
import "github.com/blacktop/go-termimg"

// Check what's available
func detectImageSupport() string {
    enc, err := termimg.NewEncoder(os.Stdout)
    if err != nil {
        return "none" // Fallback to [icon] placeholder
    }
    return enc.Protocol() // "kitty", "sixel", "iterm", "halfblocks"
}
```

### Integration with BubbleTea

Terminal graphics protocols write escape sequences directly to stdout. In BubbleTea:

```go
func (m model) renderDetailView() string {
    var buf strings.Builder

    // Header and text content
    buf.WriteString(m.renderHeader())

    // Icon placeholder - actual rendering happens in View()
    buf.WriteString("  ")
    buf.WriteString(m.iconPlaceholder()) // Reserved space

    // Rest of content
    buf.WriteString(m.renderNotificationText())

    return buf.String()
}

// Render icon after BubbleTea draws frame
func (m *model) renderIconOverlay() tea.Cmd {
    return func() tea.Msg {
        if m.selected == nil || m.selected.IconPath == "" {
            return nil
        }

        // Move cursor to icon position and render
        // This uses raw terminal output, coordinated with BubbleTea
        renderIconAt(m.selected.IconPath, m.iconX, m.iconY)
        return nil
    }
}
```

### Kitty 0.45+ Animation Support

Kitty 0.45 (December 2025) added native support for:
- Animated PNG (APNG)
- Animated WebP
- ICC color profiles
- CCIP color space

For animated icons (rare but possible for custom notifications):

```go
// Kitty-specific animation
import "github.com/dolmen-go/kittyimg"

func renderAnimatedIcon(path string, w io.Writer) error {
    f, _ := os.Open(path)
    defer f.Close()

    // kittyimg handles animation frames automatically
    return kittyimg.Fprint(w, f)
}
```

### Fallback Behavior

When no image protocol is available or icon rendering fails:

```
┌─────────┐
│ [icon]  │   Download Complete
│         │   ──────────────────
└─────────┘   ...
```

The placeholder uses Unicode box drawing characters and displays `[icon]` or the app name's first letter.

---

## 4. ULID Implementation

### Decision
Use **oklog/ulid/v2** for unique, sortable notification IDs.

### Rationale
- Industry standard Go ULID library (1,600+ dependents)
- Lexicographically sortable by creation time (ideal for time-series data)
- 128-bit (same as UUID) but 26 characters vs 36
- Constitution schema specifies ULIDs

### Generation Pattern

```go
import "github.com/oklog/ulid/v2"

// Simple generation (recommended for most use cases)
id := ulid.Make()

// For explicit entropy control
import "crypto/rand"
id := ulid.MustNew(ulid.Now(), rand.Reader)
```

### Storage Format

| Format | Size | Use Case |
|--------|------|----------|
| Text | 26 chars | JSONL storage, logs, APIs |
| Binary | 16 bytes | Database primary keys |

```go
// Text representation (for JSONL)
idStr := id.String() // "01ARZ3NDEKTSV4RRFFQ69G5FAV"

// Parse from string
id, err := ulid.Parse("01ARZ3NDEKTSV4RRFFQ69G5FAV")

// Extract timestamp
ms := id.Time() // milliseconds since Unix epoch
```

### Performance
- Parsing: ~30 ns/op
- Generation: ~65 ns/op
- Zero allocations for parsing

---

## 5. JSONL File Handling

### Decision
Use **bufio.Scanner** for streaming reads, atomic writes with fsync.

### Rationale
- Memory-efficient line-by-line processing
- Crash-safe appends with sync
- Simple format, easy debugging

### Reading Pattern

```go
func ReadHistory(filename string) ([]Notification, error) {
    file, err := os.Open(filename)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil // Empty history is valid
        }
        return nil, err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    buf := make([]byte, 0, 64*1024)
    scanner.Buffer(buf, 1024*1024) // 1MB max line

    var notifications []Notification
    lineNum := 0

    for scanner.Scan() {
        lineNum++
        line := scanner.Bytes()
        if len(line) == 0 {
            continue
        }

        var n Notification
        if err := json.Unmarshal(line, &n); err != nil {
            // Log and skip corrupted lines
            slog.Warn("skipping corrupted line", "line", lineNum, "error", err)
            continue
        }
        notifications = append(notifications, n)
    }

    return notifications, scanner.Err()
}
```

### Append Pattern (Crash-Safe)

```go
func AppendNotification(filename string, n Notification) error {
    file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
    if err != nil {
        return err
    }
    defer file.Close()

    data, err := json.Marshal(n)
    if err != nil {
        return err
    }

    // Write with newline (atomic up to PIPE_BUF)
    data = append(data, '\n')
    if _, err := file.Write(data); err != nil {
        return err
    }

    // Sync for durability
    return file.Sync()
}
```

### Error Recovery

```go
// Schema version in first line
type HistoryHeader struct {
    HistuiSchemaVersion int   `json:"histui_schema_version"`
    CreatedAt           int64 `json:"created_at"`
}

// Backup corrupted file before recreation
func backupCorrupted(filename string) error {
    backup := filename + ".backup." + time.Now().Format("20060102-150405")
    return os.Rename(filename, backup)
}
```

---

## 6. Dunstctl History Format

### Decision
Parse `dunstctl history` JSON output with timestamp conversion.

### Rationale
- Only notification daemon with history export
- JSON format is stable
- Timestamp conversion is documented

### JSON Structure

```json
{
  "type": "aa{sv}",
  "data": [[
    {
      "appname": {"type": "s", "data": "firefox"},
      "summary": {"type": "s", "data": "Download Complete"},
      "body": {"type": "s", "data": "myfile.zip"},
      "id": {"type": "i", "data": 1530},
      "timestamp": {"type": "x", "data": 241591199737},
      "timeout": {"type": "x", "data": 10000000},
      "urgency": {"type": "s", "data": "NORMAL"},
      "category": {"type": "s", "data": ""},
      "icon_path": {"type": "s", "data": ""},
      "progress": {"type": "i", "data": -1},
      "stack_tag": {"type": "s", "data": ""},
      "urls": {"type": "s", "data": ""}
    }
  ]]
}
```

### Critical Conversions

| Field | Dunst Format | Conversion |
|-------|--------------|------------|
| timestamp | Microseconds since boot | `boot_time + (timestamp / 1_000_000)` |
| timeout | Microseconds | `timeout / 1_000` (to milliseconds) |
| urgency | String ("LOW", "NORMAL", "CRITICAL") | Map to int (0, 1, 2) |

### Boot Time Calculation

```go
import "syscall"

func getBootTime() (int64, error) {
    var info syscall.Sysinfo_t
    if err := syscall.Sysinfo(&info); err != nil {
        return 0, err
    }
    return time.Now().Unix() - info.Uptime, nil
}

func convertDunstTimestamp(dunstTS int64) (int64, error) {
    bootTime, err := getBootTime()
    if err != nil {
        return 0, err
    }
    return bootTime + (dunstTS / 1_000_000), nil
}
```

### Parser Structure

```go
type DunstHistory struct {
    Type string              `json:"type"`
    Data [][]DunstNotification `json:"data"`
}

type DunstField[T any] struct {
    Type string `json:"type"`
    Data T      `json:"data"`
}

type DunstNotification struct {
    AppName   DunstField[string] `json:"appname"`
    Summary   DunstField[string] `json:"summary"`
    Body      DunstField[string] `json:"body"`
    ID        DunstField[int]    `json:"id"`
    Timestamp DunstField[int64]  `json:"timestamp"`
    Timeout   DunstField[int64]  `json:"timeout"`
    Urgency   DunstField[string] `json:"urgency"`
    Category  DunstField[string] `json:"category"`
    IconPath  DunstField[string] `json:"icon_path"`
    Progress  DunstField[int]    `json:"progress"`
    StackTag  DunstField[string] `json:"stack_tag"`
    URLs      DunstField[string] `json:"urls"`
}
```

---

## 7. Clipboard Integration

### Decision
Use **wl-copy** primary with **xclip/xsel** fallback.

### Rationale
- Wayland-native solution for Hyprland users
- X11 fallback for XWayland applications
- Constitution specifies wl-copy

### Detection and Fallback

```go
package clipboard

import (
    "bytes"
    "fmt"
    "os"
    "os/exec"
)

func Copy(text string) error {
    tool, args, err := detectTool()
    if err != nil {
        return err
    }

    cmd := exec.Command(tool, args...)
    cmd.Stdin = bytes.NewReader([]byte(text))

    var stderr bytes.Buffer
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        if stderr.Len() > 0 {
            return fmt.Errorf("%s failed: %s", tool, stderr.String())
        }
        return fmt.Errorf("%s failed: %w", tool, err)
    }

    return nil
}

func detectTool() (string, []string, error) {
    // Prefer wl-copy on Wayland
    if os.Getenv("WAYLAND_DISPLAY") != "" {
        if _, err := exec.LookPath("wl-copy"); err == nil {
            return "wl-copy", []string{"-f"}, nil
        }
    }

    // X11 fallback
    if _, err := exec.LookPath("xclip"); err == nil {
        return "xclip", []string{"-selection", "clipboard", "-i"}, nil
    }

    if _, err := exec.LookPath("xsel"); err == nil {
        return "xsel", []string{"-b", "-i"}, nil
    }

    return "", nil, fmt.Errorf("no clipboard tool found\n\n" +
        "Install one of:\n" +
        "  - wl-clipboard (for Wayland)\n" +
        "  - xclip (for X11/XWayland)\n" +
        "  - xsel (for X11/XWayland)")
}
```

---

## 8. Waybar Status Output

### Decision
Produce JSON matching waybar custom module format.

### Rationale
- Direct integration with user's existing waybar
- Standard format documented in waybar wiki

### Output Format

```json
{
  "text": "",
  "alt": "enabled",
  "tooltip": "Notifications enabled\n3 in history",
  "class": "enabled"
}
```

### Status States

| State | alt | class | icon (in waybar config) |
|-------|-----|-------|-------------------------|
| Enabled | "enabled" | "enabled" | 󰂚 |
| Paused | "paused" | "paused" | 󰂛 |
| Paused with count | "paused-5" | "paused" | 󰂛 (5) |
| Unavailable | "unavailable" | "error" | 󰂭 |

### Implementation

```go
type WaybarStatus struct {
    Text    string `json:"text"`
    Alt     string `json:"alt"`
    Tooltip string `json:"tooltip"`
    Class   string `json:"class"`
}

func formatStatus(paused bool, historyCount int) WaybarStatus {
    if paused {
        alt := "paused"
        if historyCount > 0 {
            alt = fmt.Sprintf("paused-%d", historyCount)
        }
        return WaybarStatus{
            Text:    "",
            Alt:     alt,
            Tooltip: fmt.Sprintf("Notifications paused\n%d in history", historyCount),
            Class:   "paused",
        }
    }

    return WaybarStatus{
        Text:    "",
        Alt:     "enabled",
        Tooltip: fmt.Sprintf("Notifications enabled\n%d in history", historyCount),
        Class:   "enabled",
    }
}
```

---

## 9. Filtering, Sorting, and Lookup

### Duration Parsing

The `--since` flag accepts human-readable durations:

```go
import "time"

// parseDuration parses duration strings like "48h", "7d", "1h30m"
func parseDuration(s string) (time.Duration, error) {
    if s == "0" {
        return 0, nil // Special case: no time filter
    }

    // Handle days (not in stdlib)
    if strings.HasSuffix(s, "d") {
        days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
        if err != nil {
            return 0, err
        }
        return time.Duration(days) * 24 * time.Hour, nil
    }

    // Standard Go duration parsing
    return time.ParseDuration(s)
}
```

### Filter Options

```go
type FilterOptions struct {
    Since      time.Duration // Filter to notifications newer than now-since (0=all)
    AppFilter  string        // Exact match on app name
    Urgency    *int          // Filter by urgency level (nil=any)
    Limit      int           // Maximum results (0=unlimited)
    SortField  string        // Field to sort by: "timestamp", "app", "urgency"
    SortOrder  string        // "asc" or "desc"
}

func (f *FilterOptions) Apply(notifications []Notification) []Notification {
    result := notifications

    // Time filter
    if f.Since > 0 {
        cutoff := time.Now().Add(-f.Since)
        result = filterByTime(result, cutoff)
    }

    // App filter
    if f.AppFilter != "" {
        result = filterByApp(result, f.AppFilter)
    }

    // Urgency filter
    if f.Urgency != nil {
        result = filterByUrgency(result, *f.Urgency)
    }

    // Sort
    result = sortNotifications(result, f.SortField, f.SortOrder)

    // Limit
    if f.Limit > 0 && len(result) > f.Limit {
        result = result[:f.Limit]
    }

    return result
}
```

### Sorting Implementation

```go
func sortNotifications(ns []Notification, field, order string) []Notification {
    result := make([]Notification, len(ns))
    copy(result, ns)

    sort.Slice(result, func(i, j int) bool {
        var less bool
        switch field {
        case "app":
            less = result[i].AppName < result[j].AppName
        case "urgency":
            less = result[i].Urgency < result[j].Urgency
        default: // "timestamp"
            less = result[i].Timestamp < result[j].Timestamp
        }

        if order == "desc" {
            return !less
        }
        return less
    })

    return result
}
```

### Notification Lookup

The `get` command performs lookup when it receives input on stdin:

```go
// LookupNotification finds a notification matching the input line.
// Priority: ULID match > content match
func LookupNotification(input string, notifications []Notification) (*Notification, error) {
    input = strings.TrimSpace(input)
    if input == "" {
        return nil, fmt.Errorf("empty input")
    }

    // Try ULID match first (tab-separated first field)
    if fields := strings.SplitN(input, "\t", 2); len(fields) >= 1 {
        if id, err := ulid.Parse(fields[0]); err == nil {
            for i := range notifications {
                if notifications[i].HistuiID == id {
                    return &notifications[i], nil
                }
            }
        }
    }

    // Fall back to content matching
    // Parse "app | summary - body | time" format
    for i := range notifications {
        if notificationMatchesLine(&notifications[i], input) {
            return &notifications[i], nil
        }
    }

    return nil, fmt.Errorf("no matching notification found")
}
```

### Prune Utility (Reusable)

```go
// PruneOptions configures the prune operation.
type PruneOptions struct {
    OlderThan time.Duration // Remove notifications older than this
    Keep      int           // Keep at most N notifications (0=unlimited)
    DryRun    bool          // If true, don't actually remove
}

// Prune removes old notifications from the store.
// Returns the count of removed notifications.
func (s *Store) Prune(opts PruneOptions) (int, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    cutoff := time.Now().Add(-opts.OlderThan)
    var keep, remove []Notification

    for _, n := range s.notifications {
        if time.Unix(n.Timestamp, 0).Before(cutoff) {
            remove = append(remove, n)
        } else {
            keep = append(keep, n)
        }
    }

    // Apply --keep limit if specified
    if opts.Keep > 0 && len(keep) > opts.Keep {
        // Sort by timestamp desc, keep newest
        sort.Slice(keep, func(i, j int) bool {
            return keep[i].Timestamp > keep[j].Timestamp
        })
        remove = append(remove, keep[opts.Keep:]...)
        keep = keep[:opts.Keep]
    }

    if opts.DryRun {
        return len(remove), nil
    }

    s.notifications = keep
    if s.persistEnable {
        if err := s.rewritePersistence(); err != nil {
            return 0, err
        }
    }

    return len(remove), nil
}
```

---

## 10. Configuration File

### Decision
Use **TOML** format with **pelletier/go-toml/v2** library.

### Rationale
- TOML is human-readable and edit-friendly (better than JSON for config)
- Well-suited for hierarchical configuration with sections
- pelletier/go-toml/v2 is the most performant and actively maintained Go TOML library
- Follows XDG Base Directory specification for location

### File Location

```go
import "os"

func configPath() string {
    // XDG_CONFIG_HOME or default
    configDir := os.Getenv("XDG_CONFIG_HOME")
    if configDir == "" {
        home, _ := os.UserHomeDir()
        configDir = filepath.Join(home, ".config")
    }
    return filepath.Join(configDir, "histui", "config.toml")
}
```

### Configuration Structure

```go
import "github.com/pelletier/go-toml/v2"

type Config struct {
    Filter    FilterConfig    `toml:"filter"`
    Sort      SortConfig      `toml:"sort"`
    Prune     PruneConfig     `toml:"prune"`
    Templates TemplateConfig  `toml:"templates"`
    TUI       TUIConfig       `toml:"tui"`
    Clipboard ClipboardConfig `toml:"clipboard"`
}

type FilterConfig struct {
    Since string `toml:"since"` // Duration string: "48h", "7d", "0"
    Limit int    `toml:"limit"`
}

type SortConfig struct {
    Field string `toml:"field"` // "timestamp", "app", "urgency"
    Order string `toml:"order"` // "asc", "desc"
}

type PruneConfig struct {
    OlderThan string `toml:"older_than"`
    Keep      int    `toml:"keep"`
}

type TemplateConfig struct {
    Dmenu     string            `toml:"dmenu"`
    Full      string            `toml:"full"`
    Body      string            `toml:"body"`
    JSON      string            `toml:"json"`
    TUIOutput string            `toml:"tui_output"`
    Custom    map[string]string `toml:"custom"`
}

type TUIConfig struct {
    ShowIcons bool `toml:"show_icons"`
    IconSize  int  `toml:"icon_size"`
    ShowHelp  bool `toml:"show_help"`
}

type ClipboardConfig struct {
    Command string `toml:"command"`
}
```

### Loading with Defaults

```go
func LoadConfig() (*Config, error) {
    cfg := &Config{
        Filter: FilterConfig{Since: "48h", Limit: 0},
        Sort:   SortConfig{Field: "timestamp", Order: "desc"},
        Prune:  PruneConfig{OlderThan: "48h", Keep: 0},
        Templates: TemplateConfig{
            Dmenu:     "{{.AppName}} | {{.Summary}} - {{.BodyTruncated 50}} | {{.RelativeTime}}",
            Full:      "{{.Timestamp | formatTime}} {{.AppName}}: {{.Summary}}\n{{.Body}}",
            Body:      "{{.Body}}",
            TUIOutput: "{{.Timestamp | formatTime}} {{.AppName}}: {{.Summary}}\n{{.Body}}",
        },
        TUI: TUIConfig{ShowIcons: true, IconSize: 64, ShowHelp: true},
    }

    path := configPath()
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        return cfg, nil // Use defaults
    }
    if err != nil {
        return nil, err
    }

    if err := toml.Unmarshal(data, cfg); err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }
    return cfg, nil
}
```

### Template Execution

```go
import "text/template"

// Template function map
var funcMap = template.FuncMap{
    "formatTime": func(ts int64) string {
        return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
    },
    "formatTimeRFC3339": func(ts int64) string {
        return time.Unix(ts, 0).Format(time.RFC3339)
    },
    "truncate": func(n int, s string) string {
        if len(s) <= n {
            return s
        }
        return s[:n] + "..."
    },
    "escapeJSON": func(s string) string {
        b, _ := json.Marshal(s)
        return string(b[1 : len(b)-1]) // Remove quotes
    },
    "upper": strings.ToUpper,
    "lower": strings.ToLower,
}

func executeTemplate(tmplStr string, n *Notification) (string, error) {
    tmpl, err := template.New("output").Funcs(funcMap).Parse(tmplStr)
    if err != nil {
        return "", err
    }
    var buf strings.Builder
    if err := tmpl.Execute(&buf, n); err != nil {
        return "", err
    }
    return buf.String(), nil
}
```

### CLI Override Pattern

```go
// Flag values override config
type Flags struct {
    Since          string
    Format         string
    OutputTemplate string
    // ...
}

func mergeConfig(cfg *Config, flags *Flags) {
    if flags.Since != "" {
        cfg.Filter.Since = flags.Since
    }
    if flags.Format != "" {
        // Format could be preset name or inline template
        if tmpl, ok := cfg.Templates.Custom[flags.Format]; ok {
            cfg.Templates.TUIOutput = tmpl
        } else {
            cfg.Templates.TUIOutput = flags.Format
        }
    }
    if flags.OutputTemplate != "" {
        cfg.Templates.TUIOutput = flags.OutputTemplate
    }
}
```

---

## Alternatives Considered

### CLI Framework
- **urfave/cli**: Simpler but less ecosystem support
- **Kong**: Good but less common in Go projects
- **Decision**: Cobra (constitution requirement, industry standard)

### TUI Framework
- **tview**: More traditional widget-based, less composable
- **tcell**: Lower-level, more boilerplate
- **Decision**: BubbleTea (constitution requirement, modern architecture)

### ID Format
- **UUID v4**: No time-ordering, 36 characters
- **UUID v7**: Good but newer, less library support
- **NanoID**: Shorter but no time component
- **Decision**: ULID (constitution requirement, time-sortable)

### Persistence Format
- **SQLite**: Overkill for simple list storage
- **BoltDB**: More complex than needed
- **JSON file**: Single record, hard to append
- **Decision**: JSONL (append-friendly, simple, human-readable)

### Clipboard
- **Go clipboard libraries**: Poor Wayland support
- **Direct wl-copy**: Native Wayland, works with Hyprland
- **Decision**: wl-copy + xclip fallback
