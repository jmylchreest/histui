package dbus

import (
	"fmt"
	"testing"
)

func BenchmarkImageDataToPNG(b *testing.B) {
	for _, size := range []int{64, 256, 1024} {
		for _, channels := range []int{3, 4} {
			b.Run(fmt.Sprintf("%dx%d/%dch", size, size, channels), func(b *testing.B) {
				data := make([]byte, size*size*channels)
				for i := range data {
					data[i] = byte(i*31 + i/256)
				}
				img := &ImageDataStruct{Width: int32(size), Height: int32(size), Rowstride: int32(size * channels), HasAlpha: channels == 4, BitsPerSample: 8, Channels: int32(channels), Data: data}
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if len(img.ToPNG()) == 0 {
						b.Fatal("empty PNG")
					}
				}
			})
		}
	}
}
