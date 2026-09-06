package main

import (
	"time"

	"github.com/AlexTransit/vender/hardware/display/ebitendisplay"
	"github.com/AlexTransit/vender/hardware/display/framebuffer"
)

func main() {
	// Открываем framebuffer
	fb, err := framebuffer.New("/dev/fb0")
	if err != nil {
		panic(err)
	}
	defer fb.Close()

	// Создаем дисплей
	display, err := ebitendisplay.New(fb)
	if err != nil {
		panic(err)
	}

	// Создаем анимацию
	animation := ebitendisplay.NewBouncingBall(fb.Size().X, fb.Size().Y)
	display.SetAnimation(animation)

	// Запускаем рендеринг
	display.Start()

	// Работаем 10 секунд
	time.Sleep(10 * time.Second)

	// Останавливаем
	display.Stop()
}
