package display

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/dbus"
	"github.com/jmylchreest/histui/internal/icon"
	"github.com/jmylchreest/histui/internal/layout"
	"github.com/jmylchreest/histui/internal/model"
)

// Popup represents a notification popup widget.
type Popup struct {
	notification *dbus.DBusNotification
	config       *config.DaemonConfig
	layout       *layout.LayoutConfig
	logger       *slog.Logger
	iconResolver *icon.Resolver

	// Theme-provided icons directory (for custom icon images)
	themeIconsDir string

	// Widgets
	box           *gtk.Box
	summaryLbl    *gtk.Label
	bodyLbl       *gtk.Label
	appNameLbl    *gtk.Label
	timestampLbl  *gtk.Label
	iconImage     *gtk.Image
	actionBox     *gtk.Box
	progressBar   *gtk.ProgressBar
	stackCountLbl *gtk.Label

	// Callbacks
	onClose    func(reason dbus.CloseReason)
	onAction   func(actionKey string)
	onHover    func(hovering bool)
	onCloseAll func()

	// State
	closed           bool
	stackCount       int    // Number of stacked identical notifications
	stackCountFormat string // "number" (default) or "dots"
	timestamp        time.Time

	// Stack position for unified stack styling
	stackPosition string // "single", "first", "middle", "last"

	// Layout-derived sizing (for position calculations)
	minWidth int
	maxWidth int
}

// NewPopupWidget creates a notification popup widget for embedding in a container.
// This creates just the notification content box, not a window.
// All notifications share one layer-shell window managed by the stack.
func NewPopupWidget(notification *dbus.DBusNotification, cfg *config.DaemonConfig, logger *slog.Logger, iconResolver *icon.Resolver, themeIconsDir string) (*Popup, error) {
	p := newPopupBase(notification, cfg, logger)
	p.iconResolver = iconResolver
	p.themeIconsDir = themeIconsDir

	// Build the UI from layout template
	p.buildUI()

	// Apply CSS classes for theming
	p.applyThemeClasses()

	// Connect widget-level signals (hover, click on box instead of window)
	p.connectWidgetSignals()

	// Set size constraints on the box itself
	p.box.SetSizeRequest(p.minWidth, -1)

	return p, nil
}

// newPopupBase creates the base Popup with configuration but no window or UI.
func newPopupBase(notification *dbus.DBusNotification, cfg *config.DaemonConfig, logger *slog.Logger) *Popup {
	if logger == nil {
		logger = slog.Default()
	}

	// Load layout template from theme
	// Layout is bundled with the theme (themes/{name}/layout.xml)
	var layoutConfig *layout.LayoutConfig
	themeName := cfg.Theme.Name
	if themeName == "" {
		themeName = "default"
	}

	if tmpl, found := layout.GetEmbeddedTemplate(themeName); found {
		layoutConfig = tmpl
	} else {
		// Fall back to default layout
		layoutConfig = layout.DefaultLayout()
		logger.Debug("theme has no layout.xml, using default layout", "theme", themeName)
	}

	p := &Popup{
		notification: notification,
		config:       cfg,
		layout:       layoutConfig,
		logger:       logger,
		timestamp:    time.Now(),
	}

	// Use layout sizing (layout package provides sensible defaults)
	p.minWidth = layoutConfig.MinWidth
	p.maxWidth = layoutConfig.MaxWidth

	return p
}

// applyThemeClasses adds CSS classes for advanced theming.
func (p *Popup) applyThemeClasses() {
	// Color scheme class (light/dark)
	p.box.AddCSSClass(p.getColorSchemeClass())

	// Urgency class
	p.box.AddCSSClass(urgencyToClass(p.notification.Urgency()))

	// Per-app class (sanitized app name)
	if p.notification.AppName != "" {
		appClass := "app-" + sanitizeClassName(p.notification.AppName)
		p.box.AddCSSClass(appClass)
	}

	// Category class
	if cat := p.notification.Category(); cat != "" {
		catClass := "category-" + sanitizeClassName(cat)
		p.box.AddCSSClass(catClass)
	}

	// State classes
	if p.notification.Body != "" {
		p.box.AddCSSClass("has-body")
	}
	if p.notification.AppIcon != "" {
		p.box.AddCSSClass("has-icon")
	}

	// Action-related classes
	actions := p.notification.ParsedActions()
	if len(actions) > 0 {
		p.box.AddCSSClass("has-actions")
	}

	// Check for default action (key="default") - indicates notification is clickable
	hasDefaultAction := false
	for _, a := range actions {
		if a.Key == "default" {
			hasDefaultAction = true
			break
		}
	}
	if hasDefaultAction {
		p.box.AddCSSClass("has-default-action")
		p.box.AddCSSClass("is-clickable") // Semantic alias
	}

	// Check for visible actions (non-empty labels, non-default key)
	hasVisibleActions := false
	for _, a := range actions {
		if a.Key != "default" && a.Label != "" {
			hasVisibleActions = true
			break
		}
	}
	if hasVisibleActions {
		p.box.AddCSSClass("has-visible-actions")
	}
	if p.notification.Resident() {
		p.box.AddCSSClass("is-resident")
	}
	if p.notification.Transient() {
		p.box.AddCSSClass("is-transient")
	}

	// Progress class
	if progress := p.notification.Progress(); progress >= 0 {
		p.box.AddCSSClass("has-progress")
		// Add progress range classes for styling
		switch {
		case progress == 100:
			p.box.AddCSSClass("progress-complete")
		case progress >= 75:
			p.box.AddCSSClass("progress-high")
		case progress >= 50:
			p.box.AddCSSClass("progress-medium")
		case progress >= 25:
			p.box.AddCSSClass("progress-low")
		default:
			p.box.AddCSSClass("progress-minimal")
		}
	}
}

