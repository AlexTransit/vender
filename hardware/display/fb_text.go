package display

import (
	"image"
	"image/color"
	"image/draw"
	"os"
	"strings"
	"sync"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

const fontPath = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"

var (
	fontOnce sync.Once
	fontTTF  *truetype.Font
	fontErr  error

	faceMu    sync.Mutex
	faceCache = map[float64]font.Face{}
)

func loadFontFace(size float64) (font.Face, error) {
	fontOnce.Do(func() {
		data, err := os.ReadFile(fontPath)
		if err != nil {
			fontErr = err
			return
		}
		fontTTF, fontErr = truetype.Parse(data)
	})
	if fontErr != nil {
		return nil, fontErr
	}

	faceMu.Lock()
	defer faceMu.Unlock()
	if f, ok := faceCache[size]; ok {
		return f, nil
	}
	f := truetype.NewFace(fontTTF, &truetype.Options{Size: size})
	faceCache[size] = f
	return f, nil
}

func (d *Display) DrawText(text string, size float64, x float64, y float64) error {
	dc := gg.NewContext(d.size.X, d.size.Y)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)

	face, err := loadFontFace(size)
	if err != nil {
		return err
	}
	dc.SetFontFace(face)
	dc.DrawString(text, x, y)

	img := dc.Image().(*image.RGBA)
	d.rgba(img)

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

// OverlayText накладывает однострочный текст поверх буфера.
// Если bgColor == nil или его альфа-канал == 0, фон не рисуется.
func (d *Display) OverlayText(text string, size float64, x float64, y float64, fgColor, bgColor color.Color) error {
	tmp := image.NewRGBA(image.Rect(0, 0, d.size.X, d.size.Y))
	draw.Draw(tmp, d.buf.Bounds(), d.buf, image.Point{}, draw.Src)

	dc := gg.NewContext(d.size.X, d.size.Y)
	dc.DrawImage(tmp, 0, 0)

	face, err := loadFontFace(size)
	if err != nil {
		return err
	}
	dc.SetFontFace(face)

	if isOpaque(bgColor) {
		width, height := dc.MeasureString(text)
		padding := size * 0.2
		dc.SetColor(bgColor)
		dc.DrawRectangle(x-padding, y-height+padding, width+2*padding, height+2*padding)
		dc.Fill()
	}

	dc.SetColor(fgColor)
	dc.DrawString(text, x, y)

	draw.Draw(d.buf, d.buf.Bounds(), dc.Image(), image.Point{}, draw.Src)
	return d.Flush()
}

func (d *Display) OverlayTextSimple(text string, size float64, x float64, y float64) error {
	fgColor := color.RGBA{255, 255, 255, 255}
	bgColor := color.RGBA{0, 0, 0, 180}
	return d.OverlayText(text, size, x, y, fgColor, bgColor)
}

// OverlayTextWrapped накладывает многострочный текст с переносом по словам.
// maxWidth <= 0 -> использовать ширину от x до правого края дисплея.
// Если слово шире maxWidth, оно разбивается посимвольно.
func (d *Display) OverlayTextWrapped(text string, size, x, y, maxWidth float64, fgColor, bgColor color.Color) error {
	tmp := image.NewRGBA(image.Rect(0, 0, d.size.X, d.size.Y))
	draw.Draw(tmp, d.buf.Bounds(), d.buf, image.Point{}, draw.Src)

	dc := gg.NewContext(d.size.X, d.size.Y)
	dc.DrawImage(tmp, 0, 0)

	face, err := loadFontFace(size)
	if err != nil {
		return err
	}
	dc.SetFontFace(face)

	if maxWidth <= 0 {
		maxWidth = float64(d.size.X) - x
	}
	if maxWidth <= 0 {
		return nil
	}

	lines := wrapWords(dc, text, maxWidth)
	if len(lines) == 0 {
		return nil
	}

	lineHeight := dc.FontHeight() * 1.2

	if isOpaque(bgColor) {
		var blockWidth float64
		for _, l := range lines {
			w, _ := dc.MeasureString(l)
			if w > blockWidth {
				blockWidth = w
			}
		}
		padding := size * 0.2
		blockHeight := lineHeight*float64(len(lines)) - (lineHeight - dc.FontHeight())
		dc.SetColor(bgColor)
		dc.DrawRectangle(x-padding, y-dc.FontHeight()+padding, blockWidth+2*padding, blockHeight+2*padding)
		dc.Fill()
	}

	dc.SetColor(fgColor)
	curY := y
	for _, l := range lines {
		dc.DrawString(l, x, curY)
		curY += lineHeight
	}

	draw.Draw(d.buf, d.buf.Bounds(), dc.Image(), image.Point{}, draw.Src)
	return d.Flush()
}

func (d *Display) OverlayTextWrappedSimple(text string, size, x, y, maxWidth float64) error {
	fgColor := color.RGBA{255, 255, 255, 255}
	bgColor := color.RGBA{0, 0, 0, 180}
	return d.OverlayTextWrapped(text, size, x, y, maxWidth, fgColor, bgColor)
}

func isOpaque(c color.Color) bool {
	if c == nil {
		return false
	}
	_, _, _, a := c.RGBA()
	return a != 0
}

// wrapWords разбивает text на строки шириной не более maxWidth.
// Символ '\n' трактуется как явный перенос строки (в т.ч. пустые строки).
// Если отдельное слово не помещается целиком, оно режется посимвольно.
func wrapWords(dc *gg.Context, text string, maxWidth float64) []string {
	var lines []string

	paragraphs := strings.Split(text, "\n")
	for _, p := range paragraphs {
		if strings.TrimSpace(p) == "" {
			lines = append(lines, "") // сохраняем пустую строку от \n\n
			continue
		}
		lines = append(lines, wrapParagraph(dc, p, maxWidth)...)
	}

	return lines
}

// wrapParagraph переносит один параграф (без \n внутри) по словам,
// с посимвольным разрывом слов, которые не влезают целиком.
func wrapParagraph(dc *gg.Context, text string, maxWidth float64) []string {
	words := strings.Fields(text)
	var lines []string
	var cur string

	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
	}

	for _, w := range words {
		candidate := w
		if cur != "" {
			candidate = cur + " " + w
		}
		if cw, _ := dc.MeasureString(candidate); cw <= maxWidth {
			cur = candidate
			continue
		}

		flush()

		if ww, _ := dc.MeasureString(w); ww <= maxWidth {
			cur = w
			continue
		}

		chunks := splitWordByWidth(dc, w, maxWidth)
		for i, c := range chunks {
			if i == len(chunks)-1 {
				cur = c
			} else {
				lines = append(lines, c)
			}
		}
	}
	flush()

	return lines
}

// splitWordByWidth режет слово на куски, каждый из которых не шире maxWidth.
func splitWordByWidth(dc *gg.Context, word string, maxWidth float64) []string {
	runes := []rune(word)
	var chunks []string
	start := 0

	for start < len(runes) {
		end := start
		for end < len(runes) {
			w, _ := dc.MeasureString(string(runes[start : end+1]))
			if w > maxWidth {
				break
			}
			end++
		}
		if end == start {
			// даже один символ шире maxWidth - берём его принудительно,
			// иначе зациклимся
			end = start + 1
		}
		chunks = append(chunks, string(runes[start:end]))
		start = end
	}

	return chunks
}
