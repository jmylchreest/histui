// testnotify - Send test notifications for histuid development
//
// Usage:
//
//	go run ./cmd/testnotify                    # Send all test notifications
//	go run ./cmd/testnotify --clear            # Clear all notifications first
//	go run ./cmd/testnotify --type image       # Send specific notification type
//	go run ./cmd/testnotify --screenshot       # Take screenshot after sending
//	go run ./cmd/testnotify --screenshot-dir . # Screenshot output directory
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/godbus/dbus/v5"
)

//go:embed kitty.png
var kittyIconData []byte

const (
	notifyDest   = "org.freedesktop.Notifications"
	notifyPath   = "/org/freedesktop/Notifications"
	notifyIface  = "org.freedesktop.Notifications"
	dunstCtlDest = "org.dunstproject.cmd"
	dunstCtlPath = "/org/dunstproject/cmd"
)

var delay = 300 * time.Millisecond

// randomDelay sleeps for a random duration between 200ms and 1200ms.
// This simulates realistic notification timing and helps expose race conditions.
func randomDelay() {
	ms := 200 + rand.Intn(1000) // 200-1199ms
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// testType defines a test type with its description.
type testType struct {
	name        string
	description string
}

// getTestTypes returns all available test types with descriptions.
func getTestTypes() []testType {
	return []testType{
		{"all", "Run all tests (8 phases)"},
		{"minimal", "No icon, no image (tests fallback)"},
		{"simple", "Icon name only (vesktop -> discord)"},
		{"url", "Notification with clickable URL"},
		{"image", "Notification with app icon"},
		{"imagedata", "Medium image-data (~64KB)"},
		{"imagedata-small", "Small image-data below 100 KiB threshold"},
		{"imagedata-large", "Large image-data above 100 KiB threshold"},
		{"tallimage", "Tall image (tests cropping)"},
		{"imagepath", "image-path hint (PNG/JPEG/GIF file)"},
		{"icon-and-imagedata", "Both app_icon AND image-data"},
		{"icon-and-imagepath", "Both app_icon AND image-path"},
		{"image-sizes", "All 10 sizes (tests 100 KiB threshold)"},
		{"progress", "Progress bar with stack tag"},
		{"stacktag", "Stack tag updates (replaces in place)"},
		{"progressupdate", "Progress via replaces_id"},
		{"actions", "Action buttons (prev/play/next)"},
		{"signal", "Signal-style with View action"},
		{"low", "Low urgency notification"},
		{"critical", "Critical urgency notification"},
		{"html", "HTML formatting (bold/italic/underline)"},
		{"long", "Long body text (tests wrapping)"},
		{"stack", "Burst of N notifications (use --stack N)"},
		{"duplicates", "Identical notifications (test stacking)"},
		{"kitty", "Kitty terminal style notification"},
		{"apps", "All 38 mock app notifications"},
	}
}

// printTestTypes prints all available test types in a formatted table.
func printTestTypes() {
	types := getTestTypes()

	// Find max name length for alignment
	maxLen := 0
	for _, t := range types {
		if len(t.name) > maxLen {
			maxLen = len(t.name)
		}
	}

	fmt.Println("Available test types:")
	fmt.Println()
	for _, t := range types {
		fmt.Printf("  %-*s  %s\n", maxLen, t.name, t.description)
	}
}

func main() {
	clearFlag := flag.Bool("clear", false, "Clear all notifications before sending")
	typeFlag := flag.String("type", "all", "Type of notification to send (use --list-types to see all)")
	listTypesFlag := flag.Bool("list-types", false, "List all available test types")
	stackCount := flag.Int("stack", 5, "Number of notifications for stack test")
	screenshotFlag := flag.Bool("screenshot", false, "Take screenshot after sending notifications")
	screenshotDir := flag.String("screenshot-dir", "/tmp/histui-test", "Directory to save screenshots")
	flag.Parse()

	if *listTypesFlag {
		printTestTypes()
		return
	}

	conn, err := dbus.SessionBus()
	if err != nil {
		log.Fatalf("Failed to connect to session bus: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if *clearFlag {
		clearNotifications(conn)
	}

	switch *typeFlag {
	case "all":
		runAllTests(conn)
	case "minimal":
		sendMinimal(conn)
	case "simple":
		sendSimple(conn)
	case "url":
		sendWithURL(conn)
	case "image":
		sendWithImage(conn)
	case "imagedata":
		sendWithImageData(conn)
	case "imagedata-small":
		sendWithSmallImageData(conn)
	case "imagedata-large":
		sendWithLargeImageData(conn)
	case "tallimage":
		sendWithTallImage(conn)
	case "progress":
		sendWithProgress(conn)
	case "stacktag":
		sendStackTagProgress(conn)
	case "actions":
		sendWithActions(conn)
	case "signal":
		sendSignalStyle(conn)
	case "low":
		sendLowUrgency(conn)
	case "critical":
		sendCriticalUrgency(conn)
	case "html":
		sendHTMLFormatted(conn)
	case "long":
		sendLongBody(conn)
	case "stack":
		sendStack(conn, *stackCount)
	case "duplicates":
		sendDuplicates(conn)
	case "progressupdate":
		sendProgressUpdating(conn)
	case "imagepath":
		sendWithImagePath(conn)
	case "icon-and-imagedata":
		sendIconAndImageData(conn)
	case "icon-and-imagepath":
		sendIconAndImagePath(conn)
	case "image-sizes":
		sendImageSizeTest(conn)
	case "kitty":
		sendKittyStyle(conn)
	case "apps":
		sendAppNotifications(conn)
	default:
		fmt.Printf("Unknown type: %s\n", *typeFlag)
		os.Exit(1)
	}

	if *screenshotFlag {
		takeScreenshot(*screenshotDir, *typeFlag)
	}
}

// takeScreenshot captures the screen using grim (Wayland) or scrot (X11).
func takeScreenshot(dir, testType string) {
	// Wait for notifications to render
	time.Sleep(500 * time.Millisecond)

	// Create output directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create screenshot directory: %v", err)
		return
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(dir, fmt.Sprintf("test-%s-%s.png", testType, timestamp))

	// Try grim first (Wayland), then scrot (X11)
	var cmd *exec.Cmd
	if _, err := exec.LookPath("grim"); err == nil {
		cmd = exec.Command("grim", filename)
	} else if _, err := exec.LookPath("scrot"); err == nil {
		cmd = exec.Command("scrot", filename)
	} else {
		log.Println("No screenshot tool found (grim or scrot)")
		return
	}

	if err := cmd.Run(); err != nil {
		log.Printf("Failed to take screenshot: %v", err)
		return
	}

	fmt.Printf("[OK] Screenshot saved: %s\n", filename)
}

func notify(conn *dbus.Conn, appName, summary, body, icon string, hints map[string]dbus.Variant, timeout int32) uint32 {
	return notifyWithReplacesID(conn, appName, summary, body, icon, hints, timeout, 0)
}

func notifyWithReplacesID(conn *dbus.Conn, appName, summary, body, icon string, hints map[string]dbus.Variant, timeout int32, replacesID uint32) uint32 {
	obj := conn.Object(notifyDest, notifyPath)
	var id uint32
	err := obj.Call(notifyIface+".Notify", 0,
		appName,    // app_name
		replacesID, // replaces_id
		icon,       // app_icon
		summary,    // summary
		body,       // body
		[]string{}, // actions
		hints,      // hints
		timeout,    // expire_timeout
	).Store(&id)
	if err != nil {
		log.Printf("Failed to send notification: %v", err)
		return 0
	}
	return id
}

func notifyWithActions(conn *dbus.Conn, appName, summary, body, icon string, actions []string, hints map[string]dbus.Variant, timeout int32) {
	obj := conn.Object(notifyDest, notifyPath)
	var id uint32
	err := obj.Call(notifyIface+".Notify", 0,
		appName,   // app_name
		uint32(0), // replaces_id
		icon,      // app_icon
		summary,   // summary
		body,      // body
		actions,   // actions
		hints,     // hints
		timeout,   // expire_timeout
	).Store(&id)
	if err != nil {
		log.Printf("Failed to send notification: %v", err)
		return
	}
	_ = id // ID available if needed for future tests
}

func clearNotifications(conn *dbus.Conn) {
	fmt.Println("[TEST] Clearing all notifications...")
	obj := conn.Object(dunstCtlDest, dunstCtlPath)
	_ = obj.Call("org.dunstproject.cmd0.NotificationCloseAll", 0).Err
	time.Sleep(500 * time.Millisecond)
}

func sendSimple(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending simple notification (vesktop -> discord)...")
	notify(conn, "vesktop", "New Message",
		"[TEST] You have a new message from @friend in #general",
		"vesktop", nil, 15000) // 15 seconds for normal urgency
	randomDelay()
}

func sendWithURL(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with URL (librewolf -> firefox)...")
	notify(conn, "librewolf", "Link Notification",
		`[TEST] Check out: <a href="https://github.com/jmylchreest/histui">histui on GitHub</a>`,
		"librewolf", nil, 15000)
	randomDelay()
}

func sendWithImage(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with icon...")
	// Try common icon paths
	icons := []string{
		"/usr/share/icons/hicolor/48x48/apps/firefox.png",
		"/usr/share/icons/hicolor/48x48/apps/chromium.png",
		"/usr/share/icons/Adwaita/48x48/apps/utilities-terminal.png",
		"dialog-information",
	}
	var icon string
	for _, path := range icons {
		if _, err := os.Stat(path); err == nil || path == "dialog-information" {
			icon = path
			break
		}
	}
	notify(conn, "Image Test", "Notification with Icon",
		"[TEST] This notification includes an application icon.",
		icon, nil, 15000)
	randomDelay()
}

// ImageDataStruct matches the D-Bus signature (iiibiiay) for image-data hint.
type ImageDataStruct struct {
	Width         int32
	Height        int32
	Rowstride     int32
	HasAlpha      bool
	BitsPerSample int32
	Channels      int32
	Data          []byte
}

// TestImageSize defines standard test image sizes with their purpose.
type TestImageSize struct {
	Name        string
	Width       int32
	Height      int32
	DataSize    int // Approximate RGBA size in bytes
	Description string
}

// Test image sizes - designed to test the 100 KiB threshold
var testImageSizes = []TestImageSize{
	{"tiny", 32, 32, 4096, "~4 KB - well below threshold"},
	{"small", 64, 64, 16384, "~16 KB - typical profile pic"},
	{"medium", 128, 128, 65536, "~64 KB - below threshold"},
	{"threshold", 160, 160, 102400, "~100 KiB - exactly at threshold"},
	{"above", 200, 200, 160000, "~156 KB - above threshold"},
	{"large", 300, 300, 360000, "~351 KB - album art size"},
	{"xlarge", 500, 500, 1000000, "~977 KB - large content"},
	{"hd", 400, 225, 360000, "~351 KB - 16:9 aspect"},
	{"tall", 200, 600, 480000, "~469 KB - tall/portrait"},
	{"wide", 600, 200, 480000, "~469 KB - wide/banner"},
}

// PatternType defines different image patterns for testing.
type PatternType int

const (
	PatternSolid PatternType = iota
	PatternHorizontalStripes
	PatternVerticalStripes
	PatternDiagonalStripes
	PatternCheckerboard
	PatternGradientH
	PatternGradientV
	PatternGradientDiag
	PatternCircle
	PatternNoise
	PatternBorder
	PatternGrid
)

// Color represents an RGBA color.
type Color struct {
	R, G, B, A byte
}

// Predefined color palettes for testing
var (
	// Dark colors
	colorBlack      = Color{20, 20, 20, 255}
	colorDarkGray   = Color{50, 50, 50, 255}
	colorDarkBlue   = Color{30, 40, 80, 255}
	colorDarkGreen  = Color{30, 60, 40, 255}
	colorDarkRed    = Color{80, 30, 30, 255}
	colorDarkPurple = Color{60, 30, 80, 255}

	// Light colors
	colorWhite      = Color{240, 240, 240, 255}
	colorLightGray  = Color{200, 200, 200, 255}
	colorLightBlue  = Color{180, 200, 240, 255}
	colorLightGreen = Color{180, 230, 190, 255}
	colorLightPink  = Color{240, 200, 210, 255}
	colorCream      = Color{255, 250, 240, 255}

	// Vibrant colors
	colorRed     = Color{220, 60, 60, 255}
	colorOrange  = Color{240, 150, 50, 255}
	colorYellow  = Color{240, 220, 60, 255}
	colorGreen   = Color{60, 180, 80, 255}
	colorCyan    = Color{60, 200, 220, 255}
	colorBlue    = Color{60, 100, 220, 255}
	colorPurple  = Color{150, 80, 200, 255}
	colorMagenta = Color{220, 80, 180, 255}

	// All colors for random selection
	allColors = []Color{
		colorBlack, colorDarkGray, colorDarkBlue, colorDarkGreen, colorDarkRed, colorDarkPurple,
		colorWhite, colorLightGray, colorLightBlue, colorLightGreen, colorLightPink, colorCream,
		colorRed, colorOrange, colorYellow, colorGreen, colorCyan, colorBlue, colorPurple, colorMagenta,
	}

	darkColors = []Color{
		colorBlack, colorDarkGray, colorDarkBlue, colorDarkGreen, colorDarkRed, colorDarkPurple,
	}

	lightColors = []Color{
		colorWhite, colorLightGray, colorLightBlue, colorLightGreen, colorLightPink, colorCream,
	}

	vibrantColors = []Color{
		colorRed, colorOrange, colorYellow, colorGreen, colorCyan, colorBlue, colorPurple, colorMagenta,
	}
)

// generateTestImage creates a test image with the specified size and pattern.
func generateTestImage(width, height int32, pattern PatternType, colors []Color) ImageDataStruct {
	channels := int32(4) // RGBA
	rowstride := width * channels
	pixels := make([]byte, height*rowstride)

	// Pick colors for the pattern
	if len(colors) == 0 {
		colors = []Color{colorBlue, colorWhite}
	}
	c1 := colors[0]
	c2 := colorWhite
	if len(colors) > 1 {
		c2 = colors[1]
	}

	for y := int32(0); y < height; y++ {
		for x := int32(0); x < width; x++ {
			offset := y*rowstride + x*channels
			var c Color

			switch pattern {
			case PatternSolid:
				c = c1

			case PatternHorizontalStripes:
				stripeWidth := height / 8
				if stripeWidth < 4 {
					stripeWidth = 4
				}
				if (y/stripeWidth)%2 == 0 {
					c = c1
				} else {
					c = c2
				}

			case PatternVerticalStripes:
				stripeWidth := width / 8
				if stripeWidth < 4 {
					stripeWidth = 4
				}
				if (x/stripeWidth)%2 == 0 {
					c = c1
				} else {
					c = c2
				}

			case PatternDiagonalStripes:
				stripeWidth := (width + height) / 16
				if stripeWidth < 4 {
					stripeWidth = 4
				}
				if ((x+y)/stripeWidth)%2 == 0 {
					c = c1
				} else {
					c = c2
				}

			case PatternCheckerboard:
				cellSize := width / 8
				if cellSize < 4 {
					cellSize = 4
				}
				if ((x/cellSize)+(y/cellSize))%2 == 0 {
					c = c1
				} else {
					c = c2
				}

			case PatternGradientH:
				ratio := float32(x) / float32(width)
				c = blendColors(c1, c2, ratio)

			case PatternGradientV:
				ratio := float32(y) / float32(height)
				c = blendColors(c1, c2, ratio)

			case PatternGradientDiag:
				ratio := (float32(x)/float32(width) + float32(y)/float32(height)) / 2
				c = blendColors(c1, c2, ratio)

			case PatternCircle:
				centerX := float32(width) / 2
				centerY := float32(height) / 2
				radius := float32(min(width, height)) / 2.2
				dx := float32(x) - centerX
				dy := float32(y) - centerY
				if dx*dx+dy*dy <= radius*radius {
					c = c1
				} else {
					c = c2
				}

			case PatternNoise:
				// Deterministic "noise" based on position
				hash := (x*7919 + y*6271) % 256
				if hash < 128 {
					c = c1
				} else {
					c = c2
				}

			case PatternBorder:
				borderWidth := min(width, height) / 10
				if borderWidth < 2 {
					borderWidth = 2
				}
				if x < borderWidth || x >= width-borderWidth || y < borderWidth || y >= height-borderWidth {
					c = c1
				} else {
					c = c2
				}

			case PatternGrid:
				lineWidth := int32(2)
				cellSize := width / 6
				if cellSize < 8 {
					cellSize = 8
				}
				if x%cellSize < lineWidth || y%cellSize < lineWidth {
					c = c1
				} else {
					c = c2
				}

			default:
				c = c1
			}

			pixels[offset] = c.R
			pixels[offset+1] = c.G
			pixels[offset+2] = c.B
			pixels[offset+3] = c.A
		}
	}

	return ImageDataStruct{
		Width:         width,
		Height:        height,
		Rowstride:     rowstride,
		HasAlpha:      true,
		BitsPerSample: 8,
		Channels:      channels,
		Data:          pixels,
	}
}

// blendColors linearly interpolates between two colors.
func blendColors(c1, c2 Color, ratio float32) Color {
	return Color{
		R: byte(float32(c1.R)*(1-ratio) + float32(c2.R)*ratio),
		G: byte(float32(c1.G)*(1-ratio) + float32(c2.G)*ratio),
		B: byte(float32(c1.B)*(1-ratio) + float32(c2.B)*ratio),
		A: 255,
	}
}

// randomColor returns a random color from the given palette.
func randomColor(palette []Color) Color {
	return palette[rand.Intn(len(palette))]
}

// randomPattern returns a random pattern type.
func randomPattern() PatternType {
	patterns := []PatternType{
		PatternSolid, PatternHorizontalStripes, PatternVerticalStripes,
		PatternDiagonalStripes, PatternCheckerboard, PatternGradientH,
		PatternGradientV, PatternGradientDiag, PatternCircle, PatternNoise,
		PatternBorder, PatternGrid,
	}
	return patterns[rand.Intn(len(patterns))]
}

// patternName returns a human-readable name for a pattern.
func patternName(p PatternType) string {
	names := []string{
		"solid", "h-stripes", "v-stripes", "diag-stripes", "checkerboard",
		"gradient-h", "gradient-v", "gradient-diag", "circle", "noise",
		"border", "grid",
	}
	if int(p) < len(names) {
		return names[p]
	}
	return "unknown"
}

// generateRandomTestImage creates a unique random test image.
func generateRandomTestImage(size TestImageSize) (ImageDataStruct, string) {
	pattern := randomPattern()

	// Pick two contrasting colors
	var colors []Color
	switch rand.Intn(4) {
	case 0: // Dark theme
		colors = []Color{randomColor(darkColors), randomColor(lightColors)}
	case 1: // Light theme
		colors = []Color{randomColor(lightColors), randomColor(darkColors)}
	case 2: // Vibrant
		colors = []Color{randomColor(vibrantColors), randomColor(vibrantColors)}
	case 3: // Mixed
		colors = []Color{randomColor(allColors), randomColor(allColors)}
	}

	img := generateTestImage(size.Width, size.Height, pattern, colors)
	desc := fmt.Sprintf("%s %dx%d %s", patternName(pattern), size.Width, size.Height, size.Description)
	return img, desc
}

// ImageFormat represents supported image file formats.
type ImageFormat int

const (
	FormatPNG ImageFormat = iota
	FormatJPEG
	FormatGIF
)

// imageFormatInfo contains format details.
type imageFormatInfo struct {
	name      string
	extension string
}

var imageFormats = []imageFormatInfo{
	{name: "PNG", extension: ".png"},
	{name: "JPEG", extension: ".jpg"},
	{name: "GIF", extension: ".gif"},
}

// randomImageFormat returns a random image format.
func randomImageFormat() ImageFormat {
	return ImageFormat(rand.Intn(len(imageFormats)))
}

// imageDataToImage converts our ImageDataStruct to Go's image.Image.
func imageDataToImage(data ImageDataStruct) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, int(data.Width), int(data.Height)))
	for y := int32(0); y < data.Height; y++ {
		for x := int32(0); x < data.Width; x++ {
			offset := y*data.Rowstride + x*data.Channels
			r := data.Data[offset]
			g := data.Data[offset+1]
			b := data.Data[offset+2]
			a := data.Data[offset+3]
			img.SetRGBA(int(x), int(y), color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	return img
}

// writeImageFile writes an image to a file in the specified format.
// Returns the file path and format name.
func writeImageFile(data ImageDataStruct, format ImageFormat) (string, string, error) {
	info := imageFormats[format]
	filePath := filepath.Join(os.TempDir(), fmt.Sprintf("histui-test-%d%s", time.Now().UnixNano(), info.extension))

	f, err := os.Create(filePath)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = f.Close() }()

	img := imageDataToImage(data)

	switch format {
	case FormatPNG:
		err = png.Encode(f, img)
	case FormatJPEG:
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	case FormatGIF:
		err = gif.Encode(f, img, nil)
	default:
		err = png.Encode(f, img)
	}

	if err != nil {
		_ = os.Remove(filePath)
		return "", "", err
	}

	return filePath, info.name, nil
}

// writeRandomImageFile creates a random test image and writes it to a file in a random format.
func writeRandomImageFile(size TestImageSize) (string, string, error) {
	imageData, _ := generateRandomTestImage(size)
	format := randomImageFormat()
	return writeImageFile(imageData, format)
}

func sendWithImageData(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with embedded image-data...")

	// Use the new generator for a medium-sized image with random pattern
	size := testImageSizes[2] // medium: 128x128
	imageData, desc := generateRandomTestImage(size)

	hints := map[string]dbus.Variant{
		"image-data": dbus.MakeVariant(imageData),
	}

	notify(conn, "Image Data Test", "Embedded Image",
		fmt.Sprintf("[TEST] %s (%d bytes)", desc, len(imageData.Data)),
		"", hints, 15000)
	randomDelay()
}

func sendWithTallImage(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with tall image (tests shrink + cropping)...")

	// Use the tall size from our test sizes
	size := testImageSizes[8] // tall: 200x600
	imageData := generateTestImage(size.Width, size.Height, PatternDiagonalStripes, []Color{colorCyan, colorDarkBlue})

	hints := map[string]dbus.Variant{
		"image-data": dbus.MakeVariant(imageData),
	}

	notify(conn, "Tall Image Test", "Cropped Image",
		fmt.Sprintf("[TEST] Tall image %dx%d (%d bytes) - tests cropping with fade", size.Width, size.Height, len(imageData.Data)),
		"", hints, 15000)
	randomDelay()
}

func sendWithProgress(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending progress notification with stack tag updates...")

	// Use stack tag so progress updates replace each other
	// Stack tag includes filename AND timestamp for uniqueness per download session
	filename := "ubuntu-24.04.iso"
	stackTag := fmt.Sprintf("qbittorrent:download:%s:%d", filename, time.Now().Unix())

	steps := []struct {
		percent int32
		body    string
	}{
		{10, "[TEST] " + filename + " - 10%"},
		{35, "[TEST] " + filename + " - 35%"},
		{60, "[TEST] " + filename + " - 60%"},
		{85, "[TEST] " + filename + " - 85%"},
		{100, "[TEST] " + filename + " - Complete!"},
	}

	for _, step := range steps {
		hints := map[string]dbus.Variant{
			"value":             dbus.MakeVariant(step.percent),
			"x-dunst-stack-tag": dbus.MakeVariant(stackTag),
		}
		notify(conn, "qbittorrent", "Downloading File", step.body, "qbittorrent", hints, 15000)
		fmt.Printf("  -> Progress: %d%%\n", step.percent)
		time.Sleep(600 * time.Millisecond)
	}
	randomDelay()
}

func sendWithActions(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with actions (rhythmbox -> music)...")
	actions := []string{
		"prev", "Previous",
		"play", "Play/Pause",
		"next", "Next",
	}
	notifyWithActions(conn, "rhythmbox", "Now Playing",
		"[TEST] Miles Davis - So What",
		"rhythmbox", actions, nil, 5000)
	randomDelay()
}

func sendLowUrgency(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending low urgency notification (spotify -> spotify)...")
	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(byte(0)), // Low
	}
	notify(conn, "spotify", "Now Playing",
		"[TEST] Artist Name - Song Title (Discover Weekly)",
		"spotify", hints, 5000) // 5 seconds for low urgency
	randomDelay()
}

