package state

import (
	"os"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/hd44780"
	mdb_config "github.com/AlexTransit/vender/hardware/mdb/config"
	evend_config "github.com/AlexTransit/vender/hardware/mdb/evend/config"
	engine_config "github.com/AlexTransit/vender/internal/engine/config"
	sound_config "github.com/AlexTransit/vender/internal/sound/config"
	ui_config "github.com/AlexTransit/vender/internal/ui/config"
	watchdog_config "github.com/AlexTransit/vender/internal/watchdog/config"
	"github.com/AlexTransit/vender/log2"
	tele_config "github.com/AlexTransit/vender/tele/config"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type Config struct {
	UpgradeScript  string                 `hcl:"upgrade_script,optional"`
	ScriptIfBroken string                 `hcl:"script_if_broken,optional"`
	Money          MoneyStruct            `hcl:"money,block"`
	Hardware       HardwareStruct         `hcl:"hardware,block"`
	Persist        PersistStruct          `hcl:"persist,block"`
	Tele           tele_config.Config     `hcl:"tele,block"`
	UI             ui_config.Config       `hcl:"ui,block"`
	Sound          sound_config.Config    `hcl:"sound,block"`
	Watchdog       watchdog_config.Config `hcl:"watchdog,block"`
	Engine         engine_config.Config   `hcl:"engine,block"`
	Remains        hcl.Body               `hcl:",remain"`
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
	HD44780      HD44780Struct       `hcl:"HD44780,block"`
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
	Scale                  int  `hcl:"scale"`
	CreditMax              int  `hcl:"credit_max"`
	EnableChangeBillToCoin bool `hcl:"enable_change_bill_to_coin,optional"`
	ChangeOverCompensate   int  `hcl:"change_over_compensate,optional"`
}

// func ReadConf(ctx context.Context, fp *string) *Config {
// 	c := CT{
// 		Inv: InvStruct{
// 			Persist: false,
// 			Stocks:  Stocktruct{},
// 		},
// 	}
// 	f := hclwrite.NewEmptyFile()
// 	gohcl.EncodeIntoBody(&c, f.Body())
// 	fmt.Printf("%s", f.Bytes())
// 	// gohcl.DecodeBody()
// 	er := hclsimple.DecodeFile(*fp, nil, &c)
// 	*fp = "/home/alexm/c.hcl"
// 	fmt.Printf("\033[41m %v \033[0m\n%+v", er, c)
// 	return nil
// }

func (c *Config) ScaleI(i int) currency.Amount {
	return currency.Amount(i) * currency.Amount(c.Money.Scale)
}
func (c *Config) ScaleU(u uint32) currency.Amount          { return currency.Amount(u * uint32(c.Money.Scale)) }
func (c *Config) ScaleA(a currency.Amount) currency.Amount { return a * currency.Amount(c.Money.Scale) }

// func (c *Config) read(log *log2.Log, fs FullReader, source ConfigSource, errs *[]error) {
// 	norm := fs.Normalize(source.Name)
// 	if _, ok := c.includeSeen[norm]; ok {
// 		log.Fatalf("config duplicate source=%s", source.Name)
// 	} else {
// 		log.Debugf("config reading source='%s' path=%s", source.Name, norm)
// 	}
// 	c.includeSeen[source.Name] = struct{}{}
// 	c.includeSeen[norm] = struct{}{}
// 	bs, err := fs.ReadAll(norm)
// 	if bs == nil && err == nil {
// 		if !source.Optional {
// 			err = errors.NotFoundf("config required name=%s path=%s", source.Name, norm)
// 			*errs = append(*errs, err)
// 			return
// 		}
// 	}
// 	if err != nil {
// 		*errs = append(*errs, errors.Annotatef(err, "config source=%s", source.Name))
// 		return
// 	}
// 	// gohcl.DecodeBody(bs, nil, c)
// 	err = hcl1.Unmarshal(bs, c)
// 	if err != nil {
// 		err = fmt.Errorf("%v parse error (%v)", source.Name, err)
// 		*errs = append(*errs, err)
// 		return
// 	}
// 	var includes []ConfigSource
// 	includes, c.XXX_Include = c.XXX_Include, nil
// 	for _, include := range includes {
// 		includeNorm := fs.Normalize(include.Name)
// 		if _, ok := c.includeSeen[includeNorm]; ok {
// 			err = errors.Errorf("config include loop: from=%s include=%s", source.Name, include.Name)
// 			*errs = append(*errs, err)
// 			continue
// 		}
// 		c.read(log, fs, include, errs)
// 	}
// }

// func ReadConfig(log *log2.Log, fs FullReader, names ...string) (*Config, error) {
// 	if len(names) == 0 {
// 		log.Fatal("code error [Must]ReadConfig() without names")
// 	}
// 	if osfs, ok := fs.(*OsFullReader); ok {
// 		dir, name := filepath.Split(names[0])
// 		osfs.SetBase(dir)
// 		names[0] = name
// 	}
// 	c := &Config{
// 		includeSeen: make(map[string]struct{}),
// 	}
// 	errs := make([]error, 0, 8)
// 	for _, name := range names {
// 		c.read(log, fs, ConfigSource{Name: name}, &errs)
// 	}
// 	return c, helpers.FoldErrors(errs)
// }

//	func MustReadConfig_old(log *log2.Log, fs FullReader, names ...string) *Config {
//		c, err := ReadConfig(log, fs, names...)
//		if err != nil {
//			log.Fatal(errors.ErrorStack(err))
//		}
//		c.rewriteStock()
//		c.rewriteMenu()
//		c.rewriteAliace()
//		c.rewriteHardware()
//		return c
//	}

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
		// os.Exit(1)
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

func ReadConfig(log *log2.Log, fn string) *Config {
	c := Config{
		Hardware: HardwareStruct{EvendDevices: map[string]DeviceConfig{}},
		UI:       ui_config.Config{Service: ui_config.ServiceStruct{Tests: map[string]ui_config.TestsStruct{}}},
		Engine: engine_config.Config{
			Aliases:   map[string]engine_config.Alias{},
			Inventory: engine_config.Inventory{Stocks: map[string]engine_config.Stock{}},
			Menu:      engine_config.MenuStruct{Items: map[string]engine_config.MenuItem{}},
		},
	}
	cc := configLoadStruct{log: log}
	cc.readConfig(fn)
	for i := range cc.bodies {
		_ = gohcl.DecodeBody(cc.bodies[i], nil, &c)
		for _, v := range c.Hardware.XXX_Devices {
			devConf := DeviceConfig{
				Name: v.Name,
			}
			if v.Required {
				devConf.Required = true
			}
			if v.Disabled {
				devConf.Disabled = true
			}
			c.Hardware.EvendDevices[v.Name] = devConf
		}
		c.Hardware.XXX_Devices = nil
		for _, v := range c.UI.Service.XXX_Tests {
			uiTest := ui_config.TestsStruct{
				Name:     v.Name,
				Scenario: v.Scenario,
			}
			if v.Scenario != "" {
				uiTest.Scenario = v.Scenario
			}
			c.UI.Service.Tests[v.Name] = uiTest
		}
		c.UI.Service.XXX_Tests = nil
		for _, v := range c.Engine.Inventory.XXX_Stocks {
			confStock := engine_config.Stock{
				Name: v.Name,
			}
			if v.Check {
				confStock.Check = true
			}
			if v.Code != 0 {
				confStock.Code = v.Code
			}
			if v.Min != 0 {
				confStock.Min = v.Min
			}
			if v.SpendRate != 0 {
				confStock.SpendRate = v.SpendRate
			}
			if v.RegisterAdd != "" {
				confStock.RegisterAdd = v.RegisterAdd
			}
			if v.Level != "" {
				confStock.Level = v.Level
			}
			if v.TuneKey != "" {
				confStock.TuneKey = v.TuneKey
			}
			c.Engine.Inventory.Stocks[v.Name] = confStock
		}
		c.Engine.Inventory.XXX_Stocks = nil
		for _, v := range c.Engine.XXX_Aliases {
			engAlias := engine_config.Alias{
				Name:     v.Name,
				Scenario: v.Scenario,
			}
			c.Engine.Aliases[v.Name] = engAlias
		}
		c.Engine.XXX_Aliases = nil
		for _, v := range c.Engine.Menu.XXX_Items {
			mi := engine_config.MenuItem{
				Code: v.Code,
			}
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
				mi.Price = currency.Amount(v.XXX_Price)
			}
			c.Engine.Menu.Items[v.Code] = mi
		}
		c.Engine.Menu.XXX_Items = nil
	}
	return &c
}

