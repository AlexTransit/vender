package state

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/internal/watchdog"
	"github.com/AlexTransit/vender/log2"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"
	"github.com/temoto/alive/v2"
)

type Global struct {
	Alive        *alive.Alive
	BuildVersion string
	Config       *Config
	Engine       *engine.Engine
	Hardware     hardware // hardware.go
	Inventory    *inventory.Inventory
	Log          *log2.Log
	Tele         tele_api.Teler
	LockCh       chan struct{}
	// TimerUIStop  chan struct{}
	// TODO UI           types.UIer

	XXX_money atomic.Value // *money.MoneySystem crutch to import cycle
	XXX_uier  atomic.Value // UIer crutch to import/init cycle

	// _copy_guard sync.Mutex //nolint:unused
}

const ContextKey = "run/state-global"

func GetGlobal(ctx context.Context) *Global {
	v := ctx.Value(ContextKey)
	if v == nil {
		panic(fmt.Sprintf("context['%s'] is nil", ContextKey))
	}
	if g, ok := v.(*Global); ok {
		return g
	}
	panic(fmt.Sprintf("context['%s'] expected type *Global actual=%#v", ContextKey, v))
}

type Pic uint32

const (
	PictureBoot Pic = iota
	PictureIdle
	PictureClient
	PictureMake
	PictureBroken
	PictureQRPayError
	PicturePayReject
)

func (g *Global) ShowPicture(pict Pic) {
	var file string
	switch {
	case pict == PictureBoot:
		file = g.Config.UI.Front.PicBoot
		types.VMC.HW.Display.Gdisplay = "PictureBoot"
	case pict == PictureClient:
		file = g.Config.UI.Front.PicClient
		types.VMC.HW.Display.Gdisplay = "PictureClient"
	case pict == PictureMake:
		file = g.Config.UI.Front.PicMake
		types.VMC.HW.Display.Gdisplay = "PictureMake"
	case pict == PictureBroken:
		file = g.Config.UI.Front.PicBroken
		types.VMC.HW.Display.Gdisplay = "PictureBroken"
	case pict == PictureQRPayError:
		file = g.Config.UI.Front.PicQRPayError
		types.VMC.HW.Display.Gdisplay = "PictureQRCreateerror"
	case pict == PicturePayReject:
		file = g.Config.UI.Front.PicPayReject
		types.VMC.HW.Display.Gdisplay = "PictureBankReject"
	default:
		types.VMC.HW.Display.Gdisplay = "PictureDefault"
	}
	if file == "" {
		file = g.Config.UI.Front.PicIdle
	}
	g.Hardware.Display.d.CopyFile2FB(file)

}

func (g *Global) VmcStop(ctx context.Context) {
	watchdog.WatchDogOff()
	g.ShowPicture(PictureBroken)
	g.Log.Infof("--- event vmc stop ---")
	go func() {
		time.Sleep(10 * time.Second)
		g.Log.Infof("--- vmc timeout EXIT ---")
		os.Exit(1)
	}()
	if types.VMC.UiState != 5 { // FixMe состояние ожидания
		types.InitRequared()
	}

	g.LockCh <- struct{}{}
	_ = g.Engine.ExecList(ctx, "on_broken", g.Config.Engine.OnBroken)
	td := g.MustTextDisplay()
	td.SetLines(g.Config.UI.Front.MsgBrokenL1, g.Config.UI.Front.MsgBrokenL2)
	g.Tele.Close()
	time.Sleep(2 * time.Second)
	g.Log.Infof("--- vmc stop ---")
	g.Stop()
	g.Alive.Done()
	os.Exit(1)
}

func (g *Global) ClientBegin() {
	g.ShowPicture(PictureClient)
	types.VMC.Client.Prepare = true
	if !types.VMC.Lock {
		// g.TimerUIStop <- struct{}{}
		types.VMC.Lock = true
		types.VMC.Client.WorkTime = time.Now()
		g.Log.Infof("--- client activity begin ---")
	}
	g.Tele.RoboSendState(tele_api.State_Client)
}

func (g *Global) ClientEnd() {
	types.VMC.EvendKeyboardInput(true)
	types.VMC.Client.Prepare = false
	if types.VMC.Lock {
		types.VMC.Lock = false
		types.VMC.Client.WorkTime = time.Now()
		g.Log.Infof("--- client activity end ---")
	}
	g.ShowPicture(PictureIdle)
}