func sendCriticalUrgency(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending critical urgency notification (gufw -> security)...")
	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(byte(2)), // Critical
	}
	notify(conn, "gufw", "Firewall Alert!",
		"[TEST] Blocked incoming connection from suspicious IP.",
		"gufw", hints, 30000) // 30 seconds for critical
	randomDelay()
}

func sendHTMLFormatted(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending HTML formatted notification (telegram-desktop -> telegram)...")
	notify(conn, "telegram-desktop", "Message from Alice",
		"[TEST] <b>Bold</b>, <i>italic</i>, and <u>underlined</u> text.",
		"telegram-desktop", nil, 15000)
	randomDelay()
}

func sendLongBody(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with long body (thunderbird -> email)...")
	notify(conn, "thunderbird", "New Email from John Doe",
		"[TEST] Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam.",
		"thunderbird", nil, 15000)
	randomDelay()
}

func sendStack(conn *dbus.Conn, count int) {
	fmt.Printf("[TEST] Sending stack of %d notifications...\n", count)
	for i := 1; i <= count; i++ {
		var hints map[string]dbus.Variant

		// Include a random image on every 3rd notification
		if i%3 == 0 {
			// Pick a random size that's above threshold so it shows
			sizes := []TestImageSize{testImageSizes[4], testImageSizes[5], testImageSizes[7]} // above, large, hd
			size := sizes[rand.Intn(len(sizes))]
			imageData, desc := generateRandomTestImage(size)

			hints = map[string]dbus.Variant{
				"image-data": dbus.MakeVariant(imageData),
			}
			notify(conn, "Stack Test", fmt.Sprintf("Notification %d of %d (with image)", i, count),
				fmt.Sprintf("[TEST] %s", desc),
				"", hints, 10000)
		} else {
			notify(conn, "Stack Test", fmt.Sprintf("Notification %d of %d", i, count),
				fmt.Sprintf("[TEST] Notification number %d in the stack.", i),
				"", nil, 10000)
		}
		time.Sleep(100 * time.Millisecond)
	}
	randomDelay()
}

