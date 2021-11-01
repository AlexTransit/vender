// Public API to easy create MDB stubs for test code.

package mdb

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
)

const MockTimeout = 5 * time.Second

type MockR [2]string

func (m MockR) String() string {
	return fmt.Sprintf("expect=%s response=%s", m[0], m[1])
}

type MockUart struct {
	t  testing.TB
	mu sync.Mutex
	m  map[string]string
	q  chan MockR
}

func NewMockUart(t testing.TB) *MockUart {
	self := &MockUart{
		t: t,
		q: make(chan MockR),
	}
	return self
}

func (mu *MockUart) Open(path string) error { return nil }
func (mu *MockUart) Close() error {
	mu.mu.Lock()
	defer mu.mu.Unlock()
	select {
	case _, ok := <-mu.q:
		err := errors.Errorf("mdb-mock: Close() with non-empty queue")
		if !ok {
			err = errors.Errorf("code error mdb-mock already closed")
		}
		// panic(err)
		// self.t.Log(err)
		mu.t.Fatal(err)
		return err
	default:
		close(mu.q)
		return nil
	}
}

func (mu *MockUart) Break(d, sleep time.Duration) error {
	runtime.Gosched()
	return nil
}

func (mu *MockUart) Tx(request, response []byte) (n int, err error) {
	mu.t.Helper()
	mu.mu.Lock()
	defer mu.mu.Unlock()
	if mu.m != nil {
		if len(mu.m) == 0 {
			err = errors.Errorf("mdb-mock: map ended, received=%x", request)
			mu.t.Error(err)
			return 0, err
		}
		return mu.txMap(request, response)
	}
	return mu.txQueue(request, response)
}

// ExpectMap() in random order
func (mu *MockUart) txMap(request, response []byte) (int, error) {
	requestHex := hex.EncodeToString(request)
	responseHex, found := mu.m[requestHex]
	if !found {
		// must not call self.t.Error() here
		return 0, ErrTimeout
	}
	delete(mu.m, requestHex)
	rp := MustPacketFromHex(responseHex, true)
	n := copy(response, rp.Bytes())
	return n, nil
}

// Expect() requests in defined order
func (mu *MockUart) txQueue(request, response []byte) (n int, err error) {
	var rr MockR
	var ok bool
	select {
	case rr, ok = <-mu.q:
		if !ok {
			err = errors.Errorf("mdb-mock: queue ended, received=%x", request)
			mu.t.Error(err)
			return 0, err
		}
	case <-time.After(MockTimeout):
		err = errors.Errorf("mdb-mock: queue timeout, received=%x", request)
		mu.t.Error(err)
		return 0, err
	}
	expect := MustPacketFromHex(rr[0], true)

	if !bytes.Equal(request, expect.Bytes()) {
		err = errors.Errorf("mdb-mock: request expected=%x actual=%x", expect.Bytes(), request)
		mu.t.Error(err)
		return 0, err
	}

	// TODO support testing errors
	// if rr.Rerr != nil {
	// 	self.t.Logf("mdb-mock: Tx returns error=%v", rr.Rerr)
	// 	return 0, rr.Rerr
	// }

	rp := MustPacketFromHex(rr[1], true)
	n = copy(response, rp.Bytes())
	return n, err
}

// usage:
// m, mock:= NewTestMdber(t)
// defer mock.Close()
// go use_mdb(m)
// mock.Expect(...)
// go use_mdb(m)
// mock.Expect(...)
// wait use_mdb() to finish to catch all possible errors
func (mu *MockUart) Expect(rrs []MockR) {
	mu.t.Helper()

	for _, rr := range rrs {
		select {
		case mu.q <- rr:
		case <-time.After(MockTimeout):
			err := errors.Errorf("mdb-mock: background processing is too slow, timeout sending into mock queue rr=%s", rr)
			mu.t.Fatal(err)
		}
	}
}

func (mu *MockUart) ExpectMap(rrs map[string]string) {
	mu.t.Helper()
	mu.mu.Lock()
	if rrs == nil {
		mu.m = nil
	} else {
		mu.m = make(map[string]string)
		for k := range rrs {
			mu.m[k] = rrs[k]
		}
	}
	mu.mu.Unlock()
}

func NewMockBus(t testing.TB) (*Bus, *MockUart) {
	mock := NewMockUart(t)
	b := NewBus(mock, log2.NewTest(t, log2.LDebug), func(e error) {
		t.Logf("bus.Error: %v", e)
	})
	return b, mock
}

const MockContextKey = "test/mdb-mock"

// sorry for this ugly convolution
// working around import cycle on a time budget
func MockFromContext(ctx context.Context) *MockUart {
	return ctx.Value(MockContextKey).(*MockUart)
}
