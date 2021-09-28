package ui

// UI lock allows dynamic override of UI state workflow.
// Use cases:
// - graceful shutdown waits until user interaction is complete
// - remote management shows users machine is not available and prevents interaction

import (
	"sync/atomic"
	"time"

	tele_api "github.com/AlexTransit/vender/tele"
)

const lockPoll = 300 * time.Millisecond

type uiLock struct {
	ch   chan struct{}
	pri  uint32
	sem  int32
	next State
}

func (ui *UI) LockFunc(pri tele_api.Priority, fun func()) bool {
	if !ui.LockWait(pri) {
		return false
	}
	defer ui.LockDecrementWait()
	fun()
	return true
}

func (ui *UI) LockWait(pri tele_api.Priority) bool {
	ui.g.Log.Debugf("LockWait")
	newSem := atomic.AddInt32(&ui.lock.sem, 1)
	oldPri := ui.lock.priority()
	if newSem == 1 || (newSem > 1 && pri != oldPri && pri == tele_api.Priority_Now) {
		atomic.StoreUint32(&ui.lock.pri, uint32(pri))
	}
	select {
	case ui.lock.ch <- struct{}{}:
	default:
	}
	for ui.g.Alive.IsRunning() {
		time.Sleep(lockPoll)
		if ui.State() == StateLocked {
			return true
		}
	}
	return false
}

func (ui *UI) LockDecrementWait() {
	ui.g.Log.Debugf("LockDecrementWait")
	new := atomic.AddInt32(&ui.lock.sem, -1)
	if new < 0 {
		// Support concurrent LockEnd
		atomic.StoreInt32(&ui.lock.sem, 0)
		new = 0
	}
	if new == 0 {
		for ui.g.Alive.IsRunning() && (ui.State() == StateLocked) {
			time.Sleep(lockPoll)
		}
	}
}

// LockEnd Stop locked state ignoring call balance
func (ui *UI) LockEnd() {
	ui.g.Log.Debugf("LockEnd")
	atomic.StoreInt32(&ui.lock.sem, 0)
	for ui.g.Alive.IsRunning() && (ui.State() == StateLocked) {
		time.Sleep(lockPoll)
	}
}

func (ui *UI) checkInterrupt(s State) bool {
	if !ui.lock.locked() {
		return false
	}

	interrupt := true
	if ui.lock.priority()&tele_api.Priority_IdleUser != 0 {
		interrupt = !(s > StateFrontBegin && s < StateFrontEnd) &&
			!(s >= StateServiceBegin && s <= StateServiceEnd)
	}
	return interrupt
}

func (l *uiLock) locked() bool                { return atomic.LoadInt32(&l.sem) > 0 }
func (l *uiLock) priority() tele_api.Priority { return tele_api.Priority(atomic.LoadUint32(&l.pri)) }
