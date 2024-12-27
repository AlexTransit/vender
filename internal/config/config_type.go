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
	"github.com/AlexTransit/vender/log2"
	tele_api "github.com/AlexTransit/vender/tele"
	tele_config "github.com/AlexTransit/vender/tele/config"
	"github.com/hashicorp/hcl/v2"
)

type Config struct {
	Version        string
	Log            *log2.Log
	TeleN          tele_api.Teler
	UpgradeScript  string                 `hcl:"upgrade_script,optional"`
	ScriptIfBroken string                 `hcl:"script_if_broken,optional"`
	Money          MoneyStruct            `hcl:"money,block"`
	Hardware       HardwareStruct         `hcl:"hardware,block"`
	Persist        PersistStruct          `hcl:"persist,block"`
	Tele           tele_config.Config     `hcl:"tele,block"`
	UI_config      ui_config.Config       `hcl:"ui,block"`
	Sound          sound_config.Config    `hcl:"sound,block"`
	Watchdog       watchdog_config.Config `hcl:"watchdog,block"`
	Engine         engine_config.Config   `hcl:"engine,block"`
	Remains        hcl.Body               `hcl:",remain"`
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
	Scale                int `hcl:"scale"`
	CreditMax            int `hcl:"credit_max"`
	ChangeOverCompensate int `hcl:"change_over_compensate,optional"`
}

// config wich presetted default values
var VMC = Config{
	Log:   &log2.Log{},
	Money: MoneyStruct{Scale: 0, CreditMax: 0, ChangeOverCompensate: 0},
	Hardware: HardwareStruct{
		EvendDevices: map[string]DeviceConfig{}, XXX_Devices: []DeviceConfig{},
		Evend: evend_config.Config{},
		Display: DisplayStruct{
			Framebuffer: "/dev/fb0",
		},
		HD44780:   HD44780Struct{},
		IodinPath: "",
		Input:     InputStruct{}, Mdb: mdb_config.Config{},
		Mega: MegaStruct{},
	},
	Persist:   PersistStruct{Root: "/home/vmc/vender-db"},
	Tele:      tele_config.Config{},
	UI_config: ui_config.Config{LogDebug: false, Front: ui_config.FrontStruct{}, Service: ui_config.ServiceStruct{ResetTimeoutSec: 0, XXX_Tests: []ui_config.TestsStruct{}, Tests: map[string]ui_config.TestsStruct{}}},
	Sound: sound_config.Config{
		DefaultVolume: 10,
		Folder:        "/home/vmc/vender-db/audio/",
		KeyBeep:       "bb.mp3",
		MoneyIn:       "moneyIn.mp3",
	},
	Watchdog: watchdog_config.Config{Folder: "/run/user/1000/"},
	Engine: engine_config.Config{
		Aliases: map[string]engine_config.Alias{},
		Inventory: inventory.Inventory{
			XXX_Stocks: []inventory.Stock{},
			Stocks:     map[string]inventory.Stock{},
		},
		Menu: menu_config.MenuStruct{
			Items: map[string]menu_config.MenuItem{},
		},
	},
	Remains: nil,
	User:    ui_config.UIUser{},
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