func sendKittyStyle(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending Kitty-style notification (replicating exact format)...")

	// Replicate exactly what Kitty sends based on captured history:
	// app_name: "kitty"
	// summary: "Claude Code"
	// body: "Claude needs your permission to use Bash"
	// expire_timeout: -1
	// urgency: 1 (normal)
	// icon (app_icon): "/usr/lib/kitty/logo/kitty.png" (absolute path to PNG)
	// actions: [{"key": "default", "label": " "}]

	// Write embedded kitty icon to temp file (mimics kitty sending absolute path to PNG)
	iconPath := filepath.Join(os.TempDir(), "histui-test-kitty.png")
	if err := os.WriteFile(iconPath, kittyIconData, 0644); err != nil {
		log.Printf("Failed to write kitty icon: %v", err)
		iconPath = ""
	}
	defer func() {
		if iconPath != "" {
			_ = os.Remove(iconPath)
		}
	}()

	actions := []string{
		"default", " ", // Kitty uses a single space as the label
	}

	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(byte(1)), // Normal urgency
	}

	notifyWithActions(conn, "kitty", "Claude Code",
		"Claude needs your permission to use Bash",
		iconPath, // app_icon as absolute path to PNG file
		actions,
		hints,
		-1, // expire_timeout: never expires
	)
	randomDelay()
}