// func findOverwrites(s *[]DeviceConfig, d []DeviceConfig) {
// 	_ = d
// 	for i := range *s {
// 		_ = i
// 	}
// }

// func (dc DeviceConfig) mergeDevice(oerwrite DeviceConfig) {
// }

// func (c *Config) rewriteAliace() {
// 	m := make(map[string]engine_config.Alias)
// 	for _, v := range c.Engine.Aliases {
// 		m[v.Name] = v
// 	}
// 	c.Engine.Aliases = nil
// 	for _, v := range m {
// 		c.Engine.Aliases = append(c.Engine.Aliases, v)
// 	}
// }

// func (c *Config) rewriteStock() {
// 	tempStock := make(map[int]engine_config.Stock)
// 	for _, v := range c.Engine.Inventory.Stocks {
// 		i := tempStock[v.Code]
// 		helpers.OverrideStructure(&i, &v)
// 		tempStock[v.Code] = i
// 	}
// 	newStore := make([]engine_config.Stock, len(tempStock))
// 	for i, v := range tempStock {
// 		newStore[i-1] = v
// 	}
// 	c.Engine.Inventory.Stocks = newStore
// }

// func (c *Config) rewriteMenu() {
// 	tempMenu := make(map[string]engine_config.MenuItem)
// 	for _, v := range c.Engine.Menu.Items {
// 		i := tempMenu[v.Code]
// 		helpers.OverrideStructure(&i, v)
// 		tempMenu[v.Code] = i
// 	}
// 	c.Engine.Menu.Items = nil
// 	for _, v := range tempMenu {
// 		if v.Disabled {
// 			continue
// 		}
// 		mi := v
// 		c.Engine.Menu.Items = append(c.Engine.Menu.Items, &mi)
// 	}
// }

// func (c *Config) rewriteHardware() {
// 	listDevices := make(map[string]DeviceConfig)
// 	for _, v := range c.Hardware.XXX_Devices {
// 		i := listDevices[v.Name]
// 		helpers.OverrideStructure(&i, &v)
// 		listDevices[v.Name] = i
// 	}
// 	c.Hardware.XXX_Devices = nil
// 	for _, v := range listDevices {
// 		if v.Disabled {
// 			continue
// 		}
// 		d := v
// 		c.Hardware.XXX_Devices = append(c.Hardware.XXX_Devices, d)
// 	}
// }
