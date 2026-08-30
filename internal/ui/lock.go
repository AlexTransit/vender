package ui

// UI lock allows dynamic override of UI state workflow.
// Use cases:
// - graceful shutdown waits until user interaction is complete
// - remote management shows users machine is not available and prevents interaction

import (
	"sync/atomic"
	"time"

	"github.com/AlexTransit/vender/internal/types"
)

const lockPoll = 300 * time.Millisecond

type uiLock struct {
	// ch   chan struct{}
	// pri  uint32
	sem  atomic.Int32
	next types.UiState
}

func (ui *UI) LockFunc(fun func()) bool {
	fun()
	return true
}

func (ui *UI) LockDecrementWait() {
	ui.g.Log.Debugf("LockDecrementWait")
	new := ui.lock.sem.Add(-1)
	if new < 0 {
		// Support concurrent LockEnd
		ui.lock.sem.Store(0)
		new = 0
	}
	if new == 0 {
		for ui.g.Alive.IsRunning() && (ui.State() == types.StateLocked) {
			time.Sleep(lockPoll)
		}
	}
}

// LockEnd Stop locked state ignoring call balance
func (ui *UI) LockEnd() {
	ui.g.Log.Debugf("LockEnd")
	ui.lock.sem.Store(0)
	for ui.g.Alive.IsRunning() && (ui.State() == types.StateLocked) {
		time.Sleep(lockPoll)
	}
}

// func (ui *UI) checkInterrupt(s types.UiState) bool {
// 	if !ui.lock.locked() {
// 		return false
// 	}

// 	interrupt := true
// 	// if ui.lock.priority()&tele_api.Priority_IdleUser != 0 {
// 	// 	interrupt = !(s > StateFrontBegin && s < StateFrontEnd) &&
// 	// 		!(s >= StateServiceBegin && s <= StateServiceEnd)
// 	// }
// 	return interrupt
// }

func (l *uiLock) locked() bool { return l.sem.Load() > 0 }

// func (l *uiLock) priority() tele_api.Priority { return tele_api.Priority(atomic.LoadUint32(&l.pri)) }
