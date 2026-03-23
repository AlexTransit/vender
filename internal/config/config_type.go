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
	Version        string
	TeleN          tele_api.Teler
	UpgradeScript  string              `hcl:"upgrade_script,optional"`
	ScriptIfBroken string              `hcl:"script_if_broken,optional"`
	ErrorFolder    string              `hcl:"error_folder,optional"`
	BrokenFile     string              `hcl:"broken_file,optional"`
	Inventory      inventory.Inventory `hcl:"inventory,block"`
	Money          MoneyStruct         `hcl:"money,block"`
	Hardware       HardwareStruct      `hcl:"hardware,block"`
	// Persist        PersistStruct          `hcl:"persist,block"`
	Tele      tele_config.Config     `hcl:"tele,block"`
	UI_config ui_config.Config       `hcl:"ui,block"`
	Sound     sound_config.Config    `hcl:"sound,block"`
	Watchdog  watchdog_config.Config `hcl:"watchdog,block"`
	Engine    engine_config.Config   `hcl:"engine,block"`
	// Remains   hcl.Body               `hcl:",remain"`
	User ui_config.UIUser
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

type DisplayStruct struct {
	Framebuffer string `hcl:"framebuffer"`
}

type HD44780Struct struct {
	Enable        bool           `hcl:"enable"`
	PinChip       string         `hcl:"pin_chip"`
	Pinmap        hd44780.PinMap `hcl:"pinmap,block"`
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
	Scale       int `hcl:"scale"`
	CreditMax   int `hcl:"credit_max"`
	MinimalBill int `hcl:"minimal_bill"`
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
