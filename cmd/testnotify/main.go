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

func main() {
	clearFlag := flag.Bool("clear", false, "Clear all notifications before sending")
	typeFlag := flag.String("type", "all", "Type of notification to send (all, simple, url, image, imagedata, tallimage, progress, stacktag, progressupdate, actions, signal, low, critical, html, long, stack, duplicates, apps)")
	stackCount := flag.Int("stack", 5, "Number of notifications for stack test")
	screenshotFlag := flag.Bool("screenshot", false, "Take screenshot after sending notifications")
	screenshotDir := flag.String("screenshot-dir", "/tmp/histui-test", "Directory to save screenshots")
	flag.Parse()

	conn, err := dbus.SessionBus()
	if err != nil {
		log.Fatalf("Failed to connect to session bus: %v", err)
	}
	defer conn.Close()

	if *clearFlag {
		clearNotifications(conn)
	}

	switch *typeFlag {
	case "all":
		runAllTests(conn)
	case "simple":
		sendSimple(conn)
	case "url":
		sendWithURL(conn)
	case "image":
		sendWithImage(conn)
	case "imagedata":
		sendWithImageData(conn)
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

func notifyWithActions(conn *dbus.Conn, appName, summary, body, icon string, actions []string, hints map[string]dbus.Variant, timeout int32) uint32 {
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
		return 0
	}
	return id
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
		"vesktop", nil, 5000)
	time.Sleep(delay)
}

func sendWithURL(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with URL (librewolf -> firefox)...")
	notify(conn, "librewolf", "Link Notification",
		`[TEST] Check out: <a href="https://github.com/jmylchreest/histui">histui on GitHub</a>`,
		"librewolf", nil, 5000)
	time.Sleep(delay)
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
		icon, nil, 5000)
	time.Sleep(delay)
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

func sendWithImageData(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with embedded image-data...")

	// Create a 32x32 red square image (RGBA)
	width, height := int32(32), int32(32)
	hasAlpha := true
	bitsPerSample := int32(8)
	channels := int32(4) // RGBA
	rowstride := width * channels

	// Create red pixel data
	pixels := make([]byte, height*rowstride)
	for y := int32(0); y < height; y++ {
		for x := int32(0); x < width; x++ {
			offset := y*rowstride + x*channels
			pixels[offset] = 255   // R
			pixels[offset+1] = 50  // G
			pixels[offset+2] = 50  // B
			pixels[offset+3] = 255 // A
		}
	}

	// Create the image-data struct: (iiibiiay)
	// godbus will automatically marshal this struct with the correct D-Bus signature
	imageData := ImageDataStruct{
		Width:         width,
		Height:        height,
		Rowstride:     rowstride,
		HasAlpha:      hasAlpha,
		BitsPerSample: bitsPerSample,
		Channels:      channels,
		Data:          pixels,
	}

	hints := map[string]dbus.Variant{
		"image-data": dbus.MakeVariant(imageData),
	}

	notify(conn, "Image Data Test", "Embedded Image",
		"[TEST] Raw pixel data embedded (32x32 red square).",
		"", hints, 5000)
	time.Sleep(delay)
}

func sendWithTallImage(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with tall image (tests shrink + cropping + gradient)...")

	// Create a large tall image (400x800) with a gradient from blue to green
	// This will trigger both shrinking to fit width and cropping with fade gradient
	width, height := int32(400), int32(800)
	hasAlpha := true
	bitsPerSample := int32(8)
	channels := int32(4) // RGBA
	rowstride := width * channels

	// Create 45-degree diagonal striped pattern (makes fade overlay more visible)
	pixels := make([]byte, height*rowstride)
	for y := int32(0); y < height; y++ {
		for x := int32(0); x < width; x++ {
			// Diagonal stripes at 45 degrees, alternating every 20 pixels
			isLightStripe := ((x+y)/20)%2 == 0
			var r, g, b byte
			if isLightStripe {
				// Light cyan stripe
				r, g, b = 100, 200, 255
			} else {
				// Dark blue stripe
				r, g, b = 50, 100, 200
			}

			offset := y*rowstride + x*channels
			pixels[offset] = r
			pixels[offset+1] = g
			pixels[offset+2] = b
			pixels[offset+3] = 255
		}
	}

	imageData := ImageDataStruct{
		Width:         width,
		Height:        height,
		Rowstride:     rowstride,
		HasAlpha:      hasAlpha,
		BitsPerSample: bitsPerSample,
		Channels:      channels,
		Data:          pixels,
	}

	hints := map[string]dbus.Variant{
		"image-data": dbus.MakeVariant(imageData),
	}

	notify(conn, "Tall Image Test", "Cropped Image",
		"[TEST] Tall image cropped with fade gradient at bottom.",
		"", hints, 5000)
	time.Sleep(delay)
}

