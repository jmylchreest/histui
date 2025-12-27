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
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/godbus/dbus/v5"
)

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
	typeFlag := flag.String("type", "all", "Type of notification to send (all, simple, url, image, imagedata, tallimage, progress, actions, low, critical, html, long, stack)")
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
	case "actions":
		sendWithActions(conn)
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
	case "imagepath":
		sendWithImagePath(conn)
	case "kitty":
		sendKittyStyle(conn)
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
	obj := conn.Object(notifyDest, notifyPath)
	var id uint32
	err := obj.Call(notifyIface+".Notify", 0,
		appName,             // app_name
		uint32(0),           // replaces_id
		icon,                // app_icon
		summary,             // summary
		body,                // body
		[]string{},          // actions
		hints,               // hints
		timeout,             // expire_timeout
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
	fmt.Println("[TEST] Sending simple notification...")
	notify(conn, "Test App", "Simple Notification",
		"This is a basic test notification with just text content.",
		"", nil, 5000)
	time.Sleep(delay)
}

func sendWithURL(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with URL...")
	notify(conn, "Browser", "Link Notification",
		`Check out this link: <a href="https://github.com/jmylchreest/histui">histui on GitHub</a>`,
		"", nil, 5000)
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
		"This notification includes an application icon.",
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
		"This notification has raw pixel data embedded (32x32 red square).",
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

	// Create gradient pixel data (blue at top, green at bottom)
	pixels := make([]byte, height*rowstride)
	for y := int32(0); y < height; y++ {
		// Gradient from blue (top) to green (bottom)
		ratio := float32(y) / float32(height)
		r := byte(50)
		g := byte(100 + int(155*ratio)) // 100 -> 255
		b := byte(255 - int(205*ratio)) // 255 -> 50
		a := byte(255)

		for x := int32(0); x < width; x++ {
			offset := y*rowstride + x*channels
			pixels[offset] = r
			pixels[offset+1] = g
			pixels[offset+2] = b
			pixels[offset+3] = a
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
		"This tall image should be cropped with a fade gradient at the bottom.",
		"", hints, 5000)
	time.Sleep(delay)
}

func sendWithProgress(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with progress...")
	hints := map[string]dbus.Variant{
		"value": dbus.MakeVariant(int32(75)),
	}
	notify(conn, "Download Manager", "Downloading File",
		"file.zip - 75% complete",
		"", hints, 5000)
	time.Sleep(delay)
}

func sendWithActions(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with actions...")
	actions := []string{
		"prev", "Previous",
		"play", "Play/Pause",
		"next", "Next",
	}
	notifyWithActions(conn, "Music Player", "Now Playing",
		"Artist - Song Title",
		"", actions, nil, 5000)
	time.Sleep(delay)
}

func sendLowUrgency(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending low urgency notification...")
	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(byte(0)), // Low
	}
	notify(conn, "System", "Low Priority",
		"This is a low urgency notification that can be easily dismissed.",
		"", hints, 5000)
	time.Sleep(delay)
}

func sendCriticalUrgency(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending critical urgency notification...")
	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(byte(2)), // Critical
	}
	notify(conn, "System Monitor", "Critical Alert!",
		"System temperature is dangerously high!",
		"", hints, 5000)
	time.Sleep(delay)
}

func sendHTMLFormatted(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending HTML formatted notification...")
	notify(conn, "Messenger", "Message from Alice",
		"<b>Bold text</b>, <i>italic text</i>, and <u>underlined text</u>.",
		"", nil, 5000)
	time.Sleep(delay)
}

func sendLongBody(conn *dbus.Conn) {
	fmt.Println("[TEST] Sending notification with long body...")
	notify(conn, "Email", "New Email from John Doe",
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris.",
		"", nil, 5000)
	time.Sleep(delay)
}

func sendStack(conn *dbus.Conn, count int) {
	fmt.Printf("[TEST] Sending stack of %d notifications...\n", count)
	for i := 1; i <= count; i++ {
		notify(conn, "Stack Test", fmt.Sprintf("Notification %d of %d", i, count),
			fmt.Sprintf("This is notification number %d in the stack.", i),
			"", nil, 5000)
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
	// icon (app_icon): "/usr/lib/kitty/logo/kitty.png"
	// actions: [{"key": "default", "label": " "}]

	actions := []string{
		"default", " ", // Kitty uses a single space as the label
	}

	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(byte(1)), // Normal urgency
	}

	notifyWithActions(conn, "kitty", "Claude Code",
		"Claude needs your permission to use Bash",
		"/usr/lib/kitty/logo/kitty.png", // app_icon
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

	// Clean up temp files after a delay
	go func() {
		time.Sleep(10 * time.Second)
		os.Remove(ppmFile)
		os.Remove(tmpFile)
	}()

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

	fmt.Println()
	fmt.Println("[OK] All test notifications sent!")
}