func sendWithImagePath(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with image-path hint (file-based image)...")

	// Use a size above threshold and random format
	size := testImageSizes[5] // large: 300x300
	filePath, formatName, err := writeRandomImageFile(size)
	if err != nil {
		log.Printf("Failed to create image file: %v", err)
		return
	}

	hints := map[string]dbus.Variant{
		"image-path": dbus.MakeVariant(filePath),
	}

	notify(conn, "Image Path Test", fmt.Sprintf("File-Based Image (%s)", formatName),
		fmt.Sprintf("[TEST] image-path hint loading %s file. Always shown (explicit paths are intentional).", formatName),
		"", hints, 10000)

	// Give notification daemon time to read the file, then clean up
	time.Sleep(delay)
	_ = os.Remove(filePath)
	randomDelay()
}

// sendStackTagProgress sends progress updates using stack tag (dunst-compatible).
// This tests the x-dunst-stack-tag hint - notifications with the same tag replace each other.
// No need to track IDs - just use the same tag!
// The stack tag includes filename AND timestamp for uniqueness per download session.
func sendStackTagProgress(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending stack tag progress updates (like dunstify -h string:x-dunst-stack-tag:download)...")

	// Simulate downloading a file - stack tag includes filename and timestamp for uniqueness
	filename := "ubuntu-24.04.iso"
	stackTag := fmt.Sprintf("qbittorrent:download:%s:%d", filename, time.Now().Unix())

	// Send progress updates with the same stack tag
	steps := []struct {
		percent int32
		body    string
	}{
		{0, "[TEST] " + filename + " - Starting..."},
		{25, "[TEST] " + filename + " - 25%"},
		{50, "[TEST] " + filename + " - 50%"},
		{75, "[TEST] " + filename + " - 75%"},
		{100, "[TEST] " + filename + " - Complete!"},
	}

	for _, step := range steps {
		hints := map[string]dbus.Variant{
			"value":             dbus.MakeVariant(step.percent),
			"x-dunst-stack-tag": dbus.MakeVariant(stackTag),
		}
		notify(conn, "qbittorrent", "Downloading File", step.body, "qbittorrent", hints, 5000)
		fmt.Printf("  -> Progress: %d%%\n", step.percent)
		time.Sleep(800 * time.Millisecond)
	}
	randomDelay()
}

