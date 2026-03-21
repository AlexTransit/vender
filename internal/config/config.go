package config_global

import (
	"os"

	"github.com/AlexTransit/vender/currency"
	engine_config "github.com/AlexTransit/vender/internal/engine/config"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	menu_config "github.com/AlexTransit/vender/internal/menu/menu_config"
	sound_config "github.com/AlexTransit/vender/internal/sound/config"
	ui_config "github.com/AlexTransit/vender/internal/ui/config"
	watchdog_config "github.com/AlexTransit/vender/internal/watchdog/config"
	"github.com/AlexTransit/vender/log2"
	tele_config "github.com/AlexTransit/vender/tele/config"
	"github.com/AlexTransit/vender/hardware/hd44780"
	mdb_config "github.com/AlexTransit/vender/hardware/mdb/config"
	evend_config "github.com/AlexTransit/vender/hardware/mdb/evend/config"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// for test/ make config from ctrusture
func WriteConfigToFile() {
	// 	c := CT{
	// 		Inv: InvStruct{
	// 			Persist: false,
	// 			Stocks:  Stocktruct{},
	// 		},
	// 	}
	f := hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(newDefaultConfig(), f.Body())
	file, err := os.OpenFile("defaultConfig.hcl", os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	_, err = file.Write(f.Bytes())
	if err != nil {
		panic(err)
	}
}

func (c *Config) ScaleI(i int) currency.Amount {
	return currency.Amount(i) * currency.Amount(c.Money.Scale)
}
func (c *Config) ScaleU(u uint32) currency.Amount          { return currency.Amount(u * uint32(c.Money.Scale)) }
func (c *Config) ScaleA(a currency.Amount) currency.Amount { return a * currency.Amount(c.Money.Scale) }

type configLoadStruct struct {
	log      *log2.Log
	includes []string
	bodies   []hcl.Body
}

var includeFile = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "include", LabelNames: []string{""}},
	},
}

func (c *configLoadStruct) readConfig(fileName string) {
	for _, v := range c.includes {
		if v == fileName {
			return
		}
	}
	c.includes = append(c.includes, fileName)
	src, err := os.ReadFile(fileName)
	if err != nil {
		c.log.Errorf("read config file(%v) error(%v)", fileName, err)
		return
	}
	file, diags := hclsyntax.ParseConfig(src, fileName, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		c.log.Fatalf("parse config file(%v) error(%v)", fileName, diags)
	}
	bc, _ := file.Body.Content(includeFile)
	c.bodies = append(c.bodies, file.Body)
	for _, blockValue := range bc.Blocks {
		c.readConfig(blockValue.Labels[0])
	}
}

// конфигурация может быть перезаписана
// есть базывый конфиг, который может быть скорректирован записями ниже и записями во вложенных файлах.
// для перезаписи создается карта в которой обновляются данные из масива
// при инициалицации данные их карты перемещаются в рабочий масив
// в инвенторе есть списоки ингридиентов и складов ( бункеров)
// в складе указывается ссылка на ингредиент
// ключи ингредиента - название, для склада - код
func ReadConfig(log *log2.Log, fn string) *Config {
	cfg := newDefaultConfig()
	cc := configLoadStruct{log: log}
	cc.readConfig(fn) // read all config files
	// overwrite duplacates values
	for i := range cc.bodies {
		_ = gohcl.DecodeBody(cc.bodies[i], nil, cfg)
		for _, v := range cfg.Hardware.XXX_Devices {
			devConf := cfg.Hardware.EvendDevices[v.Name]
			devConf.Name = v.Name
			if v.Required {
				devConf.Required = true
			}
			if v.Disabled {
				devConf.Disabled = true
			}
			cfg.Hardware.EvendDevices[v.Name] = devConf
		}
		cfg.Hardware.XXX_Devices = nil
		for _, v := range cfg.UI_config.Service.XXX_Tests {
			uiTest := ui_config.TestsStruct{
				Name:     v.Name,
				Scenario: v.Scenario,
			}
			cfg.UI_config.Service.Tests[v.Name] = uiTest
		}
		cfg.UI_config.Service.XXX_Tests = nil
		for _, v := range cfg.Inventory.Stocks {
			confStock := cfg.Inventory.XXX_Stocks[v.Label]
			confStock.Label = v.Label
			if v.Code != 0 {
				confStock.Code = v.Code
			}
			if v.RegisterAdd != "" {
				confStock.RegisterAdd = v.RegisterAdd
			}
			if v.XXX_Ingredient != "" {
				confStock.XXX_Ingredient = v.XXX_Ingredient
			}
			cfg.Inventory.XXX_Stocks[v.Label] = confStock
		}
		cfg.Inventory.Stocks = nil
		for _, v := range cfg.Inventory.Ingredient {
			ing := cfg.Inventory.XXX_Ingredient[v.Name]
			ing.Name = v.Name
			if v.SpendRate != 0 {
				ing.SpendRate = v.SpendRate
			}
			if v.Level != "" {
				ing.Level = v.Level
			}
			if v.Min != 0 {
				ing.Min = v.Min
			}
			if v.Cost != 0 {
				ing.Cost = v.Cost
			}
			if v.TuneKey != "" {
				ing.TuneKey = v.TuneKey
			}
			cfg.Inventory.XXX_Ingredient[v.Name] = ing
		}
		cfg.Inventory.Ingredient = nil
		for _, v := range cfg.Engine.XXX_Aliases {
			s := engine_config.Alias{
				Name:     v.Name,
				Scenario: v.Scenario,
			}
			cfg.Engine.Aliases[v.Name] = s
		}
		cfg.Engine.XXX_Aliases = nil
		for _, v := range cfg.Engine.XXX_Menu.XXX_Items {
			mi := cfg.Engine.Menu.Items[v.Code]
			mi.Code = v.Code
			if v.Disabled {
				mi.Disabled = true
			}
			if v.Name != "" {
				mi.Name = v.Name
			}
			if v.Scenario != "" {
				mi.Scenario = v.Scenario
			}
			if v.CreamMax != 0 {
				mi.CreamMax = v.CreamMax
			}
			if v.SugarMax != 0 {
				mi.SugarMax = v.SugarMax
			}
			if v.XXX_Price != 0 {
				mi.Price = cfg.ScaleI(v.XXX_Price)
			}
			cfg.Engine.Menu.Items[v.Code] = mi
		}
		cfg.Engine.XXX_Menu.XXX_Items = nil
	}
	VMC = cfg
	return cfg
}