// If `Init` fails, consider `Global` is in broken state.
func (g *Global) Init(ctx context.Context, cfg *Config) error {
	g.Config = cfg

	g.Log.Infof("build version=%s", g.BuildVersion)
	types.VMC.Version = g.BuildVersion

	if g.Config.Persist.Root == "" {
		g.Config.Persist.Root = "./tmp-vender-db"
		g.Log.Errorf("config: persist.root=empty changed=%s", g.Config.Persist.Root)
		// return errors.Errorf("config: persist.root=empty")
	}
	g.Log.Debugf("config: persist.root=%s", g.Config.Persist.Root)

	// Since tele is remote error reporting mechanism, it must be inited before anything else
	// Tele.Init gets g.Log clone before SetErrorFunc, so Tele.Log.Error doesn't recurse on itself
	if err := g.Tele.Init(ctx, g.Log.Clone(log2.LOG_INFO), g.Config.Tele, g.BuildVersion); err != nil {
		g.Tele = tele_api.Noop{}
		return errors.Annotate(err, "tele init")
	}
	types.TeleN = g.Tele
	g.Log.SetErrorFunc(g.Tele.Error)

	if g.BuildVersion == "unknown" {
		g.Error(fmt.Errorf("build version is not set, please use script/build"))
	} else if g.Config.Tele.VmId > 0 && strings.HasSuffix(g.BuildVersion, "-dirty") { // vmid<=0 is staging
		g.Error(fmt.Errorf("running development build with uncommited changes, bad idea for production"))
	}

	if g.Config.Money.Scale == 0 {
		g.Config.Money.Scale = 1
		g.Log.Errorf("config: money.scale is not set")
	} else if g.Config.Money.Scale < 0 {
		return errors.NotValidf("config: money.scale < 0")
	}
	g.Config.Money.CreditMax *= g.Config.Money.Scale
	g.Config.Money.ChangeOverCompensate *= g.Config.Money.Scale

	const initTasks = 4
	wg := sync.WaitGroup{}
	wg.Add(initTasks)
	errch := make(chan error, initTasks)

	// working term signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		g.Log.Infof("system signal - %v", sig)
		g.VmcStop(ctx)
	}()

	go helpers.WrapErrChan(&wg, errch, g.initDisplay)
	go helpers.WrapErrChan(&wg, errch, g.initInput)
	go helpers.WrapErrChan(&wg, errch, func() error { return g.initInventory(ctx) }) // storage read
	go helpers.WrapErrChan(&wg, errch, g.initEngine)
	// TODO init money system, load money state from storage
	g.RegisterCommands(ctx)
	wg.Wait()
	close(errch)

	// TODO engine.try-resolve-all-lazy after all other inits finished

	return helpers.FoldErrChan(errch)
}

func (g *Global) MustInit(ctx context.Context, cfg *Config) {
	err := g.Init(ctx, cfg)
	if err != nil {
		g.Fatal(err)
	}
}

func (g *Global) Error(err error, args ...interface{}) {
	if err != nil {
		if len(args) != 0 {
			msg := args[0].(string)
			args = args[1:]
			err = errors.Annotatef(err, msg, args...)
		}
		// g.Tele.Error(err)
		// эта бабуйня еще и в телеметрию отсылает
		g.Log.Error(err)
	}
}

func (g *Global) Fatal(err error, args ...interface{}) {
	if err != nil {
		g.Error(err, args...)
		g.StopWait(5 * time.Second)
		g.Log.Fatal(err)
		os.Exit(1)
	}
}

func (g *Global) ScheduleSync(ctx context.Context, fun types.TaskFunc) error {
	// TODO task := g.Schedule(ctx, priority, fun)
	// return task.wait()

	g.Alive.Add(1)
	defer g.Alive.Done()
	return fun(ctx)

	// switch priority {
	// case tele_api.Priority_Default, tele_api.Priority_Now:
	// 	return fun(ctx)

	// case tele_api.Priority_IdleEngine:
	// 	// TODO return g.Engine.Schedule(ctx, priority, fun)
	// 	return fun(ctx)

	// case tele_api.Priority_IdleUser:
	// 	return g.UI().ScheduleSync(ctx, priority, fun)

	// default:
	// 	return errors.Errorf("code error ScheduleSync invalid priority=(%d)%s", priority, priority.String())
	// }
}

func (g *Global) Stop() {
	g.Alive.Stop()
}

func (g *Global) StopWait(timeout time.Duration) bool {
	g.Alive.Stop()
	select {
	case <-g.Alive.WaitChan():
		return true
	case <-time.After(timeout):
		return false
	}
}

func (g *Global) UI() types.UIer {
	for {
		x := g.XXX_uier.Load()
		if x != nil {
			return x.(types.UIer)
		}
		g.Log.Errorf("CRITICAL g.uier is not set")
		time.Sleep(5 * time.Second)
	}
}

func (g *Global) initDisplay() error {
	d, err := g.Display()
	if d != nil {
		types.VMC.HW.Display.GdisplayValid = true
		g.ShowPicture(PictureBoot)
	}
	return err
}