// sanitizeClassName converts a string to a valid CSS class name.
// Replaces spaces and special characters with hyphens, lowercases.
func sanitizeClassName(name string) string {
	var result strings.Builder
	prevHyphen := false

	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z':
			result.WriteRune(r)
			prevHyphen = false
		case r >= '0' && r <= '9':
			result.WriteRune(r)
			prevHyphen = false
		case r == '-' || r == '_':
			if !prevHyphen && result.Len() > 0 {
				result.WriteRune('-')
				prevHyphen = true
			}
		case r == ' ' || r == '.' || r == '/':
			if !prevHyphen && result.Len() > 0 {
				result.WriteRune('-')
				prevHyphen = true
			}
		}
	}

	// Trim trailing hyphen
	s := result.String()
	if len(s) > 0 && s[len(s)-1] == '-' {
		s = s[:len(s)-1]
	}
	return s
}

// buildUI constructs the popup widget hierarchy from the layout template.
// Note: Does NOT set window child - caller must do that if using windowed mode.
func (p *Popup) buildUI() {
	// Main container
	// Margins are set via CSS for flexibility with stack positions
	p.box = gtk.NewBox(gtk.OrientationVertical, 6)
	p.box.AddCSSClass("notification-popup")

	// Build from layout template
	for _, elem := range p.layout.Elements {
		if widget := p.buildElement(elem); widget != nil {
			p.box.Append(widget)
		}
	}
}

// buildElement builds a GTK widget from a layout element.
func (p *Popup) buildElement(elem layout.LayoutElement) gtk.Widgetter {
	switch elem.Type {
	case layout.ElementTypeHeader:
		return p.buildHeader(elem)
	case layout.ElementTypeBody:
		return p.buildBody()
	case layout.ElementTypeActions:
		return p.buildActions()
	case layout.ElementTypeProgress:
		return p.buildProgress()
	case layout.ElementTypeIcon:
		return p.buildIcon(elem)
	case layout.ElementTypeSummary:
		return p.buildSummary()
	case layout.ElementTypeAppName:
		return p.buildAppName()
	case layout.ElementTypeTimestamp:
		return p.buildTimestamp()
	case layout.ElementTypeStackCount:
		return p.buildStackCount(elem)
	case layout.ElementTypeImage:
		return p.buildImage()
	case layout.ElementTypeBox:
		return p.buildBox(elem)
	case layout.ElementTypeDefaultActionIndicator:
		return p.buildDefaultActionIndicator(elem)
	default:
		return nil
	}
}

// buildHeader creates the header row with child elements.
// Supports overlay="top-right" etc. for elements that should float over others.
// Supports underlay="top-right" etc. for elements that should float behind others.
func (p *Popup) buildHeader(elem layout.LayoutElement) gtk.Widgetter {
	headerBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	headerBox.AddCSSClass("notification-header")

	// Categorize children by positioning type
	var underlayChildren []layout.LayoutElement
	var normalChildren []layout.LayoutElement
	var overlayChildren []layout.LayoutElement

	for _, child := range elem.Children {
		if _, hasUnderlay := child.Attributes["underlay"]; hasUnderlay {
			underlayChildren = append(underlayChildren, child)
		} else if _, hasOverlay := child.Attributes["overlay"]; hasOverlay {
			overlayChildren = append(overlayChildren, child)
		} else {
			normalChildren = append(normalChildren, child)
		}
	}

	// Add normal children to header box
	for _, child := range normalChildren {
		if widget := p.buildElement(child); widget != nil {
			headerBox.Append(widget)
		}
	}

	// If no floating children, return the simple header box
	if len(underlayChildren) == 0 && len(overlayChildren) == 0 {
		return headerBox
	}

	// Wrap in GtkOverlay for floating elements
	overlay := gtk.NewOverlay()
	overlay.SetChild(headerBox)
	overlay.AddCSSClass("notification-header-overlay")

	// Add underlay children first (they render below overlays)
	for _, child := range underlayChildren {
		if widget := p.buildElement(child); widget != nil {
			pos := child.Attributes["underlay"]
			applyOverlayAlignment(widget, pos)
			overlay.AddOverlay(widget)
		}
	}

	// Add overlay children last (they render on top)
	for _, child := range overlayChildren {
		if widget := p.buildElement(child); widget != nil {
			pos := child.Attributes["overlay"]
			applyOverlayAlignment(widget, pos)
			overlay.AddOverlay(widget)
		}
	}

	return overlay
}

// applyOverlayAlignment sets halign/valign based on position string.
func applyOverlayAlignment(widget gtk.Widgetter, position string) {
	// Get the base widget to set alignment
	baseWidget := gtk.BaseWidget(widget)

	var hAlign, vAlign gtk.Align
	switch position {
	case "top-right":
		hAlign, vAlign = gtk.AlignEnd, gtk.AlignStart
	case "top-left":
		hAlign, vAlign = gtk.AlignStart, gtk.AlignStart
	case "bottom-right":
		hAlign, vAlign = gtk.AlignEnd, gtk.AlignEnd
	case "bottom-left":
		hAlign, vAlign = gtk.AlignStart, gtk.AlignEnd
	default:
		// Default to top-right
		hAlign, vAlign = gtk.AlignEnd, gtk.AlignStart
	}

	baseWidget.SetHAlign(hAlign)
	baseWidget.SetVAlign(vAlign)
}

