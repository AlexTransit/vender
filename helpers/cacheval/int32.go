// Atomic value with validity timeout.
// Value and "modified" timestamp are stored together in a single atomic word,
// so readers always observe a consistent (value, timestamp) pair.
// Usage scenario examples: DNS resolve, sensor reading.
// All methods except `Init` are thread-safe.
package cacheval

import (
	"sync/atomic"
	"time"

	"github.com/temoto/atomic_clock"
)

// Packed state word layout (64 bit):
// bits 0..31: value as uint32 (int32 reinterpreted)
// bits 32..63: monotonic timestamp source() as uint32 (relative to package epoch)
const (
	tsShift = 32
	tsMask  = uint64(^uint32(0)) << tsShift // mask of timestamp bits
	// max representable `valid` duration: timestamps wrap every ~49 days
	maxValid = time.Duration(^uint32(0)) * time.Nanosecond
)

type Int32 struct {
	state atomic.Uint64 // packed (timestamp<<32 | value), zero == never set
	valid time.Duration
}

// Not thread-safe. `valid` duration cannot be changed later.
func (c *Int32) Init(valid time.Duration) {
	if valid < 0 {
		panic("cacheval: negative valid duration")
	}
	if valid > maxValid {
		panic("cacheval: valid duration too large")
	}
	c.valid = valid
	c.state.Store(0)
}

func (c *Int32) get() (int32, bool) {
	s := c.state.Load()
	if s == 0 {
		return 0, false // never Set()
	}
	age := time.Duration(atomic_clock.Source()-int64(s>>tsShift)) * time.Nanosecond
	return int32(uint32(s)), age >= 0 && age <= c.valid
}

// Returns current (possibly stale) value. Fast and cheap.
func (c *Int32) Get() int32 { return int32(uint32(c.state.Load())) }

// Returns current value and true if it's fresh. Costs current timestamp lookup.
func (c *Int32) GetFresh() (int32, bool) { return c.get() }

// Always returns fresh value.
// If value is stale, runs `f()`.
// It is `f()` responsibility to update value with `Set()` method.
// No cache stampede guard.
// May return value from concurrent `GetOrUpdate` or `Set`.
// Costs current timestamp lookup.
func (c *Int32) GetOrUpdate(f func()) int32 {
	v, ok := c.get()
	if !ok {
		f()
		v, _ = c.get()
	}
	return v
}

// Updates value and modified timestamp.
// Both value and timestamp are updated atomically as a single consistent unit:
// readers never observe a timestamp without the corresponding value.
// Costs current timestamp lookup.
func (c *Int32) Set(v int32) {
	ts := uint64(uint32(atomic_clock.Source())) << tsShift
	for {
		old := c.state.Load()
		packed := ts | uint64(uint32(v))
		if old&tsMask > ts {
			// preserve newer timestamp from a concurrent Set
			packed = old&tsMask | uint64(uint32(v))
		}
		if c.state.CompareAndSwap(old, packed) {
			return
		}
	}
}
