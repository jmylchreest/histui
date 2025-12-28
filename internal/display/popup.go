package display

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	layershell "github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
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

// Popup represents a notification popup window.
type Popup struct {
	window       *gtk.Window
	notification *dbus.DBusNotification
	config       *config.DaemonConfig
	layout       *layout.LayoutConfig
	logger       *slog.Logger
	iconResolver *icon.Resolver

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
	onClose       func(reason dbus.CloseReason)
	onAction      func(actionKey string)
	onHover       func(hovering bool)
	onCloseAll    func()
	onHeightReady func(height int) // Called when actual height is measured
	onWidthReady  func(width int)  // Called when actual width is measured

	// State
	closed       bool
	stackCount   int // Number of stacked identical notifications
	timestamp    time.Time
	actualHeight int // Measured height after GTK layout
	actualWidth  int // Measured width after GTK layout

	// Stack position for unified stack styling
	stackPosition string // "single", "first", "middle", "last"

	// Animation state
	currentOffsetY int // Current animated Y offset
	targetOffsetY  int // Target Y offset for animation
	animating      bool

	// Layout-derived sizing (for position calculations)
	minWidth  int
	maxWidth  int
	maxHeight int
}

// NewPopup creates a new notification popup with its own layer-shell window.
// Use NewPopupWidget for embedding in a container (single-window mode).
func NewPopup(app *gtk.Application, notification *dbus.DBusNotification, cfg *config.DaemonConfig, logger *slog.Logger) (*Popup, error) {
	p := newPopupBase(notification, cfg, logger)

	// Create the window
	p.window = gtk.NewWindow()
	p.window.SetApplication(app)
	p.window.SetDecorated(false)
	p.window.SetResizable(false)

	// Set window size constraints
	p.window.SetDefaultSize(p.maxWidth, -1)
	p.window.SetSizeRequest(p.minWidth, p.layout.MinHeight)

	// Initialize layer-shell
	layershell.InitForWindow(p.window)
	layershell.SetLayer(p.window, layershell.LayerShellLayerTop)
	layershell.SetExclusiveZone(p.window, 0) // Don't reserve space
	layershell.SetKeyboardMode(p.window, layershell.LayerShellKeyboardModeNone)

	// Set namespace for window managers
	layershell.SetNamespace(p.window, "histui-notification")

	// Build the UI from layout template
	p.buildUI()

	// Apply CSS classes for theming
	p.applyThemeClasses()

	// Connect signals (window-level)
	p.connectSignals()

	// Set the box as window child
	p.window.SetChild(p.box)

	return p, nil
}

// NewPopupWidget creates a notification popup widget for embedding in a container.
// Unlike NewPopup, this does not create a window - just the notification content box.
// Use this for single-window mode where all notifications share one layer-shell window.
func NewPopupWidget(notification *dbus.DBusNotification, cfg *config.DaemonConfig, logger *slog.Logger, iconResolver *icon.Resolver) (*Popup, error) {
	p := newPopupBase(notification, cfg, logger)
	p.iconResolver = iconResolver

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
	p.maxHeight = layoutConfig.MaxHeight

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
	if len(p.notification.ParsedActions()) > 0 {
		p.box.AddCSSClass("has-actions")
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
		return p.buildStackCount()
	case layout.ElementTypeImage:
		return p.buildImage()
	case layout.ElementTypeBox:
		return p.buildBox(elem)
	default:
		return nil
	}
}

