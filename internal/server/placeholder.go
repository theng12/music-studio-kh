package server

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// PNG works in mobile artwork decoders as well as browsers.
var placeholderArtwork = func() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 300, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 300; x++ {
			dx, dy := x-150, y-150
			d := dx*dx + dy*dy
			c := color.RGBA{37, 58, 61, 255}
			if d < 91*91 {
				c = color.RGBA{24, 39, 46, 255}
			}
			if d > 59*59 && d < 62*62 {
				c = color.RGBA{52, 76, 78, 255}
			}
			if d < 33*33 {
				c = color.RGBA{185, 223, 185, 255}
			}
			if d < 6*6 {
				c = color.RGBA{24, 39, 46, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var output bytes.Buffer
	png.Encode(&output, img)
	return output.Bytes()
}()
