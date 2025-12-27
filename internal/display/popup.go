package display

import (
	"log/slog"
	"strings"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	layershell "github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/dbus"
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

	// Widgets
	box           *gtk.Box
	summaryLbl    *gtk.Label
	bodyLbl       *gtk.Label
	appNameLbl    *gtk.Label
	timestampLbl  *gtk.Label
	iconImage     *gtk.Image
	actionBox     *gtk.Box
	progressBar   *gtk.ProgressBar
	closeBtn      *gtk.Button
	stackCountLbl *gtk.Label
	imageWidget   *gtk.Image

	// Callbacks
	onClose    func(reason dbus.CloseReason)
	onAction   func(actionKey string)
	onHover    func(hovering bool)
	onCloseAll func()

	// State
	position   int
	closed     bool
	stackCount int // Number of stacked identical notifications
	timestamp  time.Time

	// Layout-derived sizing (for position calculations)
	maxWidth  int
	maxHeight int
}

// NewPopup creates a new notification popup.
func NewPopup(app *gtk.Application, notification *dbus.DBusNotification, cfg *config.DaemonConfig, logger *slog.Logger) (*Popup, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Load layout template
	var layoutConfig *layout.LayoutConfig
	templateName := cfg.Layout.Template
	if templateName == "" {
		templateName = "default"
	}

	if tmpl, found := layout.GetEmbeddedTemplate(templateName); found {
		layoutConfig = tmpl
	} else {
		// Fall back to default layout
		layoutConfig = layout.DefaultLayout()
		logger.Warn("layout template not found, using default", "template", templateName)
	}

	p := &Popup{
		notification: notification,
		config:       cfg,
		layout:       layoutConfig,
		logger:       logger,
		timestamp:    time.Now(),
	}

	// Create the window
	p.window = gtk.NewWindow()
	p.window.SetApplication(app)
	p.window.SetDecorated(false)
	p.window.SetResizable(false)

	// Use layout sizing if specified, otherwise fall back to config
	minWidth := layoutConfig.MinWidth
	if minWidth == 0 {
		minWidth = cfg.Display.Width
	}
	p.maxWidth = layoutConfig.MaxWidth
	if p.maxWidth == 0 {
		p.maxWidth = cfg.Display.Width
	}
	p.maxHeight = layoutConfig.MaxHeight
	if p.maxHeight == 0 {
		p.maxHeight = cfg.Display.MaxHeight
	}

	// Set window size constraints
	p.window.SetDefaultSize(p.maxWidth, -1)
	p.window.SetSizeRequest(minWidth, layoutConfig.MinHeight)

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

	// Connect signals
	p.connectSignals()

	return p, nil
}

// applyThemeClasses adds CSS classes for advanced theming.
func (p *Popup) applyThemeClasses() {
	// Color scheme class (light/dark)
	p.box.AddCSSClass(p.getColorSchemeClass())

	// Urgency class
	p.box.AddCSSClass(urgencyToClass(p.notification.Urgency()))

	// Opacity class for compositor blur effects
	if p.config.Display.Opacity < 1.0 {
		p.box.AddCSSClass("translucent")
	}

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
func (p *Popup) buildUI() {
	// Main container
	p.box = gtk.NewBox(gtk.OrientationVertical, 6)
	p.box.AddCSSClass("notification-popup")
	p.box.SetMarginTop(8)
	p.box.SetMarginBottom(8)
	p.box.SetMarginStart(12)
	p.box.SetMarginEnd(12)

	// Build from layout template
	for _, elem := range p.layout.Elements {
		if widget := p.buildElement(elem); widget != nil {
			p.box.Append(widget)
		}
	}

	p.window.SetChild(p.box)
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
		return p.buildIcon()
	case layout.ElementTypeSummary:
		return p.buildSummary()
	case layout.ElementTypeAppName:
		return p.buildAppName()
	case layout.ElementTypeTimestamp:
		return p.buildTimestamp()
	case layout.ElementTypeStackCount:
		return p.buildStackCount()
	case layout.ElementTypeClose:
		return p.buildClose()
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

// buildIcon creates the notification icon.
func (p *Popup) buildIcon() gtk.Widgetter {
	p.iconImage = gtk.NewImage()
	p.iconImage.AddCSSClass("notification-icon")
	p.iconImage.SetPixelSize(48)
	if p.notification.AppIcon != "" {
		p.iconImage.SetFromIconName(p.notification.AppIcon)
	} else {
		p.iconImage.SetFromIconName("dialog-information")
	}
	return p.iconImage
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

// buildClose creates the close button.
func (p *Popup) buildClose() gtk.Widgetter {
	p.closeBtn = gtk.NewButtonFromIconName("window-close-symbolic")
	p.closeBtn.AddCSSClass("notification-close")
	p.closeBtn.SetVisible(false) // Hidden by default, shown on hover
	return p.closeBtn
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

// buildImage creates the embedded image widget.
func (p *Popup) buildImage() gtk.Widgetter {
	imagePath := p.notification.ImagePath()
	if imagePath == "" {
		return nil
	}

	p.imageWidget = gtk.NewImage()
	p.imageWidget.AddCSSClass("notification-image")
	p.imageWidget.SetFromFile(imagePath)
	return p.imageWidget
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
	// Close button click (if present in layout)
	if p.closeBtn != nil {
		p.closeBtn.ConnectClicked(func() {
			p.Close()
			if p.onClose != nil {
				p.onClose(dbus.CloseReasonDismissed)
			}
		})
	}

	// Mouse enter/leave for hover effects
	motionCtrl := gtk.NewEventControllerMotion()
	motionCtrl.ConnectEnter(func(x, y float64) {
		if p.closeBtn != nil {
			p.closeBtn.SetVisible(true)
		}
		if p.actionBox != nil {
			p.actionBox.SetVisible(true)
		}
		if p.onHover != nil {
			p.onHover(true)
		}
	})
	motionCtrl.ConnectLeave(func() {
		if p.closeBtn != nil {
			p.closeBtn.SetVisible(false)
		}
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

// Show displays the popup at the given stack position.
func (p *Popup) Show(position int) {
	p.position = position
	p.updateAnchorPosition()
	p.window.Present()
}

// Close closes the popup.
func (p *Popup) Close() {
	if p.closed {
		return
	}
	p.closed = true
	p.window.Close()
}

// UpdatePosition updates the popup's position in the stack.
func (p *Popup) UpdatePosition(position int) {
	if p.position == position {
		return
	}
	p.position = position
	p.updateAnchorPosition()
}

// updateAnchorPosition sets the layer-shell anchors and margins based on config.
func (p *Popup) updateAnchorPosition() {
	pos := config.Position(p.config.Display.Position)
	offsetX := p.config.Display.OffsetX
	offsetY := p.config.Display.OffsetY + (p.position * (p.maxHeight + p.config.Display.Gap))

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

// SetStackCount updates the stack count badge.
// A count of 1 or less hides the badge.
func (p *Popup) SetStackCount(count int) {
	p.stackCount = count
	if p.stackCountLbl == nil {
		return
	}
	if count > 1 {
		p.stackCountLbl.SetText("(" + itoa(count) + ")")
		p.stackCountLbl.SetVisible(true)
	} else {
		p.stackCountLbl.SetVisible(false)
	}
}

// GetStackCount returns the current stack count.
func (p *Popup) GetStackCount() int {
	return p.stackCount
}

// IncrementStackCount increases the stack count by 1 and updates the display.
func (p *Popup) IncrementStackCount() {
	p.SetStackCount(p.stackCount + 1)
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
