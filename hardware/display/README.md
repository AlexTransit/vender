--- hardware/display/ebitendisplay/README.md (原始)


+++ hardware/display/ebitendisplay/README.md (修改后)
# Ebiten Display для Orange Pi Lite

Пакет `ebitendisplay` предоставляет рендеринг анимации через библиотеку [Ebiten](https://github.com/hajimehoshi/ebiten) с выводом в framebuffer Linux.

## Возможности

- Рендеринг через Ebiten (аппаратно-ускоренная графика)
- Вывод в framebuffer (`/dev/fb0`, `/dev/fb1` и т.д.)
- Готовые анимации:
  - Прыгающий мяч (`BouncingBall`)
  - Вращающийся квадрат (`RotatingSquare`)
  - Кольцо прогресса (`ProgressRing`)
- Композиция анимаций (`CompositeAnimation`)
- Поддержка собственных анимаций через интерфейс `Animation`

## Установка зависимостей

Для работы с Ebiten требуются системные библиотеки:

```bash
apt update
apt install pkg-config libasound2-dev libgl1-mesa-dev xorg-dev
```

## Обновление vendor

Если вы используете vendor-зависимости, обновите их:

```bash
go mod vendor
```

Или используйте режим без vendor:

```bash
go build -mod=mod
```

## Быстрый старт

### Пример 1: Прыгающий мяч

```go
package main

import (
    "context"
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
```

### Пример 2: Несколько анимаций одновременно

```go
package main

import (
    "context"
    "time"

    "github.com/AlexTransit/vender/hardware/display/ebitendisplay"
    "github.com/AlexTransit/vender/hardware/display/framebuffer"
)

func main() {
    fb, _ := framebuffer.New("/dev/fb0")
    defer fb.Close()

    display, _ := ebitendisplay.New(fb)

    // Создаем композицию анимаций
    composite := ebitendisplay.NewCompositeAnimation()
    composite.Add(ebitendisplay.NewBouncingBall(fb.Size().X, fb.Size().Y))
    composite.Add(ebitendisplay.NewRotatingSquare(fb.Size().X, fb.Size().Y))
    composite.Add(ebitendisplay.NewProgressRing(fb.Size().X, fb.Size().Y))

    display.SetAnimation(composite)
    display.Start()

    time.Sleep(15 * time.Second)
    display.Stop()
}
```

### Пример 3: Анимация загрузки

```go
package main

import (
    "context"
    "time"

    "github.com/AlexTransit/vender/hardware/display/ebitendisplay"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Показываем анимацию загрузки 5 секунд
    ebitendisplay.ShowLoading(ctx, "/dev/fb0", 5*time.Second)
}
```

### Пример 4: Собственная анимация

```go
package main

import (
    "image/color"
    "math"

    "github.com/AlexTransit/vender/hardware/display/ebitendisplay"
    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/vector"
)

// PulsingCircle анимация пульсирующего круга
type PulsingCircle struct {
    x, y       float32
    baseRadius float32
    radius     float32
    angle      float32
    color      color.Color
}

func NewPulsingCircle(width, height int) *PulsingCircle {
    return &PulsingCircle{
        x:          float32(width) / 2,
        y:          float32(height) / 2,
        baseRadius: 30,
        angle:      0,
        color:      color.RGBA{255, 100, 100, 255},
    }
}

func (p *PulsingCircle) Update() {
    p.angle += 0.1
    p.radius = p.baseRadius + 10*float32(math.Sin(float64(p.angle)))
}

func (p *PulsingCircle) Draw(screen *ebiten.Image) {
    vector.DrawFilledCircle(screen, p.x, p.y, p.radius, p.color, true)
}

func main() {
    // Используйте PulsingCircle как обычную анимацию
    animation := NewPulsingCircle(320, 240)
    // ... подключите к display
}
```

## API

### Основные типы

#### `Display`
Основной тип для работы с дисплеем.

- `New(fb *framebuffer.Framebuffer) (*Display, error)` - создание нового дисплея
- `SetAnimation(animation Animation)` - установка анимации
- `Start()` - запуск цикла рендеринга
- `Stop()` - остановка рендеринга
- `SetFPS(fps float64)` - установка целевого FPS (по умолчанию 30)
- `Clear() error` - очистка экрана
- `RenderOneShot(drawFn func(screen *ebiten.Image)) error` - рендер одного кадра

#### `Animation` (интерфейс)
Интерфейс для всех анимаций:

```go
type Animation interface {
    Update()
    Draw(screen *ebiten.Image)
}
```

### Готовые анимации

- `NewBouncingBall(width, height int) *BouncingBall` - прыгающий мяч
- `NewRotatingSquare(width, height int) *RotatingSquare` - вращающийся квадрат
- `NewProgressRing(width, height int) *ProgressRing` - кольцо прогресса
- `NewCompositeAnimation() *CompositeAnimation` - композиция анимаций

### Утилиты

- `DrawCircleOnScreen(dev string, x, y, radius float32, clr color.Color) error` - нарисовать круг
- `DrawRectangleOnScreen(dev string, x, y, w, h float32, clr color.Color) error` - нарисовать прямоугольник
- `ShowLoading(ctx context.Context, dev string, duration time.Duration) error` - показать анимацию загрузки
- `Demo(ctx context.Context, dev string) error` - демонстрация всех анимаций

## Интеграция с существующим кодом

Если у вас уже есть код, работающий с `hardware/display`, вы можете использовать `ebitendisplay` как дополнение:

```go
// Старый способ (копирование картинки)
display.CopyFile2FB("image.raw")

// Новый способ (анимация через ebiten)
fb, _ := framebuffer.New("/dev/fb0")
ebitenDisplay, _ := ebitendisplay.New(fb)
ebitenDisplay.SetAnimation(ebitendisplay.NewBouncingBall(...))
ebitenDisplay.Start()
```

## Оптимизация для SPI-дисплея

Для SPI-дисплеев с низкой пропускной способностью:

1. Уменьшите FPS: `display.SetFPS(15.0)`
2. Уменьшите разрешение рендеринга
3. Используйте простые формы вместо сложных изображений
4. Избегайте частых аллокаций в цикле анимации

## Отладка

Для отладки без реального дисплея можно использовать mock-framebuffer или выводить в файл:

```go
// Создайте виртуальный framebuffer для тестов
fb := framebuffer.NewMock(320, 240)
```

## Лицензия

Лицензия совпадает с основным проектом vender.