// sendProgressUpdating sends a notification and then updates it with increasing progress.
// This tests the replaces_id behavior - when the same ID is used, the notification
// should be updated in place rather than creating a new one.
func sendProgressUpdating(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending progress update sequence (tests replaces_id)...")

	// Send initial notification at 0%
	hints := map[string]dbus.Variant{
		"value": dbus.MakeVariant(int32(0)),
	}
	id := notify(conn, "qbittorrent", "Downloading File",
		"[TEST] large-file.tar.gz - Starting...",
		"qbittorrent", hints, 0) // 0 = never expires (we'll close it manually)

	if id == 0 {
		fmt.Println("[WARN] Failed to send initial notification")
		return
	}

	// Update progress in steps
	steps := []struct {
		percent int32
		body    string
	}{
		{20, "[TEST] large-file.tar.gz - 20%"},
		{45, "[TEST] large-file.tar.gz - 45%"},
		{70, "[TEST] large-file.tar.gz - 70%"},
		{90, "[TEST] large-file.tar.gz - 90%"},
		{100, "[TEST] large-file.tar.gz - Complete!"},
	}

	for _, step := range steps {
		time.Sleep(800 * time.Millisecond)

		hints := map[string]dbus.Variant{
			"value": dbus.MakeVariant(step.percent),
		}
		notifyWithReplacesID(conn, "qbittorrent", "Downloading File",
			step.body, "qbittorrent", hints, 5000, id)

		fmt.Printf("  -> Progress: %d%%\n", step.percent)
	}
	randomDelay()
}

