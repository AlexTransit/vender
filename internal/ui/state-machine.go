package ui

import (
	"context"
	"os"
	"sync/atomic"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/juju/errors"

	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/internal/watchdog"

	tele_api "github.com/AlexTransit/vender/tele"
)

//go:generate stringer -type=State -trimprefix=State
type State uint32

const (
	StateDefault State = iota

	StateBoot   // 1 t=onstart +onstartOk=FrontHello +onstartError+retry=Boot +retryMax=Broken
	StateBroken // 2 t=tele/input +inputService=ServiceBegin
	StateLocked // 3 t=tele

	StateFrontBegin   // 4 t=checkVariables +=FrontHello
	StateFrontSelect  // 5 t=input/money/timeout +inputService=ServiceBegin +input=... +money=... +inputAccept=FrontAccept +timeout=FrontTimeout
	StatePrepare      // 6
	StateFrontTune    // 7 t=input/money/timeout +inputTune=FrontTune ->FrontSelect
	StateFrontAccept  // 8 t=engine.Exec(Item) +OK=FrontEnd +err=Broken
	StateFrontTimeout // 9 t=saveMoney ->FrontEnd
	StateFrontEnd     // 10 ->FrontBegin

	StateServiceBegin     // 11 t=input/timeout ->ServiceAuth
	StateServiceAuth      // 12 +inputAccept+OK=ServiceMenu
	StateServiceMenu      //13
	StateServiceInventory //14
	StateServiceTest
	StateServiceReboot
	StateServiceNetwork
	StateServiceMoneyLoad
	StateServiceReport
	StateServiceEnd // 20 +askReport=ServiceReport ->FrontBegin

	StateStop //21

	StateFrontLock

	StateDoesNotChange
)

func (ui *UI) State() State               { return State(atomic.LoadUint32((*uint32)(&ui.state))) }
func (ui *UI) setState(new State)         { atomic.StoreUint32((*uint32)(&ui.state), uint32(new)) }
func (ui *UI) XXX_testSetState(new State) { ui.setState(new) }

func (ui *UI) Loop(ctx context.Context) {
	ui.g.Alive.Add(1)
	defer ui.g.Alive.Done()
	next := StateDefault
	for next != StateStop && ui.g.Alive.IsRunning() {
		current := ui.State()
		types.VMC.UiState = uint32(current)
		next = ui.enter(ctx, current)
		if next == StateDefault {
			ui.g.Log.Fatalf("ui state=%v next=default", current)
		}
		ui.exit(ctx, current, next)

		if current != StateLocked && ui.checkInterrupt(next) {
			ui.lock.next = next
			ui.g.Log.Infof("ui lock interrupt")
			next = StateLocked
		}

		if !ui.g.Alive.IsRunning() {
			ui.g.Log.Debugf("ui Loop stopping because g.Alive")
			next = StateStop
		}

		ui.setState(next)
		if ui.XXX_testHook != nil {
			ui.XXX_testHook(next)
		}
	}
	ui.g.Log.Debugf("ui loop end")
}