func sendWithProgress(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending progress notification with stack tag updates...")

	// Use stack tag so progress updates replace each other
	steps := []struct {
		percent int32
		body    string
	}{
		{10, "[TEST] ubuntu-24.04.iso - 10%"},
		{35, "[TEST] ubuntu-24.04.iso - 35%"},
		{60, "[TEST] ubuntu-24.04.iso - 60%"},
		{85, "[TEST] ubuntu-24.04.iso - 85%"},
		{100, "[TEST] ubuntu-24.04.iso - Complete!"},
	}

	for _, step := range steps {
		hints := map[string]dbus.Variant{
			"value":             dbus.MakeVariant(step.percent),
			"x-dunst-stack-tag": dbus.MakeVariant("download-progress"),
		}
		notify(conn, "qbittorrent", "Downloading File", step.body, "qbittorrent", hints, 5000)
		fmt.Printf("  -> Progress: %d%%\n", step.percent)
		time.Sleep(600 * time.Millisecond)
	}
	time.Sleep(delay)
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
	time.Sleep(delay)
}

func sendLowUrgency(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending low urgency notification (spotify -> spotify)...")
	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(byte(0)), // Low
	}
	notify(conn, "spotify", "Now Playing",
		"[TEST] Artist Name - Song Title (Discover Weekly)",
		"spotify", hints, 5000)
	time.Sleep(delay)
}

func sendCriticalUrgency(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending critical urgency notification (gufw -> security)...")
	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(byte(2)), // Critical
	}
	notify(conn, "gufw", "Firewall Alert!",
		"[TEST] Blocked incoming connection from suspicious IP.",
		"gufw", hints, 5000)
	time.Sleep(delay)
}

func sendHTMLFormatted(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending HTML formatted notification (telegram-desktop -> telegram)...")
	notify(conn, "telegram-desktop", "Message from Alice",
		"[TEST] <b>Bold</b>, <i>italic</i>, and <u>underlined</u> text.",
		"telegram-desktop", nil, 5000)
	time.Sleep(delay)
}

func sendLongBody(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with long body (thunderbird -> email)...")
	notify(conn, "thunderbird", "New Email from John Doe",
		"[TEST] Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam.",
		"thunderbird", nil, 5000)
	time.Sleep(delay)
}

func sendStack(conn *dbus.Conn, count int) {
	fmt.Printf("[TEST] Sending stack of %d notifications...\n", count)
	for i := 1; i <= count; i++ {
		var hints map[string]dbus.Variant

		// Include a large image on every 3rd notification to test image cropping in stack
		if i%3 == 0 {
			// Create a tall image (400x600) that will be cropped
			width, height := 400, 600
			rowstride := width * 3
			pixels := make([]byte, rowstride*height)

			// Create gradient pattern
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					offset := y*rowstride + x*3
					pixels[offset] = byte((x * 255) / width)       // R: horizontal gradient
					pixels[offset+1] = byte((y * 255) / height)   // G: vertical gradient
					pixels[offset+2] = byte(128)                   // B: constant
				}
			}

			hints = map[string]dbus.Variant{
				"image-data": dbus.MakeVariant(ImageDataStruct{
					Width:         int32(width),
					Height:        int32(height),
					Rowstride:     int32(rowstride),
					HasAlpha:      false,
					BitsPerSample: 8,
					Channels:      3,
					Data:          pixels,
				}),
			}
			notify(conn, "Stack Test", fmt.Sprintf("Notification %d of %d (with image)", i, count),
				"This notification includes a large image that should be cropped.",
				"", hints, 10000)
		} else {
			notify(conn, "Stack Test", fmt.Sprintf("Notification %d of %d", i, count),
				fmt.Sprintf("This is notification number %d in the stack.", i),
				"", nil, 10000)
		}
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(delay)
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
			os.Remove(iconPath)
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
	time.Sleep(delay)
}

