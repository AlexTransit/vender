package ebitendisplay

import (
	"context"
	"image/color"
	"time"

	"github.com/AlexTransit/vender/hardware/display/framebuffer"
	"github.com/hajimehoshi/ebiten/v2"
)

// FramebufferRenderer рендерит ebiten изображение в framebuffer
type FramebufferRenderer struct {
	fb        *framebuffer.Framebuffer
	width     int
	height    int
	display   *EbitenDisplay
	animation Animation
	ctx       context.Context
	cancel    context.CancelFunc
	fps       float64
}

// NewFramebufferRenderer создает новый рендерер для framebuffer
func NewFramebufferRenderer(fb *framebuffer.Framebuffer, animation Animation) (*FramebufferRenderer, error) {
	width := fb.Size().X
	height := fb.Size().Y

	display := New(width, height)

	ctx, cancel := context.WithCancel(context.Background())

	r := &FramebufferRenderer{
		fb:        fb,
		width:     width,
		height:    height,
		display:   display,
		animation: animation,
		ctx:       ctx,
		cancel:    cancel,
		fps:       30.0, // Целевой FPS для анимации
	}

	return r, nil
}

// Start запускает цикл рендеринга
func (r *FramebufferRenderer) Start() {
	go r.renderLoop()
}

// Stop останавливает цикл рендеринга
func (r *FramebufferRenderer) Stop() {
	r.cancel()
}

// renderLoop основной цикл рендеринга
func (r *FramebufferRenderer) renderLoop() {
	ticker := time.NewTicker(time.Duration(float64(time.Second) / r.fps))
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.renderFrame()
		}
	}
}

// renderFrame рендерит один кадр
func (r *FramebufferRenderer) renderFrame() {
	// Обновляем анимацию
	if r.animation != nil {
		r.animation.Update()
	}

	// Создаем изображение ebiten
	screen := ebiten.NewImage(r.width, r.height)
	screen.Clear(color.Black)

	// Рисуем анимацию
	if r.animation != nil {
		r.animation.Draw(screen)
	}

	// Получаем пиксели из ebiten
	pixels := make([]color.RGBA, r.width*r.height)
	screen.ReadPixels(pixels)

	// Обновляем framebuffer
	if err := r.fb.Update(pixels); err != nil {
		// Логирование ошибки
		return
	}

	// Записываем в framebuffer
	if err := r.fb.Flush(); err != nil {
		// Логирование ошибки
		return
	}
}

// SetAnimation устанавливает новую анимацию
func (r *FramebufferRenderer) SetAnimation(animation Animation) {
	r.animation = animation
}

// SetFPS устанавливает целевой FPS
func (r *FramebufferRenderer) SetFPS(fps float64) {
	r.fps = fps
}

// RunBlocking запускает цикл рендеринга в блокирующем режиме
func (r *FramebufferRenderer) RunBlocking() {
	ticker := time.NewTicker(time.Duration(float64(time.Second) / r.fps))
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.renderFrame()
		}
	}
}
