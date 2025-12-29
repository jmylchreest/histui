package tui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync"

	"github.com/blacktop/go-termimg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/jmylchreest/histui/internal/icon"
)

// nerdFontCache caches the loaded NerdFont for reuse.
var (
	nerdFontOnce   sync.Once
	nerdFontParsed *opentype.Font
	nerdFontError  error
)

// loadEmbeddedNerdFont loads the embedded NerdFont from the icon package.
func loadEmbeddedNerdFont() (*opentype.Font, error) {
	nerdFontOnce.Do(func() {
		// Read the embedded font file
		fontData, err := icon.EmbeddedFonts.ReadFile("fonts/SymbolsNerdFont-Regular.ttf")
		if err != nil {
			nerdFontError = err
			return
		}

		nerdFontParsed, nerdFontError = opentype.Parse(fontData)
	})

	return nerdFontParsed, nerdFontError
}

// createFontFace creates a font face at the specified size.
func createFontFace(size float64) (font.Face, error) {
	f, err := loadEmbeddedNerdFont()
	if err != nil {
		return nil, err
	}

	return opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// renderNerdSymbolToImage renders a NerdFont symbol to an image.
// Returns nil if rendering fails.
func renderNerdSymbolToImage(symbol string, size int, fgColor, bgColor color.Color) image.Image {
	if symbol == "" {
		return nil
	}

	// Create font face at appropriate size
	face, err := createFontFace(float64(size))
	if err != nil {
		return nil
	}
	defer func() { _ = face.Close() }()

	// Create image
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Fill background
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, bgColor)
		}
	}

	// Get the first rune from the symbol
	runes := []rune(symbol)
	if len(runes) == 0 {
		return nil
	}
	r := runes[0]

	// Calculate glyph metrics for centering
	advance, ok := face.GlyphAdvance(r)
	if !ok {
		return nil
	}

	// Draw the symbol centered
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(fgColor),
		Face: face,
	}

	// Center horizontally
	advancePixels := advance.Round()
	x := (size - advancePixels) / 2
	if x < 0 {
		x = 0
	}

	// Center vertically (approximate - use size * 0.75 as baseline)
	y := int(float64(size) * 0.75)

	drawer.Dot = fixed.Point26_6{
		X: fixed.I(x),
		Y: fixed.I(y),
	}
	drawer.DrawString(symbol)

	return img
}

// renderNerdSymbolToHalfblocks renders a NerdFont symbol to halfblocks string.
// cols and rows are the terminal character dimensions.
// Returns empty string if rendering fails.
func renderNerdSymbolToHalfblocks(symbol string, cols, rows int) string {
	if symbol == "" {
		return ""
	}

	// Render at higher resolution for better quality
	// Each halfblock cell is 1 char wide and represents 2 vertical pixels
	pixelSize := rows * 4 // 4 pixels per row for better quality

	// Use a blue foreground on dark background (matching the icon style)
	fgColor := color.RGBA{R: 100, G: 150, B: 255, A: 255} // Light blue
	bgColor := color.RGBA{R: 30, G: 30, B: 40, A: 255}    // Dark background

	img := renderNerdSymbolToImage(symbol, pixelSize, fgColor, bgColor)
	if img == nil {
		return ""
	}

	// Encode to PNG for termimg
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}

	// Use termimg to render as halfblocks
	termImg, err := termimg.From(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return ""
	}

	rendered, err := termImg.
		Protocol(termimg.Halfblocks).
		Width(cols * 2). // Double width for halfblocks
		Height(rows * 2).
		Scale(termimg.ScaleFill).
		Render()
	if err != nil {
		return ""
	}

	// Trim and limit to expected rows
	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	if len(lines) > rows {
		lines = lines[:rows]
	}

	return strings.Join(lines, "\n")
}