// buildBox creates a container box with child elements.
func (p *Popup) buildBox(elem layout.LayoutElement) gtk.Widgetter {
	orientation := gtk.OrientationVertical
	if elem.Attributes["orientation"] == "horizontal" {
		orientation = gtk.OrientationHorizontal
	}

	box := gtk.NewBox(orientation, 4)
	if orientation == gtk.OrientationVertical {
		box.SetHExpand(true)
	}

	for _, child := range elem.Children {
		if widget := p.buildElement(child); widget != nil {
			box.Append(widget)
		}
	}

	return box
}

// DefaultIconSize is the default icon size in pixels.
const DefaultIconSize = 48

// buildIcon creates the notification icon.
// Handles both icon names (e.g., "dialog-information") and file paths (e.g., "/usr/lib/kitty/logo/kitty.png").
// The icon size can be configured via the layout element's "size" attribute (e.g., <icon size="32"/>).
// Falls back to symbol font glyphs when icon theme lookup fails.
func (p *Popup) buildIcon(elem layout.LayoutElement) gtk.Widgetter {
	// Parse icon size from layout attributes (default: 48)
	iconSize := DefaultIconSize
	if sizeStr, ok := elem.Attributes["size"]; ok {
		if size, err := strconv.Atoi(sizeStr); err == nil && size > 0 {
			iconSize = size
		}
	}

	iconName := p.notification.AppIcon
	appName := p.notification.AppName

	// Use desktop-entry as fallback when app_name is empty
	// Many Flatpak apps (e.g., Discord) send empty app_name but provide desktop-entry hint
	if appName == "" && p.notification.DesktopEntry() != "" {
		appName = p.notification.DesktopEntry()
	}

	// Helper to create a symbol font label as fallback
	createSymbolLabel := func(symbol string) gtk.Widgetter {
		label := gtk.NewLabel(symbol)
		label.AddCSSClass("notification-icon")
		label.AddCSSClass("notification-icon-symbol")
		return label
	}

	// Helper to get symbol font glyph for app/category
	getSymbol := func() string {
		if p.iconResolver != nil {
			// Try app name first
			if symbol := p.iconResolver.GetSymbolForApp(appName); symbol != "" {
				return symbol
			}
			// Try resolved icon name
			if iconName != "" {
				canonical := p.iconResolver.ResolveApp(iconName)
				if symbol := p.iconResolver.GetSymbol(canonical); symbol != "" {
					return symbol
				}
			}
			// Try notification category
			if symbol := p.iconResolver.GetSymbolForCategory(p.notification.Category()); symbol != "" {
				return symbol
			}
			// Fallback based on urgency
			return p.iconResolver.GetSymbolForUrgency(p.notification.Urgency())
		}
		// No resolver available - use hardcoded fallback
		return icon.FallbackNerdSymbolForUrgency(p.notification.Urgency())
	}

	p.iconImage = gtk.NewImage()
	p.iconImage.AddCSSClass("notification-icon")
	p.iconImage.SetPixelSize(iconSize)

	// Add theme icons directory to GTK icon search path for symbolic icon coloring
	iconTheme := gtk.IconThemeGetForDisplay(p.iconImage.Display())
	if iconTheme != nil && p.themeIconsDir != "" {
		// Add our theme icons dir to GTK's search path so symbolic icons get colored
		iconTheme.AddSearchPath(p.themeIconsDir)
		p.logger.Debug("added theme icons to GTK search path", "path", p.themeIconsDir)
	}

	// Helper to try loading icon from GTK theme (includes our added search path)
	tryLoadIcon := func(name string) bool {
		if name == "" {
			return false
		}

		// Use GTK icon theme - this handles symbolic icon coloring correctly
		// Our theme icons dir was added to the search path above
		if iconTheme != nil && iconTheme.HasIcon(name) {
			p.logger.Debug("loaded icon from GTK icon theme",
				"icon_name", name,
			)
			p.iconImage.SetFromIconName(name)
			return true
		}

		// Fallback: try loading directly from file (for non-symbolic icons)
		if themeIconPath := p.findThemeIcon(name, iconSize); themeIconPath != "" {
			pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(themeIconPath, iconSize, iconSize)
			if err == nil {
				p.logger.Debug("loaded icon from file",
					"icon_name", name,
					"path", themeIconPath,
				)
				texture := gdk.NewTextureForPixbuf(pixbuf)
				if texture != nil {
					p.iconImage.SetFromPaintable(texture)
				}
				return true
			}
			p.logger.Debug("failed to load theme icon",
				"path", themeIconPath,
				"error", err,
			)
		}

		return false
	}

	if iconName == "" {
		// No app icon provided - try urgency-based icon from theme aliases
		if p.iconResolver != nil {
			urgencyIconName := p.iconResolver.GetUrgencyIconName(p.notification.Urgency())
			if urgencyIconName != "" {
				p.logger.Debug("trying urgency icon from theme aliases",
					"urgency", p.notification.Urgency(),
					"icon_name", urgencyIconName,
				)
				if tryLoadIcon(urgencyIconName) {
					return p.iconImage
				}
			}
		}

		// Fallback to symbol font glyph
		p.logger.Debug("no app icon provided and urgency icon not found, using symbol font glyph", "app", appName)
		return createSymbolLabel(getSymbol())
	}

	p.logger.Debug("loading app icon", "icon", iconName, "size", iconSize)

	// Check if it's a file path (absolute path or file:// URI)
	if strings.HasPrefix(iconName, "/") {
		// Absolute file path - load from file
		pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(iconName, iconSize, iconSize)
		if err != nil {
			p.logger.Debug("failed to load icon from file, using symbol font glyph",
				"path", iconName,
				"error", err,
			)
			return createSymbolLabel(getSymbol())
		}
		p.logger.Debug("loaded icon from file", "path", iconName, "width", pixbuf.Width(), "height", pixbuf.Height())
		texture := gdk.NewTextureForPixbuf(pixbuf)
		if texture != nil {
			p.iconImage.SetFromPaintable(texture)
		}
		return p.iconImage
	}

	if strings.HasPrefix(iconName, "file://") {
		// File URI - strip prefix and load from file
		path := strings.TrimPrefix(iconName, "file://")
		pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(path, iconSize, iconSize)
		if err != nil {
			p.logger.Debug("failed to load icon from file URI, using symbol font glyph",
				"uri", iconName,
				"error", err,
			)
			return createSymbolLabel(getSymbol())
		}
		p.logger.Debug("loaded icon from file URI", "path", path, "width", pixbuf.Width(), "height", pixbuf.Height())
		texture := gdk.NewTextureForPixbuf(pixbuf)
		if texture != nil {
			p.iconImage.SetFromPaintable(texture)
		}
		return p.iconImage
	}

	// Icon name - resolve aliases and check theme
	resolvedIcon := iconName
	if p.iconResolver != nil {
		resolvedIcon = p.iconResolver.Resolve(iconName)
		if resolvedIcon != iconName {
			p.logger.Debug("resolved icon alias", "original", iconName, "resolved", resolvedIcon)
		}
	}

	// Try theme icons folder and GTK icon theme
	if tryLoadIcon(resolvedIcon) {
		return p.iconImage
	}

	// Icon not found anywhere - try urgency fallback icon from theme aliases
	if p.iconResolver != nil {
		urgencyIconName := p.iconResolver.GetUrgencyIconName(p.notification.Urgency())
		if urgencyIconName != "" && urgencyIconName != resolvedIcon {
			p.logger.Debug("app icon not found, trying urgency fallback",
				"original", resolvedIcon,
				"urgency_icon", urgencyIconName,
			)
			if tryLoadIcon(urgencyIconName) {
				return p.iconImage
			}
		}
	}

	// Fallback to symbol font glyph
	p.logger.Debug("icon not found in any source, using symbol font glyph",
		"icon_name", resolvedIcon,
		"app", appName,
		"theme_icons_dir", p.themeIconsDir,
	)
	return createSymbolLabel(getSymbol())
}

