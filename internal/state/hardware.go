package state

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AlexTransit/vender/hardware/display"
	"github.com/AlexTransit/vender/hardware/hd44780"
	"github.com/AlexTransit/vender/hardware/input"
	"github.com/AlexTransit/vender/hardware/mdb"
	mdb_client "github.com/AlexTransit/vender/hardware/mdb/client"
	"github.com/AlexTransit/vender/hardware/mega-client"
	"github.com/AlexTransit/vender/hardware/text_display"
	"github.com/AlexTransit/vender/helpers"
	config_global "github.com/AlexTransit/vender/internal/config"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
	"github.com/temoto/iodin/client/go-iodin"
)

type hardware struct {
	Display struct {
		once
		Graphic *display.Display
	}
	HD44780 struct {
		once
		Device  *hd44780.LCD
		Display *text_display.TextDisplay
	}
	Input *input.Dispatch
	Mdb   struct {
		once
		Bus    *mdb.Bus
		Uarter mdb.Uarter
	}

	devices struct {
		once
		m map[string]*devWrap
	}
	iodin struct {
		once
		client *iodin.Client
	}
	mega struct {
		once
		client *mega.Client
	}
}

type devWrap struct {
	sync.RWMutex
	config config_global.DeviceConfig
	dev    types.Devicer
}

func (g *Global) Display() (*display.Display, error) {
	x := &g.Hardware.Display // short alias
	_ = x.do(func() error {
		cfg := &g.Config.Hardware.Display
		switch {
		case cfg.Framebuffer != "":
			x.Graphic, x.err = display.NewFb(cfg.Framebuffer)
			return x.err

		default:
			// return fmt.Errorf("config: no display device (try framebuffer)")
			return nil
		}
	})
	return x.Graphic, x.err
}

func (g *Global) Iodin() (*iodin.Client, error) {
	x := &g.Hardware.iodin // short alias
	_ = x.do(func() error {
		cfg := &g.Config.Hardware
		x.client, x.err = iodin.NewClient(cfg.IodinPath)
		return errors.Annotatef(x.err, "config: iodin_path=%s", cfg.IodinPath)
	})
	return x.client, x.err
}

func (g *Global) Mdb() (*mdb.Bus, error) {
	x := &g.Hardware.Mdb // short alias
	_ = x.do(func() error {
		if x.Bus != nil { // state-new testing mode
			return nil
		}

		switch g.Config.Hardware.Mdb.UartDriver {
		case "file":
			x.Uarter = mdb_client.NewFileUart(g.Log)

		case "mega":
			mc, err := g.Mega()
			if mc == nil && err == nil { // FIXME
				err = errors.Errorf("code error mega=nil")
			}
			if err != nil {
				return errors.Annotate(x.err, "Mdber() driver=mega")
			}
			x.Uarter = mdb_client.NewMegaUart(mc)

		case "iodin":
			iodin, err := g.Iodin()
			if iodin == nil && err == nil { // FIXME
				err = errors.Errorf("code error iodin=nil")
			}
			if err != nil {
				return errors.Annotate(err, "Mdber() driver=iodin")
			}
			x.Uarter = mdb_client.NewIodinUart(iodin)

		default:
			return fmt.Errorf("config: unknown mdb.uart_driver=\"%s\" valid: file, mega, iodin", g.Config.Hardware.Mdb.UartDriver)
		}

		mdbLog := g.Log.Clone(log2.LOG_INFO)
		if g.Config.Hardware.Mdb.LogDebug {
			mdbLog.SetLevel(log2.LOG_DEBUG)
		}
		if err := x.Uarter.Open(g.Config.Hardware.Mdb.UartDevice); err != nil {
			return errors.Annotatef(err, "config: mdb=%v", g.Config.Hardware.Mdb)
		}
		x.Bus = mdb.NewBus(x.Uarter, mdbLog, g.Tele.Error)
		return nil
	})
	return x.Bus, x.err
}

func (g *Global) Mega() (*mega.Client, error) {
	x := &g.Hardware.mega
	_ = x.do(func() error {
		devConfig := &g.Config.Hardware.Mega
		megaConfig := &mega.Config{
			SpiBus:        devConfig.Spi,
			SpiSpeed:      devConfig.SpiSpeed,
			NotifyPinChip: devConfig.PinChip,
			NotifyPinName: devConfig.Pin,
		}
		log := g.Log.Clone(log2.LOG_INFO)
		if devConfig.LogDebug {
			log.SetLevel(log2.LOG_DEBUG)
		}
		x.client, x.err = mega.NewClient(megaConfig, log)
		return errors.Annotatef(x.err, "mega config=%#v", megaConfig)
	})
	return x.client, x.err
}

func (g *Global) MustTextDisplay() *text_display.TextDisplay {
	d, err := g.TextDisplay()
	if err != nil {
		g.Log.Fatal(err)
	}
	if d == nil {
		g.Log.Fatal("text display is not available")
	}
	return d
}

func (g *Global) TextDisplay() (*text_display.TextDisplay, error) {
	x := &g.Hardware.HD44780
	_ = x.do(func() error {
		if x.Display != nil { // state-new testing mode
			return nil
		}

		devConfig := &g.Config.Hardware.HD44780
		if !devConfig.Enable {
			g.Log.Infof("text display hd44780 is disabled")
			return nil
		}

		devWrap := new(hd44780.LCD)
		if err := devWrap.Init(devConfig.PinChip, devConfig.Pinmap, devConfig.Page1); err != nil {
			err = errors.Annotatef(err, "hd44780.Init config=%#v", devConfig)
			return err
		}
		ctrl := hd44780.ControlOn
		if devConfig.ControlBlink {
			ctrl |= hd44780.ControlBlink
		}
		if devConfig.ControlCursor {
			ctrl |= hd44780.ControlUnderscore
		}
		devWrap.SetControl(ctrl)
		x.Device = devWrap

		displayConfig := &text_display.TextDisplayConfig{
			Width:       uint32(devConfig.Width),
			Codepage:    devConfig.Codepage,
			ScrollDelay: time.Duration(devConfig.ScrollDelay) * time.Millisecond,
		}
		disp, err := text_display.NewTextDisplay(displayConfig)
		if err != nil {
			return errors.Annotatef(err, "NewTextDisplay config=%#v", displayConfig)
		}
		x.Display = disp
		x.Display.SetDevice(devWrap)
		go x.Display.Run()
		return nil
	})
	return x.Display, x.err
}

