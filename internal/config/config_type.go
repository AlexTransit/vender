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
	"github.com/hashicorp/hcl/v2"
)

type Config struct {
	Version        string
	TeleN          tele_api.Teler
	UpgradeScript  string `hcl:"upgrade_script,optional"`
	ScriptIfBroken string `hcl:"script_if_broken,optional"`
	Inventory      inventory.Inventory
	XXX_Inventory  inventory.XXX_Inventory `hcl:"inventory,block"`
	Money          MoneyStruct             `hcl:"money,block"`
	Hardware       HardwareStruct          `hcl:"hardware,block"`
	Persist        PersistStruct           `hcl:"persist,block"`
	Tele           tele_config.Config      `hcl:"tele,block"`
	UI_config      ui_config.Config        `hcl:"ui,block"`
	Sound          sound_config.Config     `hcl:"sound,block"`
	Watchdog       watchdog_config.Config  `hcl:"watchdog,block"`
	Engine         engine_config.Config    `hcl:"engine,block"`
	Remains        hcl.Body                `hcl:",remain"`
	User           ui_config.UIUser
}

type DeviceConfig struct {
	Name     string `hcl:"name,label"`
	Required bool   `hcl:"required,optional"`
	Disabled bool   `hcl:"disabled,optional"`
}

type HardwareStruct struct {
	EvendDevices map[string]DeviceConfig
	XXX_Devices  []DeviceConfig      `hcl:"device,block"`
	Evend        evend_config.Config `hcl:"evend,block"`
	Display      DisplayStruct       `hcl:"display,block"`
	HD44780      HD44780Struct       `hcl:"hd44780,block"`
	IodinPath    string              `hcl:"iodin_path,optional"`
	Input        InputStruct         `hcl:"input,block"`
	Mdb          mdb_config.Config   `hcl:"mdb,block"`
	Mega         MegaStruct          `hcl:"mega,block"`
}

type PersistStruct struct {
	Root string `hcl:"root"`
}

type DisplayStruct struct {
	Framebuffer string `hcl:"framebuffer"`
}

type HD44780Struct struct {
	Enable        bool           `hcl:"enable"`
	Codepage      string         `hcl:"codepage"`
	PinChip       string         `hcl:"pin_chip"`
	Pinmap        hd44780.PinMap `hcl:"pinmap,block"`
	Page1         bool           `hcl:"page1"`
	Width         int            `hcl:"width"`
	ControlBlink  bool           `hcl:"blink"`
	ControlCursor bool           `hcl:"cursor"`
	ScrollDelay   int            `hcl:"scroll_delay"`
}

type InputStruct struct {
	EvendKeyboard EvendKeyboardStruct `hcl:"evend_keyboard,block"`
	ServiceKey    string              `hcl:"service_key,optional"`
}

type EvendKeyboardStruct struct {
	Enable bool `hcl:"enable,optional"`
}

type MegaStruct struct {
	LogDebug bool   `hcl:"log_debug,optional"`
	Spi      string `hcl:"spi,optional"`
	SpiSpeed string `hcl:"spi_speed,optional"`
	PinChip  string `hcl:"pin_chip,optional"`
	Pin      string `hcl:"pin,optional"`
}

type MoneyStruct struct {
	Scale     int `hcl:"scale"`
	CreditMax int `hcl:"credit_max"`
}

// config wich presetted default values
var VMC = Config{
	XXX_Inventory: inventory.XXX_Inventory{
		XXX_Stocks:     map[int]inventory.Conf_Stock{},
		XXX_Ingredient: map[string]inventory.Conf_Ingredient{},
	},
	Money: MoneyStruct{Scale: 100, CreditMax: 100},
	Hardware: HardwareStruct{
		EvendDevices: map[string]DeviceConfig{},
		Evend: evend_config.Config{
			Cup:      evend_config.CupStruct{TimeoutSec: 60},
			Elevator: evend_config.ElevatorStruct{MoveTimeoutSec: 100},
			Espresso: evend_config.EspressoStruct{TimeoutSec: 300},
			Valve:    evend_config.ValveStruct{TemperatureHot: 86},
		},
		Display: DisplayStruct{Framebuffer: "/dev/fb0"},
		HD44780: HD44780Struct{
			Codepage: "windows-1251",
			PinChip:  "/dev/gpiochip0",
			Pinmap: hd44780.PinMap{
				RS: "13",
				RW: "14",
				E:  "110",
				D4: "68",
				D5: "71",
				D6: "2",
				D7: "21",
			},
			Width:       16,
			ScrollDelay: 120,
		},
		IodinPath: "", //?????????????????????????????????????????
		Input: InputStruct{
			EvendKeyboard: EvendKeyboardStruct{Enable: true},
			ServiceKey:    "", //?????????????????????????????????????????
		},
		Mdb: mdb_config.Config{
			UartDevice: "", //?????????????????????????????????????????
			UartDriver: "mega",
		},
		Mega: MegaStruct{
			Spi:      "", //?????????????????????????????????????????
			SpiSpeed: "100kHz",
			PinChip:  "/dev/gpiochip0",
			Pin:      "6",
		},
	},
	Persist: PersistStruct{Root: "/home/vmc/vender-db"},
	Tele:    tele_config.Config{},
	UI_config: ui_config.Config{
		Front: ui_config.FrontStruct{
			MsgMenuError:                "ОШИБКА",
			MsgWait:                     "пожалуйста, подождите",
			MsgWaterTemp:                "температура: %d",
			MsgMenuCodeEmpty:            "Укажите код.",
			MsgMenuCodeInvalid:          "Неправильный код",
			MsgMenuInsufficientCreditL1: "Мало денег",
			MsgMenuInsufficientCreditL2: "дали:%s нужно:%s",
			MsgMenuNotAvailable:         "Не доступен. Выверите другой, или вернем деньги.",
			MsgCream:                    "Сливки",
			MsgSugar:                    "Caxap",
			MsgCredit:                   "Кредит: ",
			MsgInputCode:                "Код: %s",
			MsgPrice:                    "цена:%sp.",
			MsgRemotePay:                "QR ",
			MsgRemotePayRequest:         "запрос QR кода",
			MsgRemotePayReject:          "Банк послал :(",
			MsgNoNetwork:                "нет связи :(",
			ResetTimeoutSec:             300,
			PicQRPayError:               "/home/vmc/pic-qrerror",
			PicPayReject:                "/home/vmc/pic-pay-reject",
			LightShedule:                "(* 06:00-23:00)",
		},
		Service: ui_config.ServiceStruct{
			ResetTimeoutSec: 1800,
			Tests:           map[string]ui_config.TestsStruct{},
		},
	},
	Sound: sound_config.Config{
		DefaultVolume: 10,
		Folder:        "/home/vmc/vender-db/audio/",
		KeyBeep:       "bb.mp3",
		MoneyIn:       "moneyIn.mp3",
	},
	Watchdog: watchdog_config.Config{Folder: "/run/user/1000/"},
	Engine: engine_config.Config{
		Aliases: map[string]engine_config.Alias{},
		Menu:    menu_config.MenuStruct{Items: map[string]menu_config.MenuItem{}},
	},
}

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
