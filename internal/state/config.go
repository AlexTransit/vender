package state

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/hd44780"
	mdb_config "github.com/AlexTransit/vender/hardware/mdb/config"
	evend_config "github.com/AlexTransit/vender/hardware/mdb/evend/config"
	"github.com/AlexTransit/vender/helpers"
	engine_config "github.com/AlexTransit/vender/internal/engine/config"
	"github.com/AlexTransit/vender/internal/sound"
	ui_config "github.com/AlexTransit/vender/internal/ui/config"
	"github.com/AlexTransit/vender/internal/watchdog"
	"github.com/AlexTransit/vender/log2"
	tele_config "github.com/AlexTransit/vender/tele/config"
	hcl1 "github.com/hashicorp/hcl"

	// "github.com/hashicorp/hcl/v2/gohcl"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/juju/errors"
)

type Config struct {
	// includeSeen contains absolute paths to prevent include loops
	includeSeen map[string]struct{}
	// only used for Unmarshal, do not access
	XXX_Include    []ConfigSource `hcl:"include"`
	UpgradeScript  string         `hcl:"upgrade_script"`
	ScriptIfBroken string         `hcl:"script_if_broken"`

	Hardware struct {
		// only used for Unmarshal, do not access
		XXX_Devices []DeviceConfig      `hcl:"device"`
		Evend       evend_config.Config `hcl:"evend"`
		Display     struct {
			Framebuffer string `hcl:"framebuffer"`
		}
		HD44780 struct { //nolint:maligned
			Enable        bool           `hcl:"enable"`
			Codepage      string         `hcl:"codepage"`
			PinChip       string         `hcl:"pin_chip"`
			Pinmap        hd44780.PinMap `hcl:"pinmap"`
			Page1         bool           `hcl:"page1"`
			Width         int            `hcl:"width"`
			ControlBlink  bool           `hcl:"blink"`
			ControlCursor bool           `hcl:"cursor"`
			ScrollDelay   int            `hcl:"scroll_delay"`
		}
		IodinPath string `hcl:"iodin_path"`
		Input     struct {
			EvendKeyboard struct {
				Enable bool `hcl:"enable"`
				// TODO ListenAddr int
			} `hcl:"evend_keyboard"`
			ServiceKey string `hcl:"service_key"`
		}
		Mdb  mdb_config.Config `hcl:"mdb"`
		Mega struct {
			LogDebug bool   `hcl:"log_debug"`
			Spi      string `hcl:"spi"`
			SpiSpeed string `hcl:"spi_speed"`
			PinChip  string `hcl:"pin_chip"`
			Pin      string `hcl:"pin"`
		}
	}

	Engine engine_config.Config
	Money  struct {
		Scale                  int  `hcl:"scale"`
		CreditMax              int  `hcl:"credit_max"`
		EnableChangeBillToCoin bool `hcl:"enable_change_bill_to_coin"`
		ChangeOverCompensate   int  `hcl:"change_over_compensate"`
	}
	Persist struct {
		Root string `hcl:"root"`
	}
	Tele     tele_config.Config
	UI       ui_config.Config
	Sound    sound.Config
	Watchdog watchdog.Config
	// _copy_guard sync.Mutex //nolint:unused
}

type DeviceConfig struct {
	Name     string `hcl:"name,key"`
	Required bool   `hcl:"required"`
	Disabled bool
}

type ConfigSource struct {
	Name     string `hcl:"name,key"`
	Optional bool   `hcl:"optional"`
}

type CT struct {
	UpgradeScript string `hcl:"upgrade_script"`
}

func ReadConf(ctx context.Context, fp *string) *Config {
	var c CT
	*fp = "/home/vmc/c.hcl"
	// gohcl.DecodeBody()
	er := hclsimple.DecodeFile(*fp, nil, &c)
	fmt.Printf("\033[41m %v \033[0m\n", er)
	return nil
}

func (c *Config) ScaleI(i int) currency.Amount {
	return currency.Amount(i) * currency.Amount(c.Money.Scale)
}
func (c *Config) ScaleU(u uint32) currency.Amount          { return currency.Amount(u * uint32(c.Money.Scale)) }
func (c *Config) ScaleA(a currency.Amount) currency.Amount { return a * currency.Amount(c.Money.Scale) }