// sendSignalStyle sends a Signal-like notification with a "View" action button.
// This mimics the deep-link behavior where clicking "View" opens the chat.
func sendSignalStyle(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending Signal-style notification with View action...")

	actions := []string{
		"default", "View", // "default" is the action key, "View" is the button label
	}

	hints := map[string]dbus.Variant{
		"urgency":  dbus.MakeVariant(byte(1)), // Normal urgency
		"category": dbus.MakeVariant("im.received"),
	}

	notifyWithActions(conn, "signal-desktop", "Alice",
		"[TEST] Hey! Are you coming to the meeting today?",
		"signal-desktop",
		actions,
		hints,
		5000)
	randomDelay()
}

// sendDuplicates sends identical notifications to test duplicate stacking.
// When StackDuplicates is enabled, these should stack with a count badge.
func sendDuplicates(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending duplicate notifications (tests stacking)...")

	// Send 4 identical notifications from the same app
	for i := 1; i <= 4; i++ {
		notify(conn, "discord", "New Message",
			"[TEST] @everyone Someone mentioned you in #general",
			"discord", nil, 15000)
		fmt.Printf("  -> Duplicate %d sent\n", i)
		time.Sleep(200 * time.Millisecond)
	}
	randomDelay()
}

func runAllTests(conn *dbus.Conn) {
	fmt.Println("[TEST] Starting comprehensive notification tests...")
	fmt.Println()
	fmt.Println("=== Phase 1: Basic variations (icon types) ===")

	// 1. Minimal - no icon, no image (tests fallback)
	sendMinimal(conn)

	// 2. Simple - icon name only
	sendSimple(conn)

	// 3. Icon as file path (kitty style)
	sendKittyStyle(conn)

	fmt.Println()
	fmt.Println("=== Phase 2: Image handling (image-data vs image-path) ===")

	// 4. Small image-data (below threshold - should NOT show in body)
	sendWithSmallImageData(conn)

	// 5. Large image-data (above threshold - should show in body)
	sendWithLargeImageData(conn)

	// 6. image-path hint (always shown)
	sendWithImagePath(conn)

	// 7. Tall image (tests cropping)
	sendWithTallImage(conn)

	fmt.Println()
	fmt.Println("=== Phase 3: Combined icon + image ===")

	// 8. Both app_icon AND image-data (icon in header, image in body)
	sendIconAndImageData(conn)

	// 9. Both app_icon AND image-path
	sendIconAndImagePath(conn)

	fmt.Println()
	fmt.Println("=== Phase 4: Urgency levels ===")

	// 10. Low urgency (quick timeout)
	sendLowUrgency(conn)

	// 11. Critical urgency (long timeout, different styling)
	sendCriticalUrgency(conn)

	fmt.Println()
	fmt.Println("=== Phase 5: Content formatting ===")

	// 12. URL in body
	sendWithURL(conn)

	// 13. HTML formatting
	sendHTMLFormatted(conn)

	// 14. Long body text (tests truncation/wrapping)
	sendLongBody(conn)

	fmt.Println()
	fmt.Println("=== Phase 6: Interactive features ===")

	// 15. Actions (buttons)
	sendWithActions(conn)

	// 16. Signal-style with View action
	sendSignalStyle(conn)

	fmt.Println()
	fmt.Println("=== Phase 7: Stacking and updates ===")

	// 17. Duplicate stacking (same content)
	sendDuplicates(conn)

	// 18. Progress with stack tag (updates in place)
	sendStackTagProgress(conn)

	fmt.Println()
	fmt.Println("=== Phase 8: App variety ===")

	// 19. Random sample of different apps
	sendRandomAppSample(conn, 3)

	fmt.Println()
	fmt.Println("[OK] All test notifications sent!")
	fmt.Println()
	fmt.Println("Test summary:")
	fmt.Println("  - Minimal (no icon/image): should use fallback symbol")
	fmt.Println("  - Small image-data (~16KB): should NOT appear in body (below 100 KiB)")
	fmt.Println("  - Large image-data (~351KB): SHOULD appear in body (above 100 KiB)")
	fmt.Println("  - Icon + image-data: icon in header, image in body")
	fmt.Println("  - image-path: always shown (explicit file paths are intentional)")
}

