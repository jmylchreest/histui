package dbus

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestImageDataPNGPreservesPixelsAndInput(t *testing.T) {
	for _, tc := range []struct {
		name              string
		channels, padding int
		alpha             bool
	}{
		{"rgba", 4, 0, true}, {"padded-rgba", 4, 11, true}, {"rgb", 3, 0, false}, {"padded-rgb", 3, 7, false}, {"ignored-alpha", 4, 5, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const width, height = 7, 5
			stride := width*tc.channels + tc.padding
			// Final-row padding is optional in the notification protocol.
			data := make([]byte, (height-1)*stride+width*tc.channels)
			for i := range data {
				data[i] = byte(i * 17)
			}
			original := bytes.Clone(data)
			expected := image.NewRGBA(image.Rect(0, 0, width, height))
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					i := y*stride + x*tc.channels
					a := uint8(255)
					if tc.alpha {
						a = data[i+3]
					}
					expected.SetRGBA(x, y, color.RGBA{R: data[i], G: data[i+1], B: data[i+2], A: a})
				}
			}
			var want bytes.Buffer
			if err := png.Encode(&want, expected); err != nil {
				t.Fatal(err)
			}
			img := &ImageDataStruct{Width: width, Height: height, Rowstride: int32(stride), Channels: int32(tc.channels), HasAlpha: tc.alpha, BitsPerSample: 8, Data: data}
			for range 3 {
				got := img.ToPNG()
				if !bytes.Equal(got, want.Bytes()) {
					t.Fatal("PNG differs from historical RGBA conversion")
				}
				if !bytes.Equal(data, original) {
					t.Fatal("conversion modified input pixels")
				}
			}
		})
	}
}

func TestImageDataPNGParallel(t *testing.T) {
	for range 12 {
		t.Run("encode", func(t *testing.T) {
			t.Parallel()
			img := &ImageDataStruct{Width: 1, Height: 1, Rowstride: 4, Channels: 4, HasAlpha: true, BitsPerSample: 8, Data: []byte{64, 32, 16, 128}}
			for range 20 {
				if _, err := png.Decode(bytes.NewReader(img.ToPNG())); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