func (g *Global) ShowQR(t string) {
	display, err := g.Display()
	if err != nil {
		g.Log.Error(err, "display")
		return
	}
	if display == nil {
		g.Log.Error("display is not configured")
		return
	}
	g.Log.Infof("show QR:'%v'", t)
	err = display.QR(t, true, 2)
	if err != nil {
		g.Log.Error(err, "QR show error")
	}
	types.VMC.HW.Display.Gdisplay = t
}

func (g *Global) initEngine() error {
	errs := make([]error, 0)

	for _, x := range g.Config.Engine.Aliases {
		var err error
		x.Doer, err = g.Engine.ParseText(x.Name, x.Scenario)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// g.Log.Debugf("config.engine.alias name=%s scenario=%s", x.Name, x.Scenario)
		g.Engine.Register(x.Name, x.Doer)
	}

	for _, x := range g.Config.Engine.Menu.Items {
		var err error
		x.Price = g.Config.ScaleI(x.XXX_Price)
		x.Doer, err = g.Engine.ParseText(x.Name, x.Scenario)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// g.Log.Debugf("config.engine.menu %s pxxx=%d ps=%d", x.String(), x.XXX_Price, x.Price)
		g.Engine.Register("menu."+x.Code, x.Doer)
	}

	if pcfg := g.Config.Engine.Profile; pcfg.Regexp != "" {
		if re, err := regexp.Compile(pcfg.Regexp); err != nil {
			errs = append(errs, err)
		} else {
			format := pcfg.LogFormat
			if format == "" {
				format = `engine profile action=%s time=%s`
			}
			min := time.Duration(pcfg.MinUs) * time.Microsecond
			g.Engine.SetProfile(re, min, func(d engine.Doer, td time.Duration) { g.Log.Debugf(format, d.String(), td) })
		}
	}

	return helpers.FoldErrors(errs)
}

func (g *Global) initInventory(ctx context.Context) error {
	// TODO ctx should be enough
	if err := g.Inventory.Init(ctx, &g.Config.Engine.Inventory, g.Engine, g.Config.Persist.Root); err != nil {
		return err
	}
	g.Inventory.InventoryLoad()
	return nil
}

func VmcLock(ctx context.Context) {
	g := GetGlobal(ctx)
	g.Log.Info("Vmc Locked")
	types.VMC.Lock = true
	types.VMC.EvendKeyboardInput(false)
	if types.VMC.UiState == 5 || types.VMC.UiState == 6 {
		g.LockCh <- struct{}{}
	}
}

func VmcUnLock(ctx context.Context) {
	g := GetGlobal(ctx)
	g.Log.Info("Vmc UnLocked")
	types.VMC.Lock = false
	types.VMC.EvendKeyboardInput(true)
	if types.VMC.UiState == 22 {
		g.LockCh <- struct{}{}
	}
}

func (g *Global) RegisterCommands(ctx context.Context) {
	g.Engine.RegisterNewFunc(
		"vmc.lock!",
		func(ctx context.Context) error {
			if !types.VMC.Lock {
				VmcLock(ctx)
			}
			return nil
		},
	)

	g.Engine.RegisterNewFunc(
		"vmc.unlock!",
		func(ctx context.Context) error {
			if types.VMC.Lock {
				VmcUnLock(ctx)
				// g.LockCh <- struct{}{}
			}
			return nil
		},
	)

	g.Engine.RegisterNewFunc(
		"vmc.stop!",
		func(ctx context.Context) error {
			g.VmcStop(ctx)
			return nil
		},
	)

	g.Engine.RegisterNewFunc(
		"envs.print",
		func(ctx context.Context) error {
			err := errors.Errorf(types.ShowEnvs())
			// AlexM надо бы сделать что бы слала как сообщение а не ошибку.
			return err
		},
	)

	doEmuKey := engine.FuncArg{
		// keys 0-9, 10 = C, 11 = Ok,
		// 12-13 cream- cream+, 14-15 sugar- sugar+, 16 dot
		Name: "emulate.key(?)",
		F: func(ctx context.Context, arg engine.Arg) error {
			var key uint16
			switch arg {
			case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9:
				key = uint16(arg) + 48
			case 10:
				key = 27
			case 11:
				key = 13
			case 12, 13, 14, 15:
				key = uint16(arg) + 53
			case 16:
				key = 46
			case 99:
				ev := types.InputEvent{Source: "dev-input-event", Key: types.InputKey(256), Up: true}
				g.Hardware.Input.Emit(ev)
				return nil
			}
			event := types.InputEvent{Source: "evend-keyboard", Key: types.InputKey(key), Up: true}
			g.Hardware.Input.Emit(event)
			return nil
		}}
	g.Engine.Register(doEmuKey.Name, doEmuKey)

}
