package main

import (
	"time"

	"github.com/AlexTransit/vender/hardware/display/ebitendisplay"
	"github.com/AlexTransit/vender/hardware/display/framebuffer"
	// "github.com/AlexTransit/vender/internal/ebitendisplay"
)

func main() {
	// Открываем framebuffer
	fb, err := framebuffer.New("/dev/fb0")
	if err != nil {
		panic(err)
	}
	defer fb.Close()

	// Создаем дисплей
	display, err := ebitendisplay.NewDisplay(fb)
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

	// // Инициализация framebuffer
	// fb, err := framebuffer.New("/dev/fb0")
	// if err != nil {
	// 	log.Fatal("Ошибка открытия framebuffer:", err)
	// }
	// defer fb.Close()

	// log.Printf("Размер дисплея: %dx%d", fb.Bounds().Dx(), fb.Bounds().Dy())

	// // Создание дисплея с Ebiten
	// display, err := ebitendisplay.NewDisplay(fb)
	// if err != nil {
	// 	log.Fatal("Ошибка создания дисплея:", err)
	// }

	// // Выбор анимации для теста
	// // Вариант 1: Прыгающий мяч
	// animation := ebitendisplay.NewBouncingBall(fb.Bounds().Dx(), fb.Bounds().Dy())

	// // Вариант 2: Вращающийся квадрат (раскомментируй для теста)
	// // animation := ebitendisplay.NewRotatingSquare(fb.Bounds().Dx(), fb.Bounds().Dy())

	// // Вариант 3: Кольцевой прогресс-бар
	// // animation := ebitendisplay.NewRingProgress(fb.Bounds().Dx(), fb.Bounds().Dy())

	// display.SetAnimation(animation)

	// // Запуск цикла рендеринга
	// ebiten.SetWindowSize(fb.Bounds().Dx(), fb.Bounds().Dy())
	// ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)

	// log.Println("Запуск анимации... Нажмите Ctrl+C для остановки")

	// if err := ebiten.RunGame(display); err != nil {
	// 	log.Fatal("Ошибка запуска Ebiten:", err)
	// }
}
