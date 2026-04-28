package config_global

import (
	"github.com/AlexTransit/vender/hardware/hd44780"
	mdb_config "github.com/AlexTransit/vender/hardware/mdb/config"
	evend_config "github.com/AlexTransit/vender/hardware/mdb/evend/config"
	engine_config "github.com/AlexTransit/vender/internal/engine/config"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	menu_config "github.com/AlexTransit/vender/internal/menu/menu_config"
	sound_config "github.com/AlexTransit/vender/internal/sound/config"
	ui_config "github.com/AlexTransit/vender/internal/ui/config"
	watchdog_config "github.com/AlexTransit/vender/internal/watchdog/config"
	tele_api "github.com/AlexTransit/vender/tele"
	tele_config "github.com/AlexTransit/vender/tele/config"
)

type Config struct {
	Version string
	TeleN   tele_api.Teler
	// RU: UpgradeScript - скрипт, запускаемый для обновления системы после команды vmc.upgrade!.
	// EN: UpgradeScript - a script run to update the system after the vmc.upgrade command!
	// Example: sudo -u vmc git -C /home/vmc/vender-distr/ pull && sudo -u vmc -i /home/vmc/vender-distr/script/build && rsync -av /home/vmc/vender-distr/build/vender /home/vmc/ && logger upgrade complete. run reload triger
	UpgradeScript string `hcl:"upgrade_script,optional"`
	// ScriptIfBroken is executed when the system is broken
	ScriptIfBroken string `hcl:"script_if_broken,optional"`
	// ErrorFolder where error logs are stored
	ErrorFolder string `hcl:"error_folder,optional"`
	// BrokenFile for broken state logging
	BrokenFile string `hcl:"broken_file,optional"`
	// Inventory configuration for stock management
	Inventory inventory.Inventory `hcl:"inventory,block"`
	// RU: настройки для валидаторов
	// EN: Money configuration for payment settings
	Money MoneyStruct `hcl:"money,block"`
	// RU: Конфигурация для аппаратной части
	// EN:Hardware configuration for devices
	Hardware HardwareStruct `hcl:"hardware,block"`
	// Persist        PersistStruct          `hcl:"persist,block"`
	// Tele configuration for telemetry
	Tele tele_config.Config `hcl:"tele,block"`
	// UI_config for user interface settings
	UI_config ui_config.Config `hcl:"ui,block"`
	// RU: Конфигурация для звука
	// EN: Sound configuration for audio settings
	Sound sound_config.Config `hcl:"sound,block"`
	// RU: Конфигурация для системы наблюдения за сервисом. Если стророжевую собаку не кормить то сервис будет перезапущен.
	Watchdog watchdog_config.Config `hcl:"watchdog,block"`
	// RU: Конфигурация для движка. В ней описано как готовить напитки, какие вложенные сценарии использовать и т.д.
	Engine engine_config.Config `hcl:"engine,block"`
	// Remains   hcl.Body               `hcl:",remain"`
	User ui_config.UIUser
}

type DeviceConfig struct {
	// RU: имя устройства, например "mixer", "hopper1", "hopper2" и т.д. Должно быть уникальным в рамках конфигурации.
	Name string `hcl:"name,label"`
	// RU: required - если true, то устройство обязательно для работы системы. Если устройство с таким именем не будет найдено, система перейдет в состояние "сломано".
	Required bool `hcl:"required,optional"`
	// RU: disabled - если true, то устройство будет отключено. Система будет игнорировать его отсутствие и не будет пытаться с ним взаимодействовать. Это может быть полезно для устройств, которые не всегда нужны или для временного отключения устройства без удаления его конфигурации.
	Disabled bool `hcl:"disabled,optional"`
}

