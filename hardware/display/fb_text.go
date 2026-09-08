package display

import (
	"image"
	"image/color"

	"github.com/fogleman/gg"
)

func (d *Display) DrawText(text string, size float64, x float64, y float64) error {
	dc := gg.NewContext(240, 320)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)
	dc.LoadFontFace("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", size)
	dc.DrawString(text, x, y)
	// dc.DrawStringAnchored(text, x, y, 0.5, 0.5)
	img := dc.Image() // *image.RGBA
	d.rgba(img.(*image.RGBA))

	return d.Flush()
}

func (d *Display) rgba(img *image.RGBA) {
	min, max := img.Bounds().Min, img.Bounds().Max

	for y := min.Y; y < max.Y; y++ {
		for x := min.X; x < max.X; x++ {
			i := img.PixOffset(x, y)

			d.set(x, y, color.RGBA{
				R: img.Pix[i],
				G: img.Pix[i+1],
				B: img.Pix[i+2],
				A: img.Pix[i+3],
			})
		}
	}
}
