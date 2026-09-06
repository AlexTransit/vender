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

// NewDisplay создает новый Display с использованием ebiten для рендеринга
func NewDisplay(fb *framebuffer.Framebuffer) (*Display, error) {
	size := fb.Size()

	d := &Display{
		fb:      fb,
		width:   size.X,
		height:  size.Y,
		display: NewEbitenDisplay(size.X, size.Y),
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
	screen.Clear()

	// Обновляем и рисуем анимацию
	if d.animation != nil {
		d.animation.Update()
		d.animation.Draw(screen)
	}

	// Получаем пиксели через At() и конвертируем в color.RGBA
	cs := make([]color.RGBA, d.width*d.height)
	for y := 0; y < d.height; y++ {
		for x := 0; x < d.width; x++ {
			c := screen.At(x, y)
			rr, gg, bb, aa := c.RGBA()
			idx := y*d.width + x
			cs[idx] = color.RGBA{
				R: uint8(rr >> 8),
				G: uint8(gg >> 8),
				B: uint8(bb >> 8),
				A: uint8(aa >> 8),
			}
		}
	}

	// Обновляем framebuffer
	if err := d.fb.Update(cs); err != nil {
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
	screen.Clear()

	cs := make([]color.RGBA, d.width*d.height)
	for y := 0; y < d.height; y++ {
		for x := 0; x < d.width; x++ {
			c := screen.At(x, y)
			rr, gg, bb, aa := c.RGBA()
			idx := y*d.width + x
			cs[idx] = color.RGBA{
				R: uint8(rr >> 8),
				G: uint8(gg >> 8),
				B: uint8(bb >> 8),
				A: uint8(aa >> 8),
			}
		}
	}

	if err := d.fb.Update(cs); err != nil {
		return err
	}

	return d.fb.Flush()
}

// RenderOneShot рендерит один кадр без запуска цикла анимации
func (d *Display) RenderOneShot(drawFn func(screen *ebiten.Image)) error {
	screen := ebiten.NewImage(d.width, d.height)
	screen.Clear()

	if drawFn != nil {
		drawFn(screen)
	}

	cs := make([]color.RGBA, d.width*d.height)
	for y := 0; y < d.height; y++ {
		for x := 0; x < d.width; x++ {
			c := screen.At(x, y)
			rr, gg, bb, aa := c.RGBA()
			idx := y*d.width + x
			cs[idx] = color.RGBA{
				R: uint8(rr >> 8),
				G: uint8(gg >> 8),
				B: uint8(bb >> 8),
				A: uint8(aa >> 8),
			}
		}
	}

	if err := d.fb.Update(cs); err != nil {
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
