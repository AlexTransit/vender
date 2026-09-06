package ebitendisplay

import (
	"image"
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

// EbitenDisplay предоставляет рендеринг через ebiten с выводом в framebuffer
type EbitenDisplay struct {
	screenWidth  int
	screenHeight int
	buffer       *image.RGBA
	mu           sync.Mutex
	drawFunc     func(screen *ebiten.Image)
	currentFrame int
}

// New создает новый EbitenDisplay
func New(width, height int) *EbitenDisplay {
	return &EbitenDisplay{
		screenWidth:  width,
		screenHeight: height,
		buffer:       image.NewRGBA(image.Rect(0, 0, width, height)),
	}
}

// SetDrawFunc устанавливает функцию отрисовки
func (d *EbitenDisplay) SetDrawFunc(fn func(screen *ebiten.Image)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.drawFunc = fn
}

// Update вызывается ebiten для обновления логики
func (d *EbitenDisplay) Update() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.currentFrame++
	return nil
}

// Draw вызывается ebiten для рендеринга кадра
func (d *EbitenDisplay) Draw(screen *ebiten.Image) {
	d.mu.Lock()
	fn := d.drawFunc
	d.mu.Unlock()

	if fn != nil {
		fn(screen)
	}
}

// Layout возвращает размеры экрана
func (d *EbitenDisplay) Layout(outsideWidth, outsideHeight int) (int, int) {
	return d.screenWidth, d.screenHeight
}

// RenderToBuffer рендерит текущий кадр в буфер
func (d *EbitenDisplay) RenderToBuffer() []color.RGBA {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Создаем временное изображение ebiten
	tmpImage := ebiten.NewImage(d.screenWidth, d.screenHeight)

	// Вызываем функцию отрисовки
	if d.drawFunc != nil {
		d.drawFunc(tmpImage)
	}

	// Читаем пиксели в буфер
	pixels := make([]color.RGBA, d.screenWidth*d.screenHeight)
	tmpImage.ReadPixels(pixels)

	return pixels
}

// GetFrame возвращает номер текущего кадра
func (d *EbitenDisplay) GetFrame() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.currentFrame
}

// Clear очищает экран
func (d *EbitenDisplay) Clear(screen *ebiten.Image) {
	screen.Clear(color.Black)
}

// DrawCircle рисует круг
func (d *EbitenDisplay) DrawCircle(screen *ebiten.Image, x, y float32, radius float32, clr color.Color) {
	// Простая реализация рисования круга через пиксели
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dy*dy <= radius*radius {
				px := int(x + dx)
				py := int(y + dy)
				if px >= 0 && px < d.screenWidth && py >= 0 && py < d.screenHeight {
					screen.Set(px, py, clr)
				}
			}
		}
	}
}

// DrawRectangle рисует прямоугольник
func (d *EbitenDisplay) DrawRectangle(screen *ebiten.Image, x, y, w, h float32, clr color.Color) {
	for iy := int(y); iy < int(y+h); iy++ {
		for ix := int(x); ix < int(x+w); ix++ {
			if ix >= 0 && ix < d.screenWidth && iy >= 0 && iy < d.screenHeight {
				screen.Set(ix, iy, clr)
			}
		}
	}
}

// DrawLine рисует линию (алгоритм Брезенхема)
func (d *EbitenDisplay) DrawLine(screen *ebiten.Image, x0, y0, x1, y1 float32, width float32, clr color.Color) {
	// Упрощенная реализация линии
	dx := x1 - x0
	dy := y1 - y0
	steps := maxInt(absInt(int(dx)), absInt(int(dy)))
	if steps == 0 {
		steps = 1
	}
	xInc := dx / float32(steps)
	yInc := dy / float32(steps)
	x := x0
	y := y0
	for i := 0; i <= steps; i++ {
		if int(x) >= 0 && int(x) < d.screenWidth && int(y) >= 0 && int(y) < d.screenHeight {
			screen.Set(int(x), int(y), clr)
		}
		x += xInc
		y += yInc
	}
}

// DrawText рисует текст (требует загрузки шрифта)
func (d *EbitenDisplay) DrawText(screen *ebiten.Image, text string, x, y int, clr color.Color) {
	// Здесь можно добавить поддержку текста через ebiten/text
	_ = screen
	_ = text
	_ = x
	_ = y
	_ = clr
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