// Reference registered, inited device
func (g *Global) GetDevice(name string) (types.Devicer, error) {
	d, ok, err := g.getDevice(name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.NotFoundf("device=%s", name)
	}

	d.RLock()
	defer d.RUnlock()
	if d.dev == nil {
		err = errors.Errorf("code error device=%s is not registered", name)
		g.Fatal(err)
	}

	return d.dev, nil
}

func (g *Global) GetDeviceConfig(name string) (config_global.DeviceConfig, error) {
	d, ok, err := g.getDevice(name)
	if err != nil {
		return config_global.DeviceConfig{}, err
	}
	if !ok {
		return config_global.DeviceConfig{}, errors.NotFoundf("device=%s", name)
	}

	d.RLock()
	defer d.RUnlock()
	return d.config, nil
}

// Drivers call RegisterDevice to declare device support.
// probe is called only for devices enabled in config.
func (g *Global) RegisterDevice(name string, dev types.Devicer, probe func() error) error {
	d, ok, err := g.getDevice(name)
	g.Log.Debugf("RegisterDevice name=%s ok=%t err=%v", name, ok, err)
	if err != nil {
		return err
	}
	if !ok {
		// device is not listed in config
		return nil
	}

	d.Lock()
	defer d.Unlock()
	d.dev = dev

	err = probe()
	err = errors.Annotatef(err, "probe device=%s required=%t", name, d.config.Required)
	g.Error(err)
	// TODO err=offline + Required=false -> return nil
	if !d.config.Required {
		return nil
	}
	return err
}

func (g *Global) CheckDevices() error {
	if err := g.initDevices(); err != nil {
		return err
	}
	x := &g.Hardware.devices
	x.Lock()
	defer x.Unlock()
	errs := make([]error, 0, len(x.m))
	for _, d := range x.m {
		d.RLock()
		ok := d.dev != nil
		d.RUnlock()
		if !ok {
			errs = append(errs, fmt.Errorf("unknown device=%s in config (no driver)", d.config.Name))
		}
	}
	return helpers.FoldErrors(errs)
}

func (g *Global) getDevice(name string) (*devWrap, bool, error) {
	if err := g.initDevices(); err != nil {
		return nil, false, err
	}
	g.Hardware.devices.Lock()
	d, ok := g.Hardware.devices.m[name]
	g.Hardware.devices.Unlock()
	return d, ok, nil
}

func (g *Global) initDevices() error {
	x := &g.Hardware.devices
	return x.do(func() error {
		g.Log.Debugf("initDevices")

		errs := make([]error, 0, len(g.Config.Hardware.EvendDevices))
		x.m = make(map[string]*devWrap)
		for _, d := range g.Config.Hardware.EvendDevices {
			if d.Disabled {
				continue
			}
			if d.Name == "" {
				errs = append(errs, errors.Errorf("invalid device name=%s", d.Name))
				continue
			}
			if _, ok := x.m[d.Name]; ok {
				errs = append(errs, errors.Errorf("duplicate device name=%s", d.Name))
				continue
			}

			x.m[d.Name] = &devWrap{config: d}
		}

		err := helpers.FoldErrors(errs)
		g.Error(err)
		return err
	})
}

func (g *Global) initInput() {
	g.Hardware.Input = &input.Dispatch{
		Log: g.Log,
		Bus: make(chan types.InputEvent),
	}

	// read key event from evend keyboard
	srcEvendKbd, err := g.initInputEvendKeyboard()
	if srcEvendKbd != nil && err == nil {
		go g.Hardware.Input.ReadEvendKeyboard(srcEvendKbd)
	} else {
		g.Log.Errorf("evend keyboard not work. sourse(%v) error(%v)", srcEvendKbd, err)
	}

	// read service key
	srcServiceKey, err := input.NewDevInputEventSource(g.Config.Hardware.Input.ServiceKey)
	if err == nil {
		go g.Hardware.Input.ReadEvendKeyboard(srcServiceKey)
	} else {
		g.Log.Errorf("service button not work. error(%v)", err)
	}
}

func (g *Global) initInputEvendKeyboard() (input.Source, error) {
	const tag = input.EvendKeyboardSourceTag
	if !g.Config.Hardware.Input.EvendKeyboard.Enable {
		g.Log.Infof("input=%s disabled", tag)
		return nil, nil
	}

	mc, err := g.Mega()
	if err != nil {
		err = errors.Annotatef(err, "input=%s", tag)
		err = errors.Annotatef(err, "config: evend keyboard needs mega")
		return nil, err
	}
	ekb := input.NewEvendKeyboard(mc)
	return ekb, nil
}

type once struct {
	sync.Mutex
	called uint32 // atomic bool
	err    error
}

func (o *once) done() bool {
	return atomic.LoadUint32(&o.called) == 1
}

func (o *once) do(f func() error) error {
	if o.done() { // fast path
		return o.err
	}
	o.Lock()
	defer o.Unlock()
	if o.done() {
		return o.err
	}
	o.err = f()
	atomic.StoreUint32(&o.called, 1)
	return o.err
}
