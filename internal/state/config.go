package state

import (
	"path/filepath"
	"sync"

	"github.com/hashicorp/hcl"
	"github.com/juju/errors"
	"github.com/temoto/vender/currency"
	"github.com/temoto/vender/hardware/hd44780"
	mdb_config "github.com/temoto/vender/hardware/mdb/config"
	evend_config "github.com/temoto/vender/hardware/mdb/evend/config"
	"github.com/temoto/vender/helpers"
	engine_config "github.com/temoto/vender/internal/engine/config"
	ui_config "github.com/temoto/vender/internal/ui/config"
	"github.com/temoto/vender/log2"
	tele_config "github.com/temoto/vender/tele/config"
)

type Config struct {
	// includeSeen contains absolute paths to prevent include loops
	includeSeen map[string]struct{}
	// only used for Unmarshal, do not access
	XXX_Include []ConfigSource `hcl:"include"`

	Debug struct {
		PprofListen string `hcl:"pprof_listen"`
	}

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
			DevInputEvent struct {
				Enable bool   `hcl:"enable"`
				Device string `hcl:"device"`
			} `hcl:"dev_input_event"`
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
		Scale                int `hcl:"scale"`
		CreditMax            int `hcl:"credit_max"`
		ChangeOverCompensate int `hcl:"change_over_compensate"`
	}
	Persist struct {
		Root string `hcl:"root"`
	}
	Tele tele_config.Config
	UI   ui_config.Config

	_copy_guard sync.Mutex //nolint:unused
}

type DeviceConfig struct {
	Name     string `hcl:"name,key"`
	Required bool   `hcl:"required"`
}

type ConfigSource struct {
	Name     string `hcl:"name,key"`
	Optional bool   `hcl:"optional"`
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

	err = hcl.Unmarshal(bs, c)
	if err != nil {
		err = errors.Annotatef(err, "config unmarshal source=%s content='%s'", source.Name, string(bs))
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
	return c
}