// appNotification defines a mock notification from an app.
type appNotification struct {
	appName string
	icon    string // icon name or path
	summary string
	body    string
	urgency byte
}

// getTestApps returns the list of mock app notifications for testing icon resolution.
func getTestApps() []appNotification {
	return []appNotification{
		// Messaging apps
		{"discord", "discord", "New Message", "You have a new message from @user", 1},
		{"vesktop", "vesktop", "Server Alert", "Someone mentioned you in #general", 1},
		{"telegram-desktop", "telegram", "Telegram", "New message from Alice", 1},
		{"org.telegram.desktop", "telegram", "Group Chat", "5 new messages in Work Group", 1},
		{"whatsapp-desktop", "whatsapp", "WhatsApp", "Message from Mom", 1},
		{"elecwhat", "elecwhat", "WhatsApp Web", "New message received", 1},
		{"signal-desktop", "signal", "Signal", "Encrypted message received", 1},
		{"slack", "slack", "Slack", "New message in #engineering", 1},
		{"Element", "element", "Matrix", "New message in !room:matrix.org", 1},

		// Browsers
		{"firefox", "firefox", "Download Complete", "document.pdf has finished downloading", 0},
		{"librewolf", "librewolf", "Page Crashed", "A tab has crashed unexpectedly", 2},
		{"google-chrome", "google-chrome", "Chrome", "Update available", 1},
		{"chromium", "chromium", "Chromium", "Permission requested", 1},
		{"brave-browser", "brave", "Brave", "Shields blocked 42 trackers", 0},

		// Media
		{"spotify", "spotify", "Now Playing", "Artist - Song Title", 0},
		{"Spotify", "spotify", "Spotify", "Your Discover Weekly is ready", 1},
		{"vlc", "vlc", "VLC", "Playback started", 0},
		{"rhythmbox", "rhythmbox", "Rhythmbox", "Now playing: Jazz Album", 0},

		// Development
		{"code", "code", "VS Code", "Extension update available", 0},
		{"code-oss", "code-oss", "VS Code OSS", "Build completed successfully", 1},
		{"jetbrains-idea", "idea", "IntelliJ IDEA", "Indexing complete", 0},
		{"docker-desktop", "docker", "Docker", "Container started", 1},

		// System
		{"gnome-software", "gnome-software", "Software", "Updates available", 1},
		{"org.kde.discover", "discover", "Discover", "3 updates available", 1},
		{"nm-applet", "nm-applet", "Network", "Connected to WiFi", 0},
		{"blueman", "blueman", "Bluetooth", "Device connected", 1},

		// Email
		{"thunderbird", "thunderbird", "New Email", "You have 3 new messages", 1},
		{"evolution", "evolution", "Calendar Reminder", "Meeting in 15 minutes", 2},
		{"geary", "geary", "Geary", "New message from work@example.com", 1},

		// Cloud & Sync
		{"dropbox", "dropbox", "Dropbox", "File synced successfully", 0},
		{"nextcloud", "nextcloud", "Nextcloud", "Sync complete", 0},

		// Gaming
		{"steam", "steam", "Steam", "Friend is now playing", 0},
		{"lutris", "lutris", "Lutris", "Game installation complete", 1},

		// Other
		{"keepassxc", "keepassxc", "KeePassXC", "Database locked due to inactivity", 1},
		{"obs-studio", "obs-studio", "OBS", "Recording started", 1},
		{"flameshot", "flameshot", "Flameshot", "Screenshot saved", 0},
	}
}

func sendRandomAppSample(conn *dbus.Conn, count int) {
	fmt.Printf("[TEST] Sending %d random app notifications...\n", count)

	apps := getTestApps()

	// Shuffle and pick first 'count' apps
	rand.Shuffle(len(apps), func(i, j int) {
		apps[i], apps[j] = apps[j], apps[i]
	})

	if count > len(apps) {
		count = len(apps)
	}

	for _, app := range apps[:count] {
		hints := map[string]dbus.Variant{
			"urgency": dbus.MakeVariant(app.urgency),
		}
		notify(conn, app.appName, app.summary, app.body, app.icon, hints, 5000)
		time.Sleep(100 * time.Millisecond)
	}
	randomDelay()
}

