package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// MakeIcon generates a 22×22 black template icon (thin ring) for the macOS menubar.
func MakeIcon() []byte {
	const size = 22
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	cx := float64(size)/2 - 0.5
	cy := float64(size)/2 - 0.5
	outer := float64(size)/2 - 1.5
	inner := outer - 3.0
	black := color.NRGBA{0, 0, 0, 255}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			d := math.Sqrt(dx*dx + dy*dy)
			if d <= outer && d >= inner {
				img.Set(x, y, black)
			}
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