func (c *Config) read(log *log2.Log, fs FullReader, source ConfigSource, errs *[]error) {
	norm := fs.Normalize(source.Name)
	if _, ok := c.includeSeen[norm]; ok {
		log.Fatalf("config duplicate source=%s", source.Name)
	} else {
		log.Debugf("config reading source='%s' path=%s", source.Name, norm)
	}
	c.includeSeen[source.Name] = struct{}{}
	c.includeSeen[norm] = struct{}{}

	bs, err := fs.ReadAll(norm)
	if bs == nil && err == nil {
		if !source.Optional {
			err = errors.NotFoundf("config required name=%s path=%s", source.Name, norm)
			*errs = append(*errs, err)
			return
		}
	}
	if err != nil {
		*errs = append(*errs, errors.Annotatef(err, "config source=%s", source.Name))
		return
	}
	// gohcl.DecodeBody(bs, nil, c)
	err = hcl1.Unmarshal(bs, c)
	if err != nil {
		err = fmt.Errorf("%v parse error (%v)", source.Name, err)
		*errs = append(*errs, err)
		return
	}

	var includes []ConfigSource
	includes, c.XXX_Include = c.XXX_Include, nil
	for _, include := range includes {
		includeNorm := fs.Normalize(include.Name)
		if _, ok := c.includeSeen[includeNorm]; ok {
			err = errors.Errorf("config include loop: from=%s include=%s", source.Name, include.Name)
			*errs = append(*errs, err)
			continue
		}
		c.read(log, fs, include, errs)
	}
}

func ReadConfig(log *log2.Log, fs FullReader, names ...string) (*Config, error) {
	if len(names) == 0 {
		log.Fatal("code error [Must]ReadConfig() without names")
	}

	if osfs, ok := fs.(*OsFullReader); ok {
		dir, name := filepath.Split(names[0])
		osfs.SetBase(dir)
		names[0] = name
	}
	c := &Config{
		includeSeen: make(map[string]struct{}),
	}
	errs := make([]error, 0, 8)
	for _, name := range names {
		c.read(log, fs, ConfigSource{Name: name}, &errs)
	}
	return c, helpers.FoldErrors(errs)
}

func MustReadConfig(log *log2.Log, fs FullReader, names ...string) *Config {
	c, err := ReadConfig(log, fs, names...)
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
	c.rewriteStock()
	c.rewriteMenu()
	c.rewriteAliace()
	c.rewriteHardware()
	return c
}

func (c *Config) rewriteAliace() {
	m := make(map[string]engine_config.Alias)
	for _, v := range c.Engine.Aliases {
		m[v.Name] = v
	}
	c.Engine.Aliases = nil
	for _, v := range m {
		c.Engine.Aliases = append(c.Engine.Aliases, v)
	}
}

func (c *Config) rewriteStock() {
	tempStock := make(map[int]engine_config.Stock)
	for _, v := range c.Engine.Inventory.Stocks {
		i := tempStock[v.Code]
		helpers.OverrideStructure(&i, &v)
		tempStock[v.Code] = i
	}
	newStore := make([]engine_config.Stock, len(tempStock))
	for i, v := range tempStock {
		newStore[i-1] = v
	}
	c.Engine.Inventory.Stocks = newStore
}

func (c *Config) rewriteMenu() {
	tempMenu := make(map[string]engine_config.MenuItem)
	for _, v := range c.Engine.Menu.Items {
		i := tempMenu[v.Code]
		helpers.OverrideStructure(&i, v)
		tempMenu[v.Code] = i
	}
	c.Engine.Menu.Items = nil
	for _, v := range tempMenu {
		if v.Disabled {
			continue
		}
		mi := v
		c.Engine.Menu.Items = append(c.Engine.Menu.Items, &mi)
	}
}

func (c *Config) rewriteHardware() {
	listDevices := make(map[string]DeviceConfig)
	for _, v := range c.Hardware.XXX_Devices {
		i := listDevices[v.Name]
		helpers.OverrideStructure(&i, &v)
		listDevices[v.Name] = i
	}
	c.Hardware.XXX_Devices = nil
	for _, v := range listDevices {
		if v.Disabled {
			continue
		}
		d := v
		c.Hardware.XXX_Devices = append(c.Hardware.XXX_Devices, d)
	}
}
