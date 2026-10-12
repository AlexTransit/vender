package cacheval

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestInt32ConsistentPair verifies the documented invariant: readers must never
// observe a fresh timestamp paired with an older value ("modified timestamp
// updated after value, without consistency"). Regression test for the flapping
// "unexpected decrease" failure in TestInt32Stress.
func TestInt32ConsistentPair(t *testing.T) {
	t.Parallel()

	const valid = 20 * time.Millisecond
	cv := Int32{}
	cv.Init(valid)

	// pre-seed so that every observed state has a timestamp
	cv.Set(1)

	stop := atomic.Bool{}
	var wg sync.WaitGroup

	// writers: monotonically increasing values, no sleeps -> maximum contention.
	// single writer keeps the "value never decreases" invariant well-defined;
	// multiple concurrent writers may commit out of order, which is allowed by
	// the API contract and not the consistency bug under test.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := int32(2); !stop.Load(); i++ {
			cv.Set(i)
		}
	}()

	inconsistency := atomic.Int64{}
	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lastVal := int32(0)
			lastTsFresh := false
			for !stop.Load() {
				v, ok := cv.GetFresh()
				if ok {
					// fresh value must never go backwards relative to the
					// previous fresh observation
					if v < lastVal {
						inconsistency.Add(1)
					}
					lastVal = v
					lastTsFresh = true
				} else if lastTsFresh {
					// stale right after fresh is normal (timeout expiry), not an error
					lastTsFresh = false
				}
			}
		}()
	}

	time.Sleep(300 * time.Millisecond)
	stop.Store(true)
	wg.Wait()

	if n := inconsistency.Load(); n > 0 {
		t.Errorf("observed %d inconsistent (fresh,decreased-value) reads", n)
	}
}