// findThemeIcon looks for an icon file in the theme icons directory.
// Returns the full path to the icon file if found, empty string otherwise.
// Checks for common image formats: .png, .svg, .xpm, .ico
// Also checks for scalable/ and {size}x{size}/ subdirectories.
func (p *Popup) findThemeIcon(iconName string, size int) string {
	if p.themeIconsDir == "" || iconName == "" {
		return ""
	}

	extensions := []string{".png", ".svg", ".xpm", ".ico"}

	// Check direct icon files in icons/
	for _, ext := range extensions {
		path := filepath.Join(p.themeIconsDir, iconName+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Check scalable/ subdirectory (SVG preferred)
	scalableDir := filepath.Join(p.themeIconsDir, "scalable")
	for _, ext := range extensions {
		path := filepath.Join(scalableDir, iconName+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Check size-specific subdirectory (e.g., 48x48/)
	sizeDir := filepath.Join(p.themeIconsDir, itoa(size)+"x"+itoa(size))
	for _, ext := range extensions {
		path := filepath.Join(sizeDir, iconName+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// buildSummary creates the summary label.
func (p *Popup) buildSummary() gtk.Widgetter {
	p.summaryLbl = gtk.NewLabel(p.notification.Summary)
	p.summaryLbl.AddCSSClass("notification-summary")
	p.summaryLbl.SetXAlign(0)
	p.summaryLbl.SetEllipsize(3) // PANGO_ELLIPSIZE_END
	p.summaryLbl.SetMaxWidthChars(40)
	p.summaryLbl.SetHExpand(true)
	return p.summaryLbl
}

// buildAppName creates the app name label.
func (p *Popup) buildAppName() gtk.Widgetter {
	p.appNameLbl = gtk.NewLabel(p.notification.AppName)
	p.appNameLbl.AddCSSClass("notification-appname")
	p.appNameLbl.SetXAlign(0)
	return p.appNameLbl
}

// buildTimestamp creates the timestamp label.
func (p *Popup) buildTimestamp() gtk.Widgetter {
	p.timestampLbl = gtk.NewLabel(formatRelativeTime(p.timestamp))
	p.timestampLbl.AddCSSClass("notification-timestamp")
	p.timestampLbl.SetXAlign(1)
	return p.timestampLbl
}

// buildStackCount creates the stack count label.
func (p *Popup) buildStackCount(elem layout.LayoutElement) gtk.Widgetter {
	p.stackCountLbl = gtk.NewLabel("")
	p.stackCountLbl.AddCSSClass("notification-stack-count")
	p.stackCountLbl.SetVisible(false)
	// Align to top of header, don't expand to fill height
	p.stackCountLbl.SetVAlign(gtk.AlignStart)
	// Right-align text within label for overlay positioning
	p.stackCountLbl.SetXAlign(1)

	// Check for format attribute (e.g., format="dots")
	if format, ok := elem.Attributes["format"]; ok {
		p.stackCountFormat = format
	}

	return p.stackCountLbl
}

// buildBody creates the body text label.
func (p *Popup) buildBody() gtk.Widgetter {
	if p.notification.Body == "" {
		return nil
	}

	p.bodyLbl = gtk.NewLabel("")
	p.bodyLbl.AddCSSClass("notification-body")
	p.bodyLbl.SetXAlign(0)
	p.bodyLbl.SetWrap(true)
	p.bodyLbl.SetWrapMode(2) // PANGO_WRAP_WORD_CHAR
	p.bodyLbl.SetMaxWidthChars(50)

	// Apply markup if body contains markup tags
	if strings.Contains(p.notification.Body, "<") {
		p.bodyLbl.SetMarkup(sanitizeMarkup(p.notification.Body))
	} else {
		p.bodyLbl.SetText(p.notification.Body)
	}

	return p.bodyLbl
}

// buildActions creates the action buttons container.
// Only shows actions with non-empty labels that are not the "default" action.
// The "default" action is triggered by clicking the notification body (via do-action mouse binding),
// not by a visible button. Actions with empty labels are also filtered out.
func (p *Popup) buildActions() gtk.Widgetter {
	actions := p.notification.ParsedActions()
	if len(actions) == 0 {
		return nil
	}

	// Filter to only visible actions (non-empty label, not "default" key)
	visibleActions := make([]dbus.Action, 0, len(actions))
	for _, a := range actions {
		// Skip "default" action - it's triggered by clicking notification body
		if a.Key == "default" {
			continue
		}
		// Skip empty labels - nothing to display
		if a.Label == "" {
			continue
		}
		visibleActions = append(visibleActions, a)
	}

	// No visible actions after filtering
	if len(visibleActions) == 0 {
		return nil
	}

	p.actionBox = gtk.NewBox(gtk.OrientationHorizontal, 6)
	p.actionBox.AddCSSClass("notification-actions")
	p.actionBox.SetVisible(false) // Hidden by default, shown on hover

	for _, action := range visibleActions {
		actionKey := action.Key // Capture for closure
		btn := gtk.NewButtonWithLabel(action.Label)
		btn.AddCSSClass("notification-action")
		btn.ConnectClicked(func() {
			if p.onAction != nil {
				p.onAction(actionKey)
			}
			// Close after action unless resident
			if !p.notification.Resident() {
				p.Close()
				if p.onClose != nil {
					p.onClose(dbus.CloseReasonDismissed)
				}
			}
		})
		p.actionBox.Append(btn)
	}

	return p.actionBox
}

// buildDefaultActionIndicator is deprecated - the default action indicator is now
// handled via CSS using the .has-default-action class on the popup container.
// The right-hand-indicator effect from effects.css provides the visual feedback.
// This function returns nil; the layout element is kept for backwards compatibility.
func (p *Popup) buildDefaultActionIndicator(_ layout.LayoutElement) gtk.Widgetter {
	// Indicator is now CSS-only via .has-default-action class
	// See: effects.css (right-hand-indicator) and theme.css
	return nil
}

// buildProgress creates the progress bar.
func (p *Popup) buildProgress() gtk.Widgetter {
	progress := p.notification.Progress()
	if progress < 0 {
		return nil
	}

	p.progressBar = gtk.NewProgressBar()
	p.progressBar.AddCSSClass("notification-progress")
	p.progressBar.SetFraction(float64(progress) / 100.0)
	return p.progressBar
}

// Image display constants
const (
	imageMaxHeight = 150 // Maximum height before cropping
	imagePadding   = 24  // Horizontal padding (12px each side from CSS)
)

// buildImage creates the embedded image widget.
// Handles both image-path (file path) and image-data (raw pixel data) hints.
// Images are scaled to fit the popup width and cropped if too tall.
//
// Image-data display is controlled by display.image_data_preview_size config:
//   - "never" or -1: Never show image-data in body
//   - "always" or 0: Always show image-data in body
//   - "100KB" etc: Only show if raw data size >= threshold (filters small profile pics)
func (p *Popup) buildImage() gtk.Widgetter {
	var pixbuf *gdkpixbuf.Pixbuf

	// Try image-data if size threshold is met
	if imgData := p.notification.ImageData(); imgData != nil {
		dataSize := int64(len(imgData.Data))
		threshold := p.config.Display.ImageDataPreviewSize

		if threshold.ShouldShow(dataSize) {
			p.logger.Debug("image-data meets size threshold",
				"data_size", dataSize,
				"threshold", threshold.Bytes(),
			)
			pixbuf = p.createPixbufFromData(imgData)
		} else {
			p.logger.Debug("image-data below size threshold, skipping",
				"data_size", dataSize,
				"threshold", threshold.Bytes(),
			)
		}
	}

	// Fall back to image-path (file path) - always shown since explicit paths are intentional
	if pixbuf == nil {
		imagePath := p.notification.ImagePath()
		if imagePath == "" {
			return nil
		}
		pixbuf = p.createPixbufFromFile(imagePath)
	}

	if pixbuf == nil {
		return nil
	}

	return p.buildImageContainer(pixbuf)
}

// createPixbufFromData creates a pixbuf from raw D-Bus image data.
func (p *Popup) createPixbufFromData(imgData *dbus.ImageDataStruct) *gdkpixbuf.Pixbuf {
	if imgData == nil || len(imgData.Data) == 0 {
		return nil
	}

	// Use NewBytes to copy the data, ensuring it remains valid
	bytes := glib.NewBytes(imgData.Data)
	pixbuf := gdkpixbuf.NewPixbufFromBytes(
		bytes,
		gdkpixbuf.ColorspaceRGB,
		imgData.HasAlpha,
		int(imgData.BitsPerSample),
		int(imgData.Width),
		int(imgData.Height),
		int(imgData.Rowstride),
	)

	if pixbuf == nil {
		p.logger.Warn("failed to create pixbuf from image data",
			"width", imgData.Width,
			"height", imgData.Height,
		)
	}

	return pixbuf
}

// createPixbufFromFile creates a pixbuf from a file path.
func (p *Popup) createPixbufFromFile(path string) *gdkpixbuf.Pixbuf {
	pixbuf, err := gdkpixbuf.NewPixbufFromFile(path)
	if err != nil {
		p.logger.Warn("failed to load image from file",
			"path", path,
			"error", err,
		)
		return nil
	}
	return pixbuf
}

// buildImageContainer creates the image widget with proper sizing and cropping.
// - Scales image to fit popup width
// - Crops from bottom if too tall, showing top portion
// - Adds fade gradient at bottom edge (~10px) blending into background
func (p *Popup) buildImageContainer(pixbuf *gdkpixbuf.Pixbuf) gtk.Widgetter {
	if pixbuf == nil {
		return nil
	}

	// Calculate available width (popup width minus padding)
	availableWidth := p.maxWidth - imagePadding
	if availableWidth < 100 {
		availableWidth = 100
	}

	// Get original dimensions
	origWidth := pixbuf.Width()
	origHeight := pixbuf.Height()

	// Only scale DOWN if wider than available width - never scale up
	var scaledPixbuf *gdkpixbuf.Pixbuf
	var newWidth, newHeight int

	if origWidth > availableWidth {
		// Scale down to fit width
		scale := float64(availableWidth) / float64(origWidth)
		newWidth = availableWidth
		newHeight = int(float64(origHeight) * scale)
		scaledPixbuf = pixbuf.ScaleSimple(newWidth, newHeight, gdkpixbuf.InterpBilinear)
		if scaledPixbuf == nil {
			return nil
		}
	} else {
		// Keep original size
		newWidth = origWidth
		newHeight = origHeight
		scaledPixbuf = pixbuf
	}

	// Determine if cropping is needed
	isCropped := newHeight > imageMaxHeight

	p.logger.Debug("created notification image",
		"orig_size", strconv.Itoa(origWidth)+"x"+strconv.Itoa(origHeight),
		"scaled_size", strconv.Itoa(newWidth)+"x"+strconv.Itoa(newHeight),
		"cropped", isCropped,
	)

	if !isCropped {
		// No cropping needed - use gtk.Picture for proper paintable sizing
		texture := gdk.NewTextureForPixbuf(scaledPixbuf)
		if texture == nil {
			p.logger.Warn("failed to create texture from pixbuf")
			return nil
		}
		picture := gtk.NewPictureForPaintable(texture)
		picture.AddCSSClass("notification-image")
		// Preserve aspect ratio, only scale down never up
		picture.SetContentFit(gtk.ContentFitScaleDown)
		picture.SetCanShrink(false)
		// Set explicit size to prevent container from adding extra space
		picture.SetSizeRequest(newWidth, newHeight)
		// Prevent expansion when container resizes
		picture.SetHExpand(false)
		picture.SetVExpand(false)
		picture.SetHAlign(gtk.AlignCenter)
		picture.SetVAlign(gtk.AlignStart)
		return picture
	}

	// Crop from bottom (keep top portion of the image)
	croppedPixbuf := scaledPixbuf.NewSubpixbuf(0, 0, newWidth, imageMaxHeight)
	if croppedPixbuf == nil {
		// Fallback to uncropped - use gtk.Picture with explicit sizing
		texture := gdk.NewTextureForPixbuf(scaledPixbuf)
		if texture == nil {
			return nil
		}
		picture := gtk.NewPictureForPaintable(texture)
		picture.AddCSSClass("notification-image")
		picture.SetContentFit(gtk.ContentFitScaleDown)
		picture.SetCanShrink(false)
		picture.SetSizeRequest(newWidth, newHeight)
		picture.SetHExpand(false)
		picture.SetVExpand(false)
		picture.SetHAlign(gtk.AlignCenter)
		picture.SetVAlign(gtk.AlignStart)
		return picture
	}

	// Create the cropped image widget using texture and gtk.Picture
	texture := gdk.NewTextureForPixbuf(croppedPixbuf)
	if texture == nil {
		p.logger.Warn("failed to create texture from cropped pixbuf")
		return nil
	}
	picture := gtk.NewPictureForPaintable(texture)
	picture.AddCSSClass("notification-image")
	picture.SetContentFit(gtk.ContentFitScaleDown)
	picture.SetCanShrink(false)
	picture.SetSizeRequest(newWidth, imageMaxHeight)
	// Prevent expansion when container resizes
	picture.SetHExpand(false)
	picture.SetVExpand(false)
	picture.SetHAlign(gtk.AlignCenter)
	picture.SetVAlign(gtk.AlignStart)

	// Create an overlay for the fade gradient
	overlay := gtk.NewOverlay()
	overlay.AddCSSClass("notification-image-container")
	overlay.AddCSSClass("cropped")
	overlay.SetChild(picture)
	// Constrain overlay to match picture size - prevents extra space below
	overlay.SetSizeRequest(newWidth, imageMaxHeight)
	overlay.SetHExpand(false)
	overlay.SetVExpand(false)
	overlay.SetHAlign(gtk.AlignCenter)
	overlay.SetVAlign(gtk.AlignStart)

	// Add gradient overlay for fade effect at the bottom
	gradientOverlay := gtk.NewBox(gtk.OrientationVertical, 0)
	gradientOverlay.AddCSSClass("notification-image-fade")
	gradientOverlay.SetVAlign(gtk.AlignEnd)
	gradientOverlay.SetHExpand(true)       // Fill width
	gradientOverlay.SetSizeRequest(-1, 24) // Gradient height - visible fade indicator
	overlay.AddOverlay(gradientOverlay)

	return overlay
}

// formatRelativeTime formats a timestamp as a relative time string.
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return itoa(int(d.Minutes())) + "m"
	case d < 24*time.Hour:
		return itoa(int(d.Hours())) + "h"
	default:
		return itoa(int(d.Hours()/24)) + "d"
	}
}

// connectWidgetSignals sets up event handlers on the box widget.
func (p *Popup) connectWidgetSignals() {
	// Mouse enter/leave for hover effects
	motionCtrl := gtk.NewEventControllerMotion()
	motionCtrl.ConnectEnter(func(x, y float64) {
		if p.actionBox != nil {
			p.actionBox.SetVisible(true)
		}
		if p.onHover != nil {
			p.onHover(true)
		}
	})
	motionCtrl.ConnectLeave(func() {
		if p.actionBox != nil {
			p.actionBox.SetVisible(false)
		}
		if p.onHover != nil {
			p.onHover(false)
		}
	})
	p.box.AddController(motionCtrl)

	// Click handler for configurable mouse actions
	clickCtrl := gtk.NewGestureClick()
	clickCtrl.SetButton(0) // All buttons
	clickCtrl.ConnectReleased(func(nPress int, x, y float64) {
		button := clickCtrl.CurrentButton()
		p.handleClick(button)
	})
	p.box.AddController(clickCtrl)
}

// handleClick processes mouse button clicks.
func (p *Popup) handleClick(button uint) {
	var action string
	switch button {
	case 1: // Left
		action = p.config.Mouse.Left
	case 2: // Middle
		action = p.config.Mouse.Middle
	case 3: // Right
		action = p.config.Mouse.Right
	default:
		return
	}

	switch config.MouseAction(action) {
	case config.MouseActionDismiss:
		p.Close()
		if p.onClose != nil {
			p.onClose(dbus.CloseReasonDismissed)
		}
	case config.MouseActionDoAction:
		// Invoke default action if available
		actions := p.notification.ParsedActions()
		if len(actions) > 0 {
			// "default" action is special, otherwise use first action
			actionKey := actions[0].Key
			for _, a := range actions {
				if a.Key == "default" {
					actionKey = "default"
					break
				}
			}
			if p.onAction != nil {
				p.onAction(actionKey)
			}
			if !p.notification.Resident() {
				p.Close()
				if p.onClose != nil {
					p.onClose(dbus.CloseReasonDismissed)
				}
			}
		}
	case config.MouseActionCloseAll:
		// Trigger close-all via the manager callback
		if p.onCloseAll != nil {
			p.onCloseAll()
		} else {
			// Fallback: just close this popup
			p.Close()
			if p.onClose != nil {
				p.onClose(dbus.CloseReasonDismissed)
			}
		}
	case config.MouseActionNone:
		// Do nothing
	}
}

// Close marks the popup as closed (container handles removal).
func (p *Popup) Close() {
	if p.closed {
		return
	}
	p.closed = true
}

// Widget returns the notification box widget for embedding in a container.
func (p *Popup) Widget() *gtk.Box {
	return p.box
}

// OnClose sets the callback for when the popup is closed.
func (p *Popup) OnClose(cb func(reason dbus.CloseReason)) {
	p.onClose = cb
}

// OnAction sets the callback for when an action is invoked.
func (p *Popup) OnAction(cb func(actionKey string)) {
	p.onAction = cb
}

// OnHover sets the callback for hover state changes.
func (p *Popup) OnHover(cb func(hovering bool)) {
	p.onHover = cb
}

// OnCloseAll sets the callback for close-all action.
func (p *Popup) OnCloseAll(cb func()) {
	p.onCloseAll = cb
}

// SetThemeIconsDir sets the directory for theme-provided icons.
// Icons in this directory are checked before the system icon theme.
func (p *Popup) SetThemeIconsDir(dir string) {
	p.themeIconsDir = dir
}

// SetStackPosition updates the popup's position in the notification stack.
// Valid values: "single", "first", "middle", "last"
// This updates CSS classes for unified stack styling.
// Thread-safe: defers GTK operations to main thread.
func (p *Popup) SetStackPosition(position string) {
	if p.stackPosition == position {
		return
	}

	oldPosition := p.stackPosition
	p.stackPosition = position

	// Defer GTK operations to main thread
	glib.IdleAdd(func() {
		// Remove old stack position class
		if oldPosition != "" {
			p.box.RemoveCSSClass("stack-" + oldPosition)
		}

		// Add new stack position class
		if position != "" {
			p.box.AddCSSClass("stack-" + position)
		}
	})

	if position != "" {
		p.logger.Debug("set stack position CSS class",
			"position", position,
			"class", "stack-"+position,
		)
	}
}

// GetStackPosition returns the current stack position.
func (p *Popup) GetStackPosition() string {
	return p.stackPosition
}

// SetWidth sets the popup content box minimum width (for unified stack width).
// This ensures all popups in a stack share the same width.
func (p *Popup) SetWidth(width int) {
	if width > 0 && width <= p.maxWidth {
		p.box.SetSizeRequest(width, -1)
	}
}

// SetStackCount updates the stack count badge.
// A count of 1 or less hides the badge.
// When count increases, triggers a brief flash animation.
// Thread-safe: defers GTK operations to main thread.
func (p *Popup) SetStackCount(count int) {
	oldCount := p.stackCount
	p.stackCount = count
	if p.stackCountLbl == nil {
		return
	}

	// Defer GTK operations to main thread
	glib.IdleAdd(func() {
		if count > 1 {
			// Format the count based on stackCountFormat
			var text string
			if p.stackCountFormat == "dots" {
				// Show dots: one bullet per stacked item (excluding the first)
				text = strings.Repeat("•", count-1)
			} else {
				// Default: show the number
				text = itoa(count)
			}
			p.stackCountLbl.SetText(text)
			p.stackCountLbl.SetVisible(true)

			// Trigger flash animation when count increases
			if count > oldCount {
				p.stackCountLbl.AddCSSClass("stack-count-flash")
				// Remove animation class after it completes (600ms)
				glib.TimeoutAdd(650, func() bool {
					p.stackCountLbl.RemoveCSSClass("stack-count-flash")
					return false // Don't repeat
				})
			}
		} else {
			p.stackCountLbl.SetVisible(false)
		}
	})
}

// GetStackCount returns the current stack count.
func (p *Popup) GetStackCount() int {
	return p.stackCount
}

// IncrementStackCount increases the stack count by 1 and updates the display.
func (p *Popup) IncrementStackCount() {
	p.SetStackCount(p.stackCount + 1)
}

// UpdateContent updates the popup's visible content from a new notification.
// Used for stack-tag replacement where the same popup is reused with new content.
// Thread-safe: defers GTK operations to main thread.
func (p *Popup) UpdateContent(notification *dbus.DBusNotification) {
	p.notification = notification

	glib.IdleAdd(func() {
		// Update summary
		if p.summaryLbl != nil {
			p.summaryLbl.SetText(notification.Summary)
		}

		// Update body
		if p.bodyLbl != nil {
			if strings.Contains(notification.Body, "<") {
				p.bodyLbl.SetMarkup(sanitizeMarkup(notification.Body))
			} else {
				p.bodyLbl.SetText(notification.Body)
			}
		}

		// Update progress bar
		if p.progressBar != nil {
			progress := notification.Progress()
			if progress >= 0 {
				p.progressBar.SetFraction(float64(progress) / 100.0)
				p.progressBar.SetVisible(true)

				// Add/remove progress-complete class
				if progress >= 100 {
					p.box.AddCSSClass("progress-complete")
				} else {
					p.box.RemoveCSSClass("progress-complete")
				}
			}
		}

		// Update app name if visible
		if p.appNameLbl != nil {
			p.appNameLbl.SetText(notification.AppName)
		}

		// Reset timestamp to now
		p.timestamp = time.Now()
		if p.timestampLbl != nil {
			p.timestampLbl.SetText("now")
		}
	})
}

// itoa is a simple int to string conversion.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// urgencyToClass converts urgency level to CSS class name.
func urgencyToClass(urgency int) string {
	switch urgency {
	case model.UrgencyLow:
		return "urgency-low"
	case model.UrgencyCritical:
		return "urgency-critical"
	default:
		return "urgency-normal"
	}
}

// getColorSchemeClass returns "light" or "dark" based on config or system preference.
func (p *Popup) getColorSchemeClass() string {
	scheme := config.ColorScheme(p.config.Theme.ColorScheme)

	switch scheme {
	case config.ColorSchemeLight:
		return "light"
	case config.ColorSchemeDark:
		return "dark"
	default:
		// System detection using libadwaita StyleManager
		return detectSystemColorScheme()
	}
}

// detectSystemColorScheme checks libadwaita for system dark mode preference.
func detectSystemColorScheme() string {
	styleManager := adw.StyleManagerGetDefault()
	if styleManager.Dark() {
		return "dark"
	}
	return "light"
}

// sanitizeMarkup removes unsupported Pango markup tags.
// GTK4 labels support a subset of Pango markup.
func sanitizeMarkup(markup string) string {
	// For now, pass through as-is
	// TODO: Add proper sanitization if needed
	return markup
}

// Ensure adw is used (for libadwaita initialization)
var _ = adw.MAJOR_VERSION