func (u *Config) KeyboardReader(v ...bool) bool {
	if len(v) > 0 {
		u.User.KeyboardReadEnable = v[0]
	}
	return u.User.KeyboardReadEnable
}

func (u *Config) UIState(v ...uint32) uint32 {
	if len(v) > 0 {
	}
	return u.User.UiState
}

func NewConfig() *Config {
	return newDefaultConfig()
}

func newDefaultConfig() *Config {
	return &Config{
		ErrorFolder: "/home/vmc/vender-db/errors/",
		BrokenFile:  "/home/vmc/broken",
		Inventory: inventory.Inventory{
			File:           "/home/vmc/vender-db/inventory/store.file",
			XXX_Stocks:     map[string]inventory.Stock{},
			XXX_Ingredient: map[string]inventory.Ingredient{},
		},
		Money: MoneyStruct{
			Scale:       100,
			CreditMax:   100,
			MinimalBill: 10,
			MaximumBill: 100,
		},
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
				Enable:        true,
				PinChip:       "/dev/gpiochip0",
				Pinmap:        hd44780.PinMap{RS: "13", RW: "14", E: "110", D4: "68", D5: "71", D6: "2", D7: "21"},
				Width:         16,
				ControlBlink:  false,
				ControlCursor: false,
				ScrollDelay:   210,
			},
			Input: InputStruct{
				EvendKeyboard: EvendKeyboardStruct{Enable: true},
			},
			Mdb: mdb_config.Config{
				UartDriver: "mega",
			},
			Mega: MegaStruct{
				SpiSpeed: "100kHz",
				PinChip:  "/dev/gpiochip0",
				Pin:      "6",
			},
		},
		Tele: tele_config.Config{},
		UI_config: ui_config.Config{
			Front: ui_config.FrontStruct{
				MsgMenuError:                "ОШИБКА",
				MsgWait:                     "пожалуйста, подождите",
				MsgWaterTemp:                "температура: %d",
				MsgMenuCodeEmpty:            "Укажите код.",
				MsgMenuCodeInvalid:          "Неправильный код",
				MsgMenuInsufficientCreditL1: "Мало денег",
				MsgMenuInsufficientCreditL2: "дали:%s нужно:%s",
				MsgMenuNotAvailable:         "Не доступен. Выберите другой, или вернем деньги.",
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
			DefaultVolume: 100,
			Folder:        "/home/vmc/vender-db/audio/",
			KeyBeep:       "bb.mp3",
			MoneyIn:       "moneyIn.mp3",
			TTSExec: []string{
				"/home/vmc/vender-db/audio/tts/piper",
				"--model", "/home/vmc/vender-db/audio/tts/ruslan/voice.onnx",
				"--config", "/home/vmc/vender-db/audio/tts/ruslan/voice.json",
			},
		},
		Watchdog: watchdog_config.Config{Folder: "/run/user/1000/"},
		Engine: engine_config.Config{
			Aliases: map[string]engine_config.Alias{},
			Menu: menu_config.MenuStruct{
				DefaultCream:    4,
				DefaultCreamMax: 6,
				DefaultSugar:    4,
				DefaultSugarMax: 8,
				Items:           map[string]menu_config.MenuItem{},
			},
		},
	}
}
