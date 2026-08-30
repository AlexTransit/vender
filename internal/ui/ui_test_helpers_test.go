package ui_test

import (
	"context"
	"testing"
	"time"

	"github.com/AlexTransit/vender/hardware/input"
	config_global "github.com/AlexTransit/vender/internal/config"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/internal/ui"
	"github.com/stretchr/testify/require"
)

type tenv struct {
	ctx     context.Context
	g       *state.Global
	ui      *ui.UI
	uiState chan types.UiState
}

func uiTestSetup(t *testing.T, env *tenv, begin, endState types.UiState) {
	t.Helper()

	require.NotNil(t, env)
	require.NotNil(t, env.ctx)
	require.NotNil(t, env.g)

	// NewTestContext intentionally only creates the basic Global/context.
	// Complete the parts required by the UI state machine here.
	env.g.Config.Inventory.File = t.TempDir() + "/inventory"
	env.g.Config.Hardware.Input.EvendKeyboard.Enable = false
	// Unit tests do not initialise the physical eVend valve.  The default
	// configuration enables its temperature check, which would otherwise use
	// the package-global valve with no MDB bus attached.
	env.g.Config.Hardware.Evend.Valve.TemperatureHot = 0
	env.g.Config.Hardware.HD44780.Enable = true
	env.g.Config.Hardware.HD44780.PinChip = "dummy"
	if env.g.Config.Hardware.HD44780.Width == 0 {
		env.g.Config.Hardware.HD44780.Width = 16
	}

	// UI code uses the package-global VMC configuration, as production code
	// does after config.ReadConfig().

	require.NoError(t, env.g.Init(env.ctx, env.g.Config))
	config_global.VMC = env.g.Config

	env.ui = new(ui.UI)
	require.NoError(t, env.ui.Init(env.ctx))

	env.ui.XXX_testSetState(begin)
	env.ui.XXX_testHook = func(s types.UiState) {
		select {
		case env.uiState <- s:
		default:
		}
		// ui.Loop runs until StateStop; without this, it cycles
		// FrontBegin->FrontSelect->...->FrontEnd->FrontBegin forever and the
		// final env.g.Alive.Wait() in tests blocks indefinitely.
		if s == endState {
			env.g.Alive.Stop()
		}
	}
}

func (e *tenv) emit(key types.InputKey) {
	e.g.Hardware.Input.Emit(types.InputEvent{
		Source: input.EvendKeyboardSourceTag,
		Key:    key,
	})
}

// emitMoneyAbort sends the money-abort event the way a real coin/bill
// validator does. input.IsMoneyAbort checks Source == MoneySourceTag, not
// just the key value — sending MoneyKeyAbort via emit() (EvendKeyboardSourceTag)
// would land in IsReject (backspace) instead, since IsReject only checks Key.
func (e *tenv) emitMoneyAbort() {
	e.g.Hardware.Input.Emit(types.InputEvent{
		Source: input.MoneySourceTag,
		Key:    input.MoneyKeyAbort,
	})
}

func (e *tenv) requireState(t *testing.T, want types.UiState) {
	t.Helper()

	const timeout = 5 * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		if e.ui.State() == want {
			return
		}

		select {
		case got := <-e.uiState:
			if got == want {
				return
			}
		case <-timer.C:
			require.Equal(t, want, e.ui.State(), "UI state timeout")
			return
		}
	}
}

func (e *tenv) requireDisplay(t *testing.T, line1, line2 string) {
	t.Helper()

	const timeout = 5 * time.Second
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		got1 := e.uiDisplayLine(1)
		got2 := e.uiDisplayLine(2)
		if got1 == line1 && got2 == line2 {
			return
		}

		select {
		case <-ticker.C:
		case <-deadline.C:
			require.Equal(t, line1, e.uiDisplayLine(1))
			require.Equal(t, line2, e.uiDisplayLine(2))
			return
		}
	}
}

func (e *tenv) uiDisplayLine(line int) string {
	return e.g.MustTextDisplay().GetLine(line)
}

func (e *tenv) _Key(key byte) types.InputKey {
	return types.InputKey(key)
}

var _KeyAccept = input.EvendKeyAccept
