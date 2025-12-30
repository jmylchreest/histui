// Package tui provides image conversion utilities for terminal rendering.
package tui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// isSVGPath checks if a file path points to an SVG file.
func isSVGPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".svg"
}

// isSVGData checks if byte data appears to be SVG content.
// Checks for XML declaration or svg element at the start.
func isSVGData(data []byte) bool {
	if len(data) < 10 {
		return false
	}
	// Skip any leading whitespace/BOM
	content := strings.TrimSpace(string(data[:min(len(data), 500)]))
	content = strings.ToLower(content)
	return strings.HasPrefix(content, "<?xml") ||
		strings.HasPrefix(content, "<svg") ||
		strings.Contains(content[:min(len(content), 100)], "<svg")
}

// RasterizeSVG converts SVG data to a PNG image.
// The size parameter sets both width and height (square output).
// fgColor is used to replace "currentColor" in the SVG.
// Returns PNG-encoded data or an error.
func RasterizeSVG(svgData []byte, size int, fgColor color.Color) ([]byte, error) {
	return RasterizeSVGWithBg(svgData, size, fgColor, color.Transparent)
}

// RasterizeSVGWithBg converts SVG data to a PNG image with a background color.
func RasterizeSVGWithBg(svgData []byte, size int, fgColor, bgColor color.Color) ([]byte, error) {
	// Replace "currentColor" with the actual color
	svgStr := string(svgData)
	if fgColor != nil {
		r, g, b, _ := fgColor.RGBA()
		hexColor := "#" + rgbToHex(uint8(r>>8), uint8(g>>8), uint8(b>>8))
		svgStr = strings.ReplaceAll(svgStr, "currentColor", hexColor)
		svgStr = strings.ReplaceAll(svgStr, "currentcolor", hexColor)
	}

	// Parse SVG
	icon, err := oksvg.ReadIconStream(strings.NewReader(svgStr))
	if err != nil {
		return nil, err
	}

	// Set target size
	w, h := float64(size), float64(size)
	icon.SetTarget(0, 0, w, h)

	// Create RGBA image
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Fill background if not transparent
	if bgColor != nil {
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				img.Set(x, y, bgColor)
			}
		}
	}

	// Create scanner and draw
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	raster := rasterx.NewDasher(size, size, scanner)
	icon.Draw(raster, 1.0)

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// RasterizeSVGFile reads and rasterizes an SVG file.
func RasterizeSVGFile(path string, size int, fgColor color.Color) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return RasterizeSVG(data, size, fgColor)
}

// ConvertToRaster converts any supported image format to raster PNG data.
// Handles SVG and NerdFont symbols; passes through raster formats unchanged.
// For raster formats (PNG, JPEG, etc.), returns nil to indicate no conversion needed.
func ConvertToRaster(path string, data []byte, size int, fgColor color.Color) ([]byte, bool) {
	// Check if it's SVG
	if path != "" && isSVGPath(path) {
		converted, err := RasterizeSVGFile(path, size, fgColor)
		if err == nil {
			return converted, true
		}
		return nil, false
	}

	if len(data) > 0 && isSVGData(data) {
		converted, err := RasterizeSVG(data, size, fgColor)
		if err == nil {
			return converted, true
		}
		return nil, false
	}

	// Not SVG - no conversion needed
	return nil, false
}

// ConvertToRasterReader is like ConvertToRaster but returns an io.Reader.
// Returns nil if no conversion was needed (pass original to termimg).
func ConvertToRasterReader(path string, data []byte, size int, fgColor color.Color) io.Reader {
	converted, ok := ConvertToRaster(path, data, size, fgColor)
	if !ok || converted == nil {
		return nil
	}
	return bytes.NewReader(converted)
}

// rgbToHex converts RGB values to a hex string (without #).
func rgbToHex(r, g, b uint8) string {
	return string([]byte{
		hexDigit(r >> 4), hexDigit(r & 0xf),
		hexDigit(g >> 4), hexDigit(g & 0xf),
		hexDigit(b >> 4), hexDigit(b & 0xf),
	})
}

func hexDigit(n uint8) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + n - 10
}

// DefaultIconColor returns a default foreground color for icon rendering.
// This is a light blue-gray that works well on most backgrounds.
func DefaultIconColor() color.Color {
	return color.RGBA{R: 180, G: 200, B: 220, A: 255}
}
