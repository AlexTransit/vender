package display

import (
	"image"

	"github.com/fogleman/gg"
)

func (d *Display) DrawText(text string) error {
	dc := gg.NewContext(240, 320)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)
	dc.LoadFontFace("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 24)
	dc.DrawString("Hello", 10, 50)
	img := dc.Image() // *image.RGBA
	d.palleted2(img.(*image.Paletted))
	return d.Flush()
}
