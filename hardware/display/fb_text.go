package display

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/fogleman/gg"
)

func (d *Display) DrawText(text string, size float64, x float64, y float64) error {
	dc := gg.NewContext(d.size.X, d.size.Y)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)
	dc.LoadFontFace("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", size)
	dc.DrawString(text, x, y)
	// dc.DrawStringAnchored(text, x, y, 0.5, 0.5)
	img := dc.Image() // *image.RGBA
	d.rgba(img.(*image.RGBA))
	// draw.Draw(d.buf, d.buf.Bounds(), img, image.Point{}, draw.Src)

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

// OverlayText накладывает текст поверх текущего содержимого буфера
// text - текст для отображения
// size - размер шрифта
// x, y - координаты начала текста
// fgColor - цвет текста (например, color.RGBA{255, 255, 255, 255} для белого)
// bgColor - цвет фона под текстом (например, color.RGBA{0, 0, 0, 128} для полупрозрачного чёрного)
// Если bgColor.A == 0, фон не рисуется
func (d *Display) OverlayText(text string, size float64, x float64, y float64, fgColor, bgColor color.Color) error {
	dc := gg.NewContext(d.size.X, d.size.Y)

	// Рисуем текущее содержимое буфера в контекст
	draw.Draw(dc.Image().(*image.RGBA), d.buf.Bounds(), d.buf, image.Point{}, draw.Src)

	// Рисуем фон под текстом если указан
	if bgColor != nil {
		width, height := dc.MeasureString(text)
		padding := size * 0.2
		dc.SetColor(bgColor)
		dc.DrawRectangle(x-padding, y-size+padding, width+2*padding, height+2*padding)
		dc.Fill()
	}

	// Рисуем текст
	dc.SetColor(fgColor)
	if err := dc.LoadFontFace("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", size); err != nil {
		return err
	}
	dc.DrawString(text, x, y)

	// Копируем результат обратно в буфер дисплея
	draw.Draw(d.buf, d.buf.Bounds(), dc.Image().(*image.RGBA), image.Point{}, draw.Src)

	return d.Flush()
}

// OverlayTextSimple накладывает белый текст с полупрозрачным чёрным фоном
// Удобная версия OverlayText с предустановленными цветами
func (d *Display) OverlayTextSimple(text string, size float64, x float64, y float64) error {
	fgColor := color.RGBA{255, 255, 255, 255}
	bgColor := color.RGBA{0, 0, 0, 180}
	return d.OverlayText(text, size, x, y, fgColor, bgColor)
}