func (ui *UI) enter(ctx context.Context, s State) State {
	// ui.g.Log.Debugf("ui enter %s", s.String())
	switch s {
	case StateBoot:
		ui.g.Tele.RoboSendState(tele_api.State_Boot)
		ui.g.ShowPicture(state.PictureBoot)
		watchdog.WatchDogEnable()

		onBootScript := ui.g.Config.Engine.OnBoot
		if types.FirstInit() {
			onBootScript = append(ui.g.Config.Engine.FirstInit, onBootScript[:]...)
		}
		if errs := ui.g.Engine.ExecList(ctx, "on_boot", onBootScript); len(errs) != 0 {
			ui.g.Tele.Error(errors.Annotatef(helpers.FoldErrors(errs), "on_boot "))
			ui.g.Log.Error(errs)
			return StateBroken
		}
		if err := os.MkdirAll("/run/vender", 0700); err != nil {
			ui.g.Tele.Error(errors.Annotatef(err, "create vender folder"))
		}
		ui.broken = false
		ui.g.Tele.RoboSendState(tele_api.State_Nominal)
		return StateFrontBegin

	case StateBroken:
		watchdog.WatchDogDisable()
		types.InitRequared()
		ui.g.Log.Infof("state=broken")
		ui.g.ShowPicture(state.PictureBroken)
		if !ui.broken {
			// ui.g.Tele.RoboSendState(tele_api.State_Broken)
			if errs := ui.g.Engine.ExecList(ctx, "on_broken", ui.g.Config.Engine.OnBroken); len(errs) != 0 {
				// TODO maybe ErrorStack should be removed
				ui.g.Log.Error(errors.ErrorStack(errors.Annotate(helpers.FoldErrors(errs), "on_broken")))
			}
			moneysys := money.GetGlobal(ctx)
			_ = moneysys.SetAcceptMax(ctx, 0)
		}
		ui.broken = true
		ui.display.SetLines(ui.g.Config.UI.Front.MsgBrokenL1, ui.g.Config.UI.Front.MsgBrokenL2)
		for ui.g.Alive.IsRunning() {
			e := ui.wait(time.Second)
			// TODO receive tele command to reboot or change state
			if e.Kind == types.EventService {
				return StateServiceBegin
			}
		}
		return StateDefault

	case StateLocked:
		ui.display.SetLines(ui.g.Config.UI.Front.MsgStateLocked, "")
		// ui.g.Tele.State(tele_api.State_Lock)
		for ui.g.Alive.IsRunning() {
			e := ui.wait(lockPoll)
			// TODO receive tele command to reboot or change state
			if e.Kind == types.EventService {
				return StateServiceBegin
			}
			if !ui.lock.locked() {
				return ui.lock.next
			}
		}
		return StateDefault

	case StateFrontBegin:
		ui.inputBuf = ui.inputBuf[:0]
		ui.broken = false
		watchdog.WatchDogEnable()
		return ui.onFrontBegin(ctx)

	case StateFrontSelect:
		return ui.onFrontSelect(ctx)

	case StateFrontTune:
		return ui.onFrontTune(ctx)

	case StateFrontAccept:
		return ui.onFrontAccept(ctx)

	case StateFrontTimeout:
		return ui.onFrontTimeout(ctx)

	case StateFrontEnd:
		// ui.onFrontEnd(ctx)
		return StateFrontBegin

	case StateFrontLock:
		return ui.onFrontLock()

	case StateServiceBegin:
		watchdog.WatchDogDisable()
		return ui.onServiceBegin(ctx)
	case StateServiceMenu:
		return ui.onServiceMenu()
	case StateServiceInventory:
		return ui.onServiceInventory()
	case StateServiceTest:
		return ui.onServiceTest(ctx)
	case StateServiceReboot:
		return ui.onServiceReboot(ctx)
	case StateServiceNetwork:
		return ui.onServiceNetwork()
	case StateServiceMoneyLoad:
		return ui.onServiceMoneyLoad(ctx)
	case StateServiceReport:
		return ui.onServiceReport(ctx)
	case StateServiceEnd:
		return replaceDefault(ui.onServiceEnd(ctx), StateFrontBegin)

	case StateStop:
		return StateStop

	default:
		ui.g.Log.Fatalf("unhandled ui state=%v", s)
		return StateDefault
	}
}

func (ui *UI) exit(ctx context.Context, current, next State) {
	// ui.g.Log.Debugf("ui exit %s -> %s", current.String(), next.String())

	if next != StateBroken {
		ui.broken = false
	}
}

func replaceDefault(s, def State) State {
	if s == StateDefault {
		return def
	}
	return s
}

// func filterErrors(errs []error, take func(error) bool) []error {
// 	if len(errs) == 0 {
// 		return nil
// 	}
// 	new := errs[:0]
// 	for _, e := range errs {
// 		if e != nil && take(e) {
// 			new = append(new, e)
// 		}
// 	}
// 	for i := len(new); i < len(errs); i++ {
// 		errs[i] = nil
// 	}
// 	return new
// }

// func removeOptionalOffline(g *state.Global, errs []error) []error {
// 	take := func(e error) bool {
// 		if errOffline, ok := errors.Cause(e).(types.DeviceOfflineError); ok {
// 			if devconf, err := g.GetDeviceConfig(errOffline.Device.Name()); err == nil {
// 				return devconf.Required
// 			}
// 		}
// 		return true
// 	}
// 	return filterErrors(errs, take)
// }

// func executeScript(ctx context.Context, onstate string, data string) {
// 	g := state.GetGlobal(ctx)
// 	g.Log.Debugf("execute script (%s)", onstate)
// 	if g.Config.Engine.Profile.StateScript != "" {
// 		cmd := exec.Command(g.Config.Engine.Profile.StateScript) //nolint:gosec
// 		cmd.Env = []string{
// 			fmt.Sprintf("state=%s", onstate),
// 			fmt.Sprintf("data=%s", data),
// 		}
// 		g.Alive.Add(1)
// 		go func() {
// 			defer g.Alive.Done()
// 			execOutput, execErr := cmd.CombinedOutput()
// 			prettyEnv := strings.Join(cmd.Env, " ")
// 			if execErr != nil {
// 				execErr = errors.Annotatef(execErr, "state_script %s (%s) output=%s", cmd.Path, prettyEnv, execOutput)
// 				g.Log.Error(execErr)
// 			}
// 		}()
// 	}
// }