// buildHeader creates the header row with child elements.
func (p *Popup) buildHeader(elem layout.LayoutElement) gtk.Widgetter {
	headerBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	headerBox.AddCSSClass("notification-header")

	for _, child := range elem.Children {
		if widget := p.buildElement(child); widget != nil {
			headerBox.Append(widget)
		}
	}

	return headerBox
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
// Falls back to Nerd Font symbols when icon theme lookup fails.
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

	// Helper to create a Nerd Font symbol label as fallback
	createNerdLabel := func(symbol string) gtk.Widgetter {
		label := gtk.NewLabel(symbol)
		label.AddCSSClass("notification-icon")
		label.AddCSSClass("notification-icon-nerd")
		return label
	}

	// Helper to get Nerd Font symbol for app/category
	getNerdSymbol := func() string {
		if p.iconResolver != nil {
			// Try app name first
			if symbol := p.iconResolver.GetNerdSymbol(appName); symbol != "" {
				return symbol
			}
			// Try resolved icon name
			if iconName != "" {
				resolved := p.iconResolver.Resolve(iconName)
				if symbol := p.iconResolver.GetNerdSymbol(resolved); symbol != "" {
					return symbol
				}
			}
			// Try notification category
			if symbol := p.iconResolver.GetNerdSymbolForCategory(p.notification.Category()); symbol != "" {
				return symbol
			}
		}
		// Fallback based on urgency
		return icon.FallbackNerdSymbolForUrgency(p.notification.Urgency())
	}

	p.iconImage = gtk.NewImage()
	p.iconImage.AddCSSClass("notification-icon")
	p.iconImage.SetPixelSize(iconSize)

	if iconName == "" {
		// No icon provided - use Nerd Font symbol
		p.logger.Debug("no app icon provided, using nerd font symbol", "app", appName)
		return createNerdLabel(getNerdSymbol())
	}

	p.logger.Debug("loading app icon", "icon", iconName, "size", iconSize)

	// Check if it's a file path (absolute path or file:// URI)
	if strings.HasPrefix(iconName, "/") {
		// Absolute file path - load from file
		pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(iconName, iconSize, iconSize)
		if err != nil {
			p.logger.Debug("failed to load icon from file, using nerd font symbol",
				"path", iconName,
				"error", err,
			)
			return createNerdLabel(getNerdSymbol())
		}
		p.logger.Debug("loaded icon from file", "path", iconName, "width", pixbuf.Width(), "height", pixbuf.Height())
		p.iconImage.SetFromPixbuf(pixbuf) //nolint:staticcheck // TODO: migrate to SetFromPaintable when API stabilizes
		return p.iconImage
	}

	if strings.HasPrefix(iconName, "file://") {
		// File URI - strip prefix and load from file
		path := strings.TrimPrefix(iconName, "file://")
		pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(path, iconSize, iconSize)
		if err != nil {
			p.logger.Debug("failed to load icon from file URI, using nerd font symbol",
				"uri", iconName,
				"error", err,
			)
			return createNerdLabel(getNerdSymbol())
		}
		p.logger.Debug("loaded icon from file URI", "path", path, "width", pixbuf.Width(), "height", pixbuf.Height())
		p.iconImage.SetFromPixbuf(pixbuf) //nolint:staticcheck // TODO: migrate to SetFromPaintable when API stabilizes
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

	// Check if icon exists in theme
	iconTheme := gtk.IconThemeGetForDisplay(p.iconImage.Display())
	if iconTheme != nil && iconTheme.HasIcon(resolvedIcon) {
		p.logger.Debug("using icon from theme", "icon_name", resolvedIcon)
		p.iconImage.SetFromIconName(resolvedIcon)
		return p.iconImage
	}

	// Icon not in theme - use Nerd Font symbol
	p.logger.Debug("icon not found in theme, using nerd font symbol",
		"icon_name", resolvedIcon,
		"app", appName,
	)
	return createNerdLabel(getNerdSymbol())
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
func (p *Popup) buildStackCount() gtk.Widgetter {
	p.stackCountLbl = gtk.NewLabel("")
	p.stackCountLbl.AddCSSClass("notification-stack-count")
	p.stackCountLbl.SetVisible(false)
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
func (p *Popup) buildActions() gtk.Widgetter {
	actions := p.notification.ParsedActions()
	if len(actions) == 0 {
		return nil
	}

	p.actionBox = gtk.NewBox(gtk.OrientationHorizontal, 6)
	p.actionBox.AddCSSClass("notification-actions")
	p.actionBox.SetVisible(false) // Hidden by default, shown on hover

	for _, action := range actions {
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
func (p *Popup) buildImage() gtk.Widgetter {
	var pixbuf *gdkpixbuf.Pixbuf

	// Try image-data first (embedded pixel data)
	if imgData := p.notification.ImageData(); imgData != nil {
		pixbuf = p.createPixbufFromData(imgData)
	} else {
		// Fall back to image-path (file path)
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
		// No cropping needed - use gtk.Image which respects intrinsic size
		texture := gdk.NewTextureForPixbuf(scaledPixbuf)
		if texture == nil {
			p.logger.Warn("failed to create texture from pixbuf")
			return nil
		}
		image := gtk.NewImageFromPaintable(texture)
		image.AddCSSClass("notification-image")
		image.SetPixelSize(newHeight) // Use height as pixel size for proper scaling
		// Prevent expansion when container resizes
		image.SetHExpand(false)
		image.SetVExpand(false)
		image.SetHAlign(gtk.AlignCenter)
		image.SetVAlign(gtk.AlignStart)
		return image
	}

	// Crop from bottom (keep top portion of the image)
	croppedPixbuf := scaledPixbuf.NewSubpixbuf(0, 0, newWidth, imageMaxHeight)
	if croppedPixbuf == nil {
		// Fallback to uncropped - use gtk.Image
		texture := gdk.NewTextureForPixbuf(scaledPixbuf)
		if texture == nil {
			return nil
		}
		image := gtk.NewImageFromPaintable(texture)
		image.AddCSSClass("notification-image")
		image.SetPixelSize(newHeight)
		image.SetHExpand(false)
		image.SetVExpand(false)
		image.SetHAlign(gtk.AlignCenter)
		image.SetVAlign(gtk.AlignStart)
		return image
	}

	// Create the cropped image widget using texture and gtk.Picture
	texture := gdk.NewTextureForPixbuf(croppedPixbuf)
	if texture == nil {
		p.logger.Warn("failed to create texture from cropped pixbuf")
		return nil
	}
	picture := gtk.NewPictureForPaintable(texture)
	picture.AddCSSClass("notification-image")
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

	// Add gradient overlay for fade effect at the bottom (~10px)
	gradientOverlay := gtk.NewBox(gtk.OrientationVertical, 0)
	gradientOverlay.AddCSSClass("notification-image-fade")
	gradientOverlay.SetVAlign(gtk.AlignEnd)
	gradientOverlay.SetHExpand(true)       // Fill width
	gradientOverlay.SetSizeRequest(-1, 10) // Gradient height
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

// connectSignals sets up event handlers.
func (p *Popup) connectSignals() {
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
	p.window.AddController(motionCtrl)

	// Click handler for configurable mouse actions
	clickCtrl := gtk.NewGestureClick()
	clickCtrl.SetButton(0) // All buttons
	clickCtrl.ConnectReleased(func(nPress int, x, y float64) {
		button := clickCtrl.CurrentButton()
		p.handleClick(button)
	})
	p.window.AddController(clickCtrl)
}

// connectWidgetSignals sets up event handlers on the box widget (for embedded mode).
// Similar to connectSignals but attaches to p.box instead of p.window.
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

// Show displays the popup at the given vertical offset.
// The offsetY is the absolute Y position from the screen edge.
func (p *Popup) Show(offsetY int) {
	p.currentOffsetY = offsetY
	p.targetOffsetY = offsetY
	p.updateAnchorPositionWithOffset(offsetY)

	// Connect to map signal to measure actual size after GTK layout
	var signalHandle glib.SignalHandle
	signalHandle = p.window.ConnectMap(func() {
		// Use idle callback to ensure layout is complete
		glib.IdleAdd(func() {
			// Get the actual window size (not preferred size) for accurate positioning
			// The window size is what layer-shell uses for positioning other windows
			height := p.window.Height()
			width := p.window.Width()

			// Clamp to max height
			if height > p.maxHeight {
				height = p.maxHeight
			}

			p.actualHeight = height
			p.actualWidth = width
			p.logger.Debug("measured popup size",
				"height", height,
				"width", width,
				"max_height", p.maxHeight,
			)

			// Notify manager of actual dimensions
			if p.onHeightReady != nil {
				p.onHeightReady(height)
			}
			if p.onWidthReady != nil {
				p.onWidthReady(width)
			}
		})

		// Disconnect after first map
		p.window.HandlerDisconnect(signalHandle)
	})

	p.window.Present()
}

// Close closes the popup.
// For windowed popups, this closes the window.
// For embedded widgets, this just marks as closed (container handles removal).
func (p *Popup) Close() {
	if p.closed {
		return
	}
	p.closed = true
	if p.window != nil {
		p.window.Close()
	}
}

// Widget returns the notification box widget for embedding in a container.
// Returns nil if called on a windowed popup.
func (p *Popup) Widget() *gtk.Box {
	return p.box
}

// IsEmbedded returns true if this popup is a widget (no window).
func (p *Popup) IsEmbedded() bool {
	return p.window == nil
}

// UpdatePosition updates the popup's vertical offset with animation.
func (p *Popup) UpdatePosition(offsetY int) {
	if p.targetOffsetY == offsetY {
		return
	}
	p.animateToOffset(offsetY)
}

// animateToOffset smoothly animates the popup to a new Y offset.
func (p *Popup) animateToOffset(targetY int) {
	if p.closed {
		return
	}

	p.targetOffsetY = targetY

	// If already animating, the existing animation will pick up the new target
	if p.animating {
		return
	}

	p.animating = true

	// Animation parameters
	const (
		animationDuration = 150 * time.Millisecond
		frameInterval     = 16 * time.Millisecond // ~60fps
	)

	startY := p.currentOffsetY
	startTime := time.Now()

	// Animation tick function
	tick := func() bool {
		if p.closed {
			p.animating = false
			return false // Stop animation
		}

		elapsed := time.Since(startTime)
		progress := float64(elapsed) / float64(animationDuration)

		if progress >= 1.0 {
			// Animation complete
			p.currentOffsetY = p.targetOffsetY
			p.updateAnchorPositionWithOffset(p.currentOffsetY)
			p.animating = false

			// Check if target changed during animation
			if p.currentOffsetY != p.targetOffsetY {
				// Start new animation to new target
				p.animateToOffset(p.targetOffsetY)
			}
			return false // Stop this animation
		}

		// Ease-out cubic: 1 - (1 - t)^3
		eased := 1.0 - (1.0-progress)*(1.0-progress)*(1.0-progress)

		// Interpolate position
		p.currentOffsetY = startY + int(float64(p.targetOffsetY-startY)*eased)
		p.updateAnchorPositionWithOffset(p.currentOffsetY)

		return true // Continue animation
	}

	// Start animation loop
	glib.TimeoutAdd(uint(frameInterval.Milliseconds()), tick)
}

// updateAnchorPositionWithOffset sets the layer-shell position with a specific Y offset.
func (p *Popup) updateAnchorPositionWithOffset(offsetY int) {
	pos := config.Position(p.config.Display.Position)
	offsetX := p.config.Display.OffsetX

	// Reset all anchors first
	layershell.SetAnchor(p.window, layershell.LayerShellEdgeTop, false)
	layershell.SetAnchor(p.window, layershell.LayerShellEdgeBottom, false)
	layershell.SetAnchor(p.window, layershell.LayerShellEdgeLeft, false)
	layershell.SetAnchor(p.window, layershell.LayerShellEdgeRight, false)

	switch pos {
	case config.PositionTopRight:
		layershell.SetAnchor(p.window, layershell.LayerShellEdgeTop, true)
		layershell.SetAnchor(p.window, layershell.LayerShellEdgeRight, true)
		layershell.SetMargin(p.window, layershell.LayerShellEdgeTop, offsetY)
		layershell.SetMargin(p.window, layershell.LayerShellEdgeRight, offsetX)

	case config.PositionTopLeft:
		layershell.SetAnchor(p.window, layershell.LayerShellEdgeTop, true)
		layershell.SetAnchor(p.window, layershell.LayerShellEdgeLeft, true)
		layershell.SetMargin(p.window, layershell.LayerShellEdgeTop, offsetY)
		layershell.SetMargin(p.window, layershell.LayerShellEdgeLeft, offsetX)

	case config.PositionTopCenter:
		layershell.SetAnchor(p.window, layershell.LayerShellEdgeTop, true)
		layershell.SetMargin(p.window, layershell.LayerShellEdgeTop, offsetY)

	case config.PositionBottomRight:
		layershell.SetAnchor(p.window, layershell.LayerShellEdgeBottom, true)
		layershell.SetAnchor(p.window, layershell.LayerShellEdgeRight, true)
		layershell.SetMargin(p.window, layershell.LayerShellEdgeBottom, offsetY)
		layershell.SetMargin(p.window, layershell.LayerShellEdgeRight, offsetX)

	case config.PositionBottomLeft:
		layershell.SetAnchor(p.window, layershell.LayerShellEdgeBottom, true)
		layershell.SetAnchor(p.window, layershell.LayerShellEdgeLeft, true)
		layershell.SetMargin(p.window, layershell.LayerShellEdgeBottom, offsetY)
		layershell.SetMargin(p.window, layershell.LayerShellEdgeLeft, offsetX)

	case config.PositionBottomCenter:
		layershell.SetAnchor(p.window, layershell.LayerShellEdgeBottom, true)
		layershell.SetMargin(p.window, layershell.LayerShellEdgeBottom, offsetY)
	}
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

// OnHeightReady sets the callback for when actual height is measured.
func (p *Popup) OnHeightReady(cb func(height int)) {
	p.onHeightReady = cb
}

// OnWidthReady sets the callback for when actual width is measured.
func (p *Popup) OnWidthReady(cb func(width int)) {
	p.onWidthReady = cb
}

// GetActualHeight returns the measured actual height of the popup.
// Returns 0 if height hasn't been measured yet.
func (p *Popup) GetActualHeight() int {
	return p.actualHeight
}

// GetMaxHeight returns the maximum allowed height.
func (p *Popup) GetMaxHeight() int {
	return p.maxHeight
}

// GetActualWidth returns the measured actual width of the popup.
// Returns 0 if width hasn't been measured yet.
func (p *Popup) GetActualWidth() int {
	return p.actualWidth
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
// Thread-safe: defers GTK operations to main thread.
func (p *Popup) SetStackCount(count int) {
	p.stackCount = count
	if p.stackCountLbl == nil {
		return
	}

	// Defer GTK operations to main thread
	glib.IdleAdd(func() {
		if count > 1 {
			p.stackCountLbl.SetText("(" + itoa(count) + ")")
			p.stackCountLbl.SetVisible(true)
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