func sendWithImagePath(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with image-path hint (file-based image)...")

	// Create a 300x400 test image with a gradient from purple to orange
	width, height := 300, 400
	pixels := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		ratio := float32(y) / float32(height)
		r := byte(128 + int(127*ratio))      // 128 -> 255
		g := byte(50)                        // constant
		b := byte(200 - int(150*ratio))      // 200 -> 50
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 4
			pixels[offset] = r
			pixels[offset+1] = g
			pixels[offset+2] = b
			pixels[offset+3] = 255 // alpha
		}
	}

	// Create temporary PNG file
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("histui-test-%d.png", time.Now().UnixNano()))
	f, err := os.Create(tmpFile)
	if err != nil {
		log.Printf("Failed to create temp file: %v", err)
		return
	}

	// Write raw PNG (simple approach using BMP-like header for testing)
	// Actually we need to use proper PNG encoding
	// For simplicity, let's write a PPM file instead (simpler format) then convert
	// But actually GdkPixbuf can load various formats. Let's write a simple PPM file.
	ppmFile := filepath.Join(os.TempDir(), fmt.Sprintf("histui-test-%d.ppm", time.Now().UnixNano()))
	pf, err := os.Create(ppmFile)
	if err != nil {
		log.Printf("Failed to create PPM file: %v", err)
		f.Close()
		return
	}

	// Write PPM header (P6 format - binary RGB)
	_, _ = fmt.Fprintf(pf, "P6\n%d %d\n255\n", width, height)
	// Write RGB data (skip alpha)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 4
			_, _ = pf.Write([]byte{pixels[offset], pixels[offset+1], pixels[offset+2]})
		}
	}
	pf.Close()
	f.Close()

	hints := map[string]dbus.Variant{
		"image-path": dbus.MakeVariant(ppmFile),
	}

	notify(conn, "Image Path Test", "File-Based Image",
		"This notification uses image-path hint to load from a PPM file.",
		"", hints, 5000)

	// Give notification daemon time to read the file, then clean up
	time.Sleep(delay)
	os.Remove(ppmFile)
	os.Remove(tmpFile)
}

// sendStackTagProgress sends progress updates using stack tag (dunst-compatible).
// This tests the x-dunst-stack-tag hint - notifications with the same tag replace each other.
// No need to track IDs - just use the same tag!
func sendStackTagProgress(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending stack tag progress updates (like dunstify -h string:x-dunst-stack-tag:download)...")

	// Send progress updates with the same stack tag
	steps := []struct {
		percent int32
		body    string
	}{
		{0, "[TEST] large-file.tar.gz - Starting..."},
		{25, "[TEST] large-file.tar.gz - 25%"},
		{50, "[TEST] large-file.tar.gz - 50%"},
		{75, "[TEST] large-file.tar.gz - 75%"},
		{100, "[TEST] large-file.tar.gz - Complete!"},
	}

	for _, step := range steps {
		hints := map[string]dbus.Variant{
			"value":              dbus.MakeVariant(step.percent),
			"x-dunst-stack-tag":  dbus.MakeVariant("download-test"),
		}
		notify(conn, "qbittorrent", "Downloading File", step.body, "qbittorrent", hints, 5000)
		fmt.Printf("  -> Progress: %d%%\n", step.percent)
		time.Sleep(800 * time.Millisecond)
	}
	time.Sleep(delay)
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
	time.Sleep(delay)
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
	time.Sleep(delay)
}

// sendDuplicates sends identical notifications to test duplicate stacking.
// When StackDuplicates is enabled, these should stack with a count badge.
func sendDuplicates(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending duplicate notifications (tests stacking)...")

	// Send 4 identical notifications from the same app
	for i := 1; i <= 4; i++ {
		notify(conn, "discord", "New Message",
			"[TEST] @everyone Someone mentioned you in #general",
			"discord", nil, 8000)
		fmt.Printf("  -> Duplicate %d sent\n", i)
		time.Sleep(200 * time.Millisecond)
	}
	time.Sleep(delay)
}

func runAllTests(conn *dbus.Conn) {
	fmt.Println("[TEST] Starting notification tests...")
	fmt.Println()

	sendSimple(conn)
	sendWithURL(conn)
	sendWithImage(conn)
	sendWithImageData(conn)
	sendWithTallImage(conn)
	sendLowUrgency(conn)
	sendCriticalUrgency(conn)
	sendWithProgress(conn)
	sendLongBody(conn)
	sendHTMLFormatted(conn)
	sendSignalStyle(conn)        // Test View action button
	sendDuplicates(conn)         // Test duplicate stacking
	sendRandomAppSample(conn, 5) // Random sample of apps

	fmt.Println()
	fmt.Println("[OK] All test notifications sent!")
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
	time.Sleep(delay)
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
	time.Sleep(delay)
}
