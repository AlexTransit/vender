package ebitendisplay

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// BouncingBall анимация прыгающего мяча
type BouncingBall struct {
	x, y         float32
	vx, vy       float32
	radius       float32
	screenWidth  float32
	screenHeight float32
	color        color.Color
}

// NewBouncingBall создает новую анимацию прыгающего мяча
func NewBouncingBall(screenWidth, screenHeight int) *BouncingBall {
	return &BouncingBall{
		x:            float32(screenWidth) / 2,
		y:            float32(screenHeight) / 2,
		vx:           2.0,
		vy:           2.0,
		radius:       15.0,
		screenWidth:  float32(screenWidth),
		screenHeight: float32(screenHeight),
		color:        color.RGBA{255, 100, 100, 255},
	}
}

// Update обновляет позицию мяча
func (b *BouncingBall) Update() {
	b.x += b.vx
	b.y += b.vy

	// Отскок от границ
	if b.x-b.radius < 0 || b.x+b.radius > b.screenWidth {
		b.vx = -b.vx
	}
	if b.y-b.radius < 0 || b.y+b.radius > b.screenHeight {
		b.vy = -b.vy
	}
}

// Draw рисует мяч на экране
func (b *BouncingBall) Draw(screen *ebiten.Image) {
	d := &EbitenDisplay{}
	d.DrawCircle(screen, b.x, b.y, b.radius, b.color)
}

// RotatingSquare анимация вращающегося квадрата
type RotatingSquare struct {
	angle        float32
	size         float32
	x, y         float32
	screenWidth  float32
	screenHeight float32
	color        color.Color
}

// NewRotatingSquare создает новую анимацию вращающегося квадрата
func NewRotatingSquare(screenWidth, screenHeight int) *RotatingSquare {
	return &RotatingSquare{
		angle:        0,
		size:         40,
		x:            float32(screenWidth) / 2,
		y:            float32(screenHeight) / 2,
		screenWidth:  float32(screenWidth),
		screenHeight: float32(screenHeight),
		color:        color.RGBA{100, 255, 100, 255},
	}
}

// Update обновляет угол вращения
func (r *RotatingSquare) Update() {
	r.angle += 0.05
	if r.angle > 2*math.Pi {
		r.angle -= 2 * math.Pi
	}
}

// Draw рисует вращающийся квадрат
func (r *RotatingSquare) Draw(screen *ebiten.Image) {
	// Для вращения нужно использовать более сложную отрисовку
	// Здесь упрощенная версия - просто рисуем квадрат в центре
	d := &EbitenDisplay{}
	halfSize := r.size / 2
	d.DrawRectangle(screen, r.x-halfSize, r.y-halfSize, r.size, r.size, r.color)
}

// ProgressRing анимация прогресс-бара в виде кольца
type ProgressRing struct {
	progress     float32 // 0.0 - 1.0
	x, y         float32
	radius       float32
	thickness    float32
	screenWidth  float32
	screenHeight float32
	color        color.Color
}

// NewProgressRing создает новое кольцо прогресса
func NewProgressRing(screenWidth, screenHeight int) *ProgressRing {
	return &ProgressRing{
		progress:     0,
		x:            float32(screenWidth) / 2,
		y:            float32(screenHeight) / 2,
		radius:       50,
		thickness:    8,
		screenWidth:  float32(screenWidth),
		screenHeight: float32(screenHeight),
		color:        color.RGBA{100, 100, 255, 255},
	}
}

// Update обновляет прогресс
func (p *ProgressRing) Update() {
	p.progress += 0.01
	if p.progress > 1.0 {
		p.progress = 0
	}
}

// Draw рисует кольцо прогресса
func (p *ProgressRing) Draw(screen *ebiten.Image) {
	d := &EbitenDisplay{}
	// Рисуем фоновое кольцо
	d.DrawCircle(screen, p.x, p.y, p.radius, color.RGBA{50, 50, 50, 255})
	// Рисуем прогресс (упрощенно - просто круг)
	d.DrawCircle(screen, p.x, p.y, p.radius*p.progress, p.color)
}

// Animation интерфейс для всех анимаций
type Animation interface {
	Update()
	Draw(screen *ebiten.Image)
}

// CompositeAnimation композиция нескольких анимаций
type CompositeAnimation struct {
	animations []Animation
}

// NewCompositeAnimation создает композицию анимаций
func NewCompositeAnimation() *CompositeAnimation {
	return &CompositeAnimation{
		animations: make([]Animation, 0),
	}
}

// Add добавляет анимацию в композицию
func (c *CompositeAnimation) Add(a Animation) {
	c.animations = append(c.animations, a)
}

// Update обновляет все анимации
func (c *CompositeAnimation) Update() {
	for _, a := range c.animations {
		a.Update()
	}
}

// Draw рисует все анимации
func (c *CompositeAnimation) Draw(screen *ebiten.Image) {
	for _, a := range c.animations {
		a.Draw(screen)
	}
}
