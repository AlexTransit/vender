package ui

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"

	config_global "github.com/AlexTransit/vender/internal/config"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/internal/watchdog"
)

func (ui *UI) State() types.UiState               { return types.UiState(atomic.LoadUint32((*uint32)(&ui.state))) }
func (ui *UI) setState(new types.UiState)         { atomic.StoreUint32((*uint32)(&ui.state), uint32(new)) }
func (ui *UI) XXX_testSetState(new types.UiState) { ui.setState(new) }

func (ui *UI) Loop(ctx context.Context) {
	ui.g.Alive.Add(1)
	defer ui.g.Alive.Done()
	next := types.StateDefault
	for next != types.StateStop && ui.g.Alive.IsRunning() {
		current := ui.State()
		config_global.VMC.UIState(uint32(current))
		next = ui.enter(ctx, current)
		if next == types.StateDefault {
			ui.g.Log.Fatalf("ui state=%v next=default", current)
		}
		ui.exit(next)

		// if current != types.StateLocked && ui.checkInterrupt(next) {
		// 	ui.lock.next = next
		// 	ui.g.Log.Infof("ui lock interrupt")
		// 	next = types.StateLocked
		// }

		if !ui.g.Alive.IsRunning() {
			ui.g.Log.Debugf("ui Loop stopping because g.Alive")
			next = types.StateStop
		}

		ui.setState(next)
		if ui.XXX_testHook != nil {
			ui.XXX_testHook(next)
		}
	}
	ui.g.Log.Debugf("ui loop end")
}

func (ui *UI) enter(ctx context.Context, s types.UiState) types.UiState {
	// ui.g.Log.Debugf("ui enter %s", s.String())
	switch s {
	case types.StateBoot:
		watchdog.Enable()

		onBootScript := ui.g.Config.Engine.OnBoot
		if watchdog.ReinitRequired() {
			time.Sleep(3 * time.Second) // wait device init after reset
			onBootScript = append(ui.g.Config.Engine.FirstInit, onBootScript[:]...)
		}
		if errs := ui.g.Engine.ExecList(ctx, "on_boot", onBootScript); len(errs) != 0 {
			ui.g.Tele.Error(errors.Annotatef(helpers.FoldErrors(errs), "on_boot "))
			ui.g.Log.Error(errs)
			watchdog.SetBroken()
			return types.StateBroken
		}
		watchdog.SetDeviceInited()
		ui.broken = false
		return types.StateOnStart

	case types.StateOnStart:
		return ui.onFrontStart()

	case types.StateBroken:
		watchdog.Disable()
		watchdog.DevicesInitializationRequired()
		watchdog.SetBroken()
		ui.g.TeleCancelOrder(tele.State_Broken)

		ui.g.RunBashSript(ui.g.Config.ScriptIfBroken)
		ui.g.Log.Infof("state=broken")
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
		for ui.g.Alive.IsRunning() {
			e := ui.wait(time.Second)
			// TODO receive tele command to reboot or change state
			if e.Kind == types.EventService {
				return types.StateServiceBegin
			}
		}
		return types.StateDefault

	case types.StateLocked:
		for ui.g.Alive.IsRunning() {
			e := ui.wait(lockPoll)
			// TODO receive tele command to reboot or change state
			if e.Kind == types.EventService {
				return types.StateServiceBegin
			}
			if !ui.lock.locked() {
				return ui.lock.next
			}
		}
		return types.StateDefault

	case types.StateFrontBegin:
		ui.inputBuf = ui.inputBuf[:0]
		ui.broken = false
		watchdog.Enable()
		return ui.onFrontBegin(ctx)

	case types.StateFrontSelect:
		return ui.onFrontSelect(ctx)

	case types.StateFrontTune:
		return ui.onFrontTune(ctx)

	case types.StateFrontAccept:
		return ui.onFrontAccept(ctx)

	case types.StateFrontTimeout:
		return ui.onFrontTimeout(ctx)

	case types.StateFrontEnd:
		// ui.onFrontEnd(ctx)
		return types.StateFrontBegin

	case types.StateFrontLock:
		return ui.onFrontLock()

	case types.StateServiceBegin:
		return ui.onServiceBegin(ctx)
	case types.StateServiceMenu:
		return ui.onServiceMenu()
	case types.StateServiceInventory:
		return ui.onServiceInventory()
	case types.StateServiceTest:
		return ui.onServiceTest(ctx)
	case types.StateServiceReboot:
		return ui.onServiceReboot(ctx)
	case types.StateServiceNetwork:
		return ui.onServiceNetwork()
	case types.StateServiceMoneyLoad:
		return ui.onServiceMoneyLoad(ctx)
	case types.StateServiceReport:
		return ui.onServiceReport(ctx)
	case types.StateServiceEnd:
		watchdog.Enable()
		watchdog.UnsetBroken()
		return replaceDefault(ui.onServiceEnd(ctx), types.StateFrontBegin)

	case types.StateStop:
		return types.StateStop

	default:
		ui.g.Log.Fatalf("unhandled ui state=%v", s)
		return types.StateDefault
	}
}

func (ui *UI) exit(next types.UiState) {
	// ui.g.Log.Debugf("ui exit %s -> %s", current.String(), next.String())

	if next != types.StateBroken {
		ui.broken = false
	}
}

func replaceDefault(s, def types.UiState) types.UiState {
	if s == types.StateDefault {
		return def
	}
	return s
}
