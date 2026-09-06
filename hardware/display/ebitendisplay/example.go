package ebitendisplay

import (
	"context"
	"fmt"
	"image/color"
	"time"

	"github.com/AlexTransit/vender/hardware/display/framebuffer"
	"github.com/hajimehoshi/ebiten/v2"
)

// Пример использования анимации с framebuffer
func Example() {
	// Открываем framebuffer
	fb, err := framebuffer.New("/dev/fb0")
	if err != nil {
		fmt.Printf("Ошибка открытия framebuffer: %v\n", err)
		return
	}
	defer fb.Close()

	// Создаем дисплей
	display, err := New(fb)
	if err != nil {
		fmt.Printf("Ошибка создания дисплея: %v\n", err)
		return
	}

	// Создаем анимацию прыгающего мяча
	animation := NewBouncingBall(fb.Size().X, fb.Size().Y)
	display.SetAnimation(animation)

	// Запускаем рендеринг
	display.Start()

	// Работаем 10 секунд
	time.Sleep(10 * time.Second)

	// Останавливаем
	display.Stop()
}

// Demo запускает демонстрацию анимации
func Demo(ctx context.Context, dev string) error {
	fb, err := framebuffer.New(dev)
	if err != nil {
		return fmt.Errorf("framebuffer device=%s: %w", dev, err)
	}
	defer fb.Close()

	display, err := New(fb)
	if err != nil {
		return fmt.Errorf("create display: %w", err)
	}

	// Создаем композицию анимаций
	composite := NewCompositeAnimation()
	composite.Add(NewBouncingBall(fb.Size().X, fb.Size().Y))
	composite.Add(NewRotatingSquare(fb.Size().X, fb.Size().Y))
	composite.Add(NewProgressRing(fb.Size().X, fb.Size().Y))

	display.SetAnimation(composite)
	display.SetFPS(30.0)
	display.Start()

	// Ждем контекст
	<-ctx.Done()

	display.Stop()
	return nil
}

// ShowLoading показывает анимацию загрузки
func ShowLoading(ctx context.Context, dev string, duration time.Duration) error {
	fb, err := framebuffer.New(dev)
	if err != nil {
		return fmt.Errorf("framebuffer device=%s: %w", dev, err)
	}
	defer fb.Close()

	display, err := New(fb)
	if err != nil {
		return fmt.Errorf("create display: %w", err)
	}

	animation := NewProgressRing(fb.Size().X, fb.Size().Y)
	display.SetAnimation(animation)
	display.Start()

	select {
	case <-time.After(duration):
	case <-ctx.Done():
	}

	display.Stop()
	return nil
}

// DrawCircleOnScreen рисует круг на экране через ebiten
func DrawCircleOnScreen(dev string, x, y, radius float32, clr color.Color) error {
	fb, err := framebuffer.New(dev)
	if err != nil {
		return fmt.Errorf("framebuffer device=%s: %w", dev, err)
	}
	defer fb.Close()

	display, err := New(fb)
	if err != nil {
		return fmt.Errorf("create display: %w", err)
	}

	return display.RenderOneShot(func(screen *ebiten.Image) {
		display2 := New(int(x)*2, int(y)*2)
		display2.DrawCircle(screen, x, y, radius, clr)
	})
}

// DrawRectangleOnScreen рисует прямоугольник на экране через ebiten
func DrawRectangleOnScreen(dev string, x, y, w, h float32, clr color.Color) error {
	fb, err := framebuffer.New(dev)
	if err != nil {
		return fmt.Errorf("framebuffer device=%s: %w", dev, err)
	}
	defer fb.Close()

	display, err := New(fb)
	if err != nil {
		return fmt.Errorf("create display: %w", err)
	}

	return display.RenderOneShot(func(screen *ebiten.Image) {
		d := New(fb.Size().X, fb.Size().Y)
		d.DrawRectangle(screen, x, y, w, h, clr)
	})
}
