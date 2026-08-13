package ui

import (
	"sync"
	"time"
)

// how long a remote order may wait to be picked up by the state machine
const orderHandoverTimeout = 30 * time.Second

// remoteOrderGuard serializes orders coming from the server: at most one order
// may be in flight, and a message identical to the previous one is ignored.
//
// RU: MQTT (QoS 1) и ретраи сервера могут доставить makeOrder повторно.
// без защитника дубль вешает второй EventAccept на небуферизованный канал,
// и напиток готовится второй раз за те же деньги.
type remoteOrderGuard struct {
	mu       sync.Mutex
	inFlight bool
	pending  bool // reserved, not yet taken out of the event channel
	lastKey  string
}

// begin reserves the single order slot. returned reason is empty on success.
func (g *remoteOrderGuard) begin(key string) (reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inFlight {
		return "another order is already in progress"
	}
	if key != "" && key == g.lastKey {
		return "duplicate of the previous order"
	}
	g.inFlight = true
	g.pending = true
	g.lastKey = key
	return ""
}

// handedOver marks the order as taken out of the event channel. called for every
// received accept event, including the ones a state drops on the floor.
func (g *remoteOrderGuard) handedOver() {
	g.mu.Lock()
	g.pending = false
	g.mu.Unlock()
}

// end frees the slot when cooking is over, however it ended. an order still
// waiting to be picked up is kept, otherwise a second one could be queued
// behind it. safe to call when no order is in flight.
func (g *remoteOrderGuard) end() {
	g.mu.Lock()
	if !g.pending {
		g.inFlight = false
	}
	g.mu.Unlock()
}

// abort frees the slot when the order never reached the state machine.
func (g *remoteOrderGuard) abort() {
	g.mu.Lock()
	g.pending = false
	g.inFlight = false
	g.mu.Unlock()
}