type HardwareStruct struct {
	EvendDevices map[string]DeviceConfig
	// RU: список устройств.
	XXX_Devices []DeviceConfig      `hcl:"device,block"`
	Evend       evend_config.Config `hcl:"evend,block"`
	// RU: графический дисплей.
	// EN: graphical display.
	Display DisplayStruct `hcl:"display,block"`
	// RU: Конфигурация для LCD 16х2 дисплея HD44780
	// EN: Configuration for LCD 16x2 HD44780 display
	HD44780   HD44780Struct `hcl:"hd44780,block"`
	IodinPath string        `hcl:"iodin_path,optional"`
	// RU: Конфигурация для устройств ввода.
	Input InputStruct `hcl:"input,block"`
	// RU: Конфигурация для MDB шины.
	// EN: Configuration for MDB bus.
	Mdb mdb_config.Config `hcl:"mdb,block"`
	// RU: Конфигурация для устройства шлюза. одноплатник не умеет работать с 9 битным UART. для работы с MDB устройствами нужен драйвер. драйвер сделал на Atmel 168 или 328. к драйверу подключена evend клавиатура по i2c и mdb шина. общение с драйверов идет поверх SPI.
	Mega MegaStruct `hcl:"mega,block"`
}

type DisplayStruct struct {
	// RU: путь к фреймбуферу для графического дисплея.
	// EN: path to framebuffer for graphical display.
	Framebuffer string `hcl:"framebuffer"`
}

type HD44780Struct struct {
	// RU: включить поддержку HD44780 дисплея.
	Enable bool `hcl:"enable"`
	// RU: путь к gpiochip для управления пинами дисплея.
	PinChip string `hcl:"pin_chip"`
	// RU: распиновка для четырех битного подключения дисплея. RS, RW, E, D4, D5, D6, D7. Значения должны быть номерами пинов на gpiochip.
	Pinmap hd44780.PinMap `hcl:"pinmap,block"`
	// RU: ширина дисплея в символах.
	Width int `hcl:"width"`
	// RU: мигание курсора.
	ControlBlink bool `hcl:"blink"`
	// RU: отображение курсора.
	ControlCursor bool `hcl:"cursor"`
	// RU: задержка при прокрутке текста в миллисекундах.
	ScrollDelay int `hcl:"scroll_delay"`
}

type InputStruct struct {
	EvendKeyboard EvendKeyboardStruct `hcl:"evend_keyboard,block"`
	ServiceKey    string              `hcl:"service_key,optional"`
}

type EvendKeyboardStruct struct {
	Enable bool `hcl:"enable,optional"`
}

type MegaStruct struct {
	// RU: логирование отладочной информации от драйвера Mega. может быть полезно для диагностики проблем с MDB устройствами и клавиатурой evend.
	LogDebug bool   `hcl:"log_debug,optional"`
	Spi      string `hcl:"spi,optional"`
	// RU: скорость SPI соединения с драйвером Mega.
	SpiSpeed string `hcl:"spi_speed,optional"`
	// RU: gpiochip для прерывания от меги (когда приходит событие от MDB устройства или клавиатуры evend).
	PinChip string `hcl:"pin_chip,optional"`
	// RU: номер пина на gpiochip для прерывания от меги.
	Pin string `hcl:"pin,optional"`
}

type MoneyStruct struct {
	// RU: десятичная точка.
	Scale int `hcl:"scale"`
	// RU: максимальный кредит. сумма после которой аппарат откажется принимать деньги.
	CreditMax int `hcl:"credit_max"`
	// RU: минимальная купюра. сумма, меньше которой аппарат не будет принимать деньги.
	MinimalBill int `hcl:"minimal_bill"`
	// RU: максимальная купюра. сумма, больше которой аппарат не будет принимать деньги.
	MaximumBill int `hcl:"maximum_bill"`
}

var VMC = newDefaultConfig()

func DefaultSugar() uint8 {
	return VMC.Engine.Menu.DefaultSugar
}

func DefaultCream() uint8 {
	return VMC.Engine.Menu.DefaultCream
}

func SugarMax() uint8 {
	return VMC.Engine.Menu.DefaultSugarMax
}

func CreamMax() uint8 {
	return VMC.Engine.Menu.DefaultCreamMax
}

func GetMenuItem(menuCode string) (mi menu_config.MenuItem, ok bool) {
	mi, ok = VMC.Engine.Menu.Items[menuCode]
	return
}