func sendAppNotifications(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending all app notifications (testing icon resolution)...")

	apps := getTestApps()
	for _, app := range apps {
		hints := map[string]dbus.Variant{
			"urgency": dbus.MakeVariant(app.urgency),
		}
		notify(conn, app.appName, app.summary, app.body, app.icon, hints, 5000)
		time.Sleep(100 * time.Millisecond)
	}
	randomDelay()
}

// sendMinimal sends a completely bare notification with no icon and no image.
// This tests the fallback behavior when no visual elements are provided.
func sendMinimal(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending minimal notification (no icon, no image)...")
	notify(conn, "Minimal Test", "Plain Notification",
		"[TEST] This notification has no app_icon and no image hints.",
		"", nil, 10000)
	randomDelay()
}

// sendWithSmallImageData sends a notification with image-data below the 100 KiB threshold.
// With the default image_data_preview_size=100KiB, this should NOT be displayed in the body.
// This simulates profile pictures from messaging apps like Signal/Discord.
func sendWithSmallImageData(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with SMALL image-data (below 100 KiB threshold)...")

	// Use "small" size: 64x64 = ~16 KB - well below threshold
	size := testImageSizes[1] // small: 64x64
	imageData := generateTestImage(size.Width, size.Height, PatternCircle, []Color{colorBlue, colorLightGray})

	hints := map[string]dbus.Variant{
		"image-data": dbus.MakeVariant(imageData),
	}

	dataSize := len(imageData.Data)
	notify(conn, "signal-desktop", "Alice",
		fmt.Sprintf("[TEST] Small image %dx%d (%d bytes, ~%d KB). Should NOT appear in body.", size.Width, size.Height, dataSize, dataSize/1024),
		"signal-desktop", hints, 15000)
	randomDelay()
}

// sendWithLargeImageData sends a notification with image-data above the 100 KiB threshold.
// With the default image_data_preview_size=100KiB, this SHOULD be displayed in the body.
// This simulates album art from music players like Spotify.
func sendWithLargeImageData(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with LARGE image-data (above 100 KiB threshold)...")

	// Use "large" size: 300x300 = ~351 KB - well above threshold
	size := testImageSizes[5] // large: 300x300
	imageData := generateTestImage(size.Width, size.Height, PatternGradientDiag, []Color{colorPurple, colorOrange})

	hints := map[string]dbus.Variant{
		"image-data": dbus.MakeVariant(imageData),
	}

	dataSize := len(imageData.Data)
	notify(conn, "spotify", "Now Playing",
		fmt.Sprintf("[TEST] Large image %dx%d (%d bytes, ~%d KB). SHOULD appear in body.", size.Width, size.Height, dataSize, dataSize/1024),
		"spotify", hints, 15000)
	randomDelay()
}

// sendIconAndImageData sends a notification with BOTH app_icon AND image-data.
// This tests that app_icon appears in the header/sidebar and image-data in the body.
// The icon and image should appear in different places per freedesktop spec.
func sendIconAndImageData(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with BOTH app_icon AND image-data...")

	// Use "above" size: 200x200 = ~156 KB - above threshold so it shows
	size := testImageSizes[4] // above: 200x200
	imageData := generateTestImage(size.Width, size.Height, PatternCheckerboard, []Color{colorGreen, colorDarkGreen})

	hints := map[string]dbus.Variant{
		"image-data": dbus.MakeVariant(imageData),
	}

	// app_icon = "firefox" (icon name, looked up from theme)
	// image-data = checkered pattern (displayed in body)
	notify(conn, "firefox", "Download Complete",
		fmt.Sprintf("[TEST] app_icon=firefox, image-data=%dx%d checkerboard (%d bytes)", size.Width, size.Height, len(imageData.Data)),
		"firefox", hints, 15000)
	randomDelay()
}

// sendImageSizeTest sends notifications with all different image sizes to test the threshold.
func sendImageSizeTest(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notifications with all image sizes (testing 100 KiB threshold)...")
	fmt.Println()

	for i, size := range testImageSizes {
		imageData, desc := generateRandomTestImage(size)
		dataSize := len(imageData.Data)

		// Determine if this should be shown based on 100 KiB threshold
		shouldShow := dataSize >= 102400
		expectation := "should NOT appear"
		if shouldShow {
			expectation = "SHOULD appear"
		}

		hints := map[string]dbus.Variant{
			"image-data": dbus.MakeVariant(imageData),
		}

		summary := fmt.Sprintf("Size Test %d: %s", i+1, size.Name)
		body := fmt.Sprintf("[TEST] %s\n%d bytes - %s in body", desc, dataSize, expectation)

		notify(conn, "Size Test", summary, body, "", hints, 10000)
		fmt.Printf("  -> %s: %d bytes (%s)\n", size.Name, dataSize, expectation)
		time.Sleep(300 * time.Millisecond)
	}
	randomDelay()
}

// sendIconAndImagePath sends a notification with BOTH app_icon AND image-path hint.
// This tests that app_icon appears in the header/sidebar and image-path in the body.
func sendIconAndImagePath(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with BOTH app_icon AND image-path...")

	// Use a size above threshold and random format
	size := testImageSizes[4] // above: 200x200
	filePath, formatName, err := writeRandomImageFile(size)
	if err != nil {
		log.Printf("Failed to create image file: %v", err)
		return
	}

	hints := map[string]dbus.Variant{
		"image-path": dbus.MakeVariant(filePath),
	}

	// app_icon = "thunderbird" (icon name)
	// image-path = random pattern image file (displayed in body)
	notify(conn, "thunderbird", "New Email",
		fmt.Sprintf("[TEST] app_icon=thunderbird (header icon), image-path=%s %dx%d (body image).", formatName, size.Width, size.Height),
		"thunderbird", hints, 15000)

	// Clean up after daemon has time to read the file
	time.Sleep(delay)
	_ = os.Remove(filePath)
	randomDelay()
}
