package ebitendisplay

import (
	"context"
	"image"
	"image/color"
	"time"

	"github.com/AlexTransit/vender/hardware/display/framebuffer"
	"github.com/hajimehoshi/ebiten/v2"
)

// Display интегрирует ebiten рендеринг с framebuffer
type Display struct {
	fb        *framebuffer.Framebuffer
	width     int
	height    int
	display   *EbitenDisplay
	animation Animation
	ctx       context.Context
	cancel    context.CancelFunc
	fps       float64
}

// New создает новый Display с использованием ebiten для рендеринга
func New(fb *framebuffer.Framebuffer) (*Display, error) {
	size := fb.Size()

	d := &Display{
		fb:      fb,
		width:   size.X,
		height:  size.Y,
		display: New(size.X, size.Y),
		fps:     30.0,
	}

	return d, nil
}

// SetAnimation устанавливает анимацию для отображения
func (d *Display) SetAnimation(animation Animation) {
	d.animation = animation
}

// Start запускает цикл рендеринга анимации
func (d *Display) Start() {
	d.ctx, d.cancel = context.WithCancel(context.Background())
	go d.renderLoop()
}

// Stop останавливает цикл рендеринга
func (d *Display) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
}

// renderLoop основной цикл рендеринга
func (d *Display) renderLoop() {
	ticker := time.NewTicker(time.Duration(float64(time.Second) / d.fps))
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.renderFrame()
		}
	}
}

// renderFrame рендерит один кадр
func (d *Display) renderFrame() {
	// Создаем изображение ebiten
	screen := ebiten.NewImage(d.width, d.height)
	screen.Clear(color.Black)

	// Обновляем и рисуем анимацию
	if d.animation != nil {
		d.animation.Update()
		d.animation.Draw(screen)
	}

	// Получаем пиксели из ebiten
	pixels := make([]color.RGBA, d.width*d.height)
	screen.ReadPixels(pixels)

	// Обновляем framebuffer
	if err := d.fb.Update(pixels); err != nil {
		return
	}

	// Записываем в framebuffer
	if err := d.fb.Flush(); err != nil {
		return
	}
}

// SetFPS устанавливает целевой FPS
func (d *Display) SetFPS(fps float64) {
	d.fps = fps
}

// Clear очищает экран
func (d *Display) Clear() error {
	screen := ebiten.NewImage(d.width, d.height)
	screen.Clear(color.Black)

	pixels := make([]color.RGBA, d.width*d.height)
	screen.ReadPixels(pixels)

	if err := d.fb.Update(pixels); err != nil {
		return err
	}

	return d.fb.Flush()
}

// RenderOneShot рендерит один кадр без запуска цикла анимации
func (d *Display) RenderOneShot(drawFn func(screen *ebiten.Image)) error {
	screen := ebiten.NewImage(d.width, d.height)
	screen.Clear(color.Black)

	if drawFn != nil {
		drawFn(screen)
	}

	pixels := make([]color.RGBA, d.width*d.height)
	screen.ReadPixels(pixels)

	if err := d.fb.Update(pixels); err != nil {
		return err
	}

	return d.fb.Flush()
}

// DrawImage рисует изображение на экране
func (d *Display) DrawImage(img image.Image, x, y int) error {
	return d.RenderOneShot(func(screen *ebiten.Image) {
		ebitenImg := ebiten.NewImageFromImage(img)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x), float64(y))
		screen.DrawImage(ebitenImg, op)
	})
}
