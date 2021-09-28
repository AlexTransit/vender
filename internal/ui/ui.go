package ui

import (
	"context"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/input"
	"github.com/AlexTransit/vender/hardware/text_display"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	ui_config "github.com/AlexTransit/vender/internal/ui/config"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"
)

type UI struct { //nolint:maligned
	FrontMaxPrice currency.Amount
	// FrontResult   UIMenuResult
	Service uiService

	config *ui_config.Config
	g      *state.Global
	state  State
	broken bool
	// menu     Menu
	display  *text_display.TextDisplay // FIXME
	inputBuf []byte
	eventch  chan types.Event
	inputch  chan types.InputEvent
	lock     uiLock

	frontResetTimeout time.Duration

	XXX_testHook func(State)
}

var _ types.UIer = &UI{} // compile-time interface test

func (ui *UI) Init(ctx context.Context) error {
	// func (ui *types.UI) Init(ctx context.Context) error {
	ui.g = state.GetGlobal(ctx)
	ui.config = &ui.g.Config.UI
	ui.setState(StateBoot)

	// ui.menu = make(Menu)
	types.UI.Menu = make(map[string]types.MenuItemType)
	FillMenu(ctx)
	// if err := ui.menu.Init(ctx); err != nil {
	// 	err = errors.Annotate(err, "ui.menu.Init")
	// 	return err
	// }
	// ui.g.Log.Debugf("menu len=%d", len(ui.menu))
	ui.g.Log.Debugf("menu len=%d", len(types.UI.Menu))

	ui.display = ui.g.MustTextDisplay()
	ui.eventch = make(chan types.Event)
	ui.inputBuf = make([]byte, 0, 32)
	ui.inputch = ui.g.Hardware.Input.SubscribeChan("ui", ui.g.Alive.StopChan())
	// TODO ui.g.Hardware.Input.Unsubscribe("ui")

	ui.frontResetTimeout = helpers.IntSecondDefault(ui.g.Config.UI.Front.ResetTimeoutSec, 0)

	// ui.lock.ch = make(chan struct{}, 1)
	ui.g.LockCh = make(chan struct{}, 1)
	ui.g.TimerUIStop = make(chan struct{}, 1)
	ui.Service.Init(ctx)
	ui.g.XXX_uier.Store(types.UIer(ui)) // FIXME import cycle traded for pointer cycle
	return nil
}

func (ui *UI) ScheduleSync(ctx context.Context, priority tele_api.Priority, fun types.TaskFunc) error {
	if !ui.LockWait(priority) {
		return errors.Trace(types.ErrInterrupted)
	}
	defer ui.LockDecrementWait()
	return fun(ctx)
}

func (ui *UI) wait(timeout time.Duration) types.Event {
	tmr := time.NewTimer(timeout)
	defer tmr.Stop()
again:
	select {

	case <-ui.g.TimerUIStop:
		return types.Event{Kind: types.EventUiTimerStop}

	case e := <-ui.eventch:
		// if e.Kind != types.EventInvalid {
		// }
		return e

	case e := <-ui.inputch:
		// if e.Source != "" {
		// }
		if e.Source == input.DevInputEventTag && e.Up {
			return types.Event{Kind: types.EventService}
		}
		return types.Event{Kind: types.EventInput, Input: e}

	case <-ui.g.LockCh:
		return types.Event{Kind: types.EventFrontLock}

	case <-ui.lock.ch:
		// chan buffer may produce false positive
		if !ui.lock.locked() {
			goto again
		}
		return types.Event{Kind: types.EventLock}

	case <-tmr.C:
		return types.Event{Kind: types.EventTime}

	case <-ui.g.Alive.StopChan():
		return types.Event{Kind: types.EventStop}
	}
}
