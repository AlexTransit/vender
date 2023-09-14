package mdb

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
	"github.com/temoto/atomic_clock"
)

const ErrCodeNone int32 = -1

const (
	DefaultDelayAfterReset  = 500 * time.Millisecond
	DefaultDelayBeforeReset = 0
	DefaultDelayIdle        = 700 * time.Millisecond
	DefaultDelayNext        = 200 * time.Millisecond
	DefaultDelayOffline     = 10 * time.Second
	DefaultIdleThreshold    = 30 * time.Second
)

type Device struct { //nolint:maligned
	state   uint32 // atomic
	errCode int32  // atomic

	bus   *Bus
	cmdLk sync.Mutex // TODO explore if chan approach is better

	LastOk      *atomic_clock.Clock // last successful tx(), 0 at init, monotonic
	LastOff     *atomic_clock.Clock // last change from online to offline (MDB timeout), 0=online
	lastReset   *atomic_clock.Clock // last RESET attempt, 0 only at init, monotonic
	Log         *log2.Log
	Address     uint8
	name        string
	ByteOrder   binary.ByteOrder
	PacketReset Packet
	PacketSetup Packet
	PacketPoll  Packet
	Action      string
	DoReset     engine.Doer
	DoInit      engine.Doer // likely Seq starting with DoReset

	DelayAfterReset  time.Duration
	DelayBeforeReset time.Duration
	DelayIdle        time.Duration
	DelayNext        time.Duration
	DelayOffline     time.Duration
	IdleThreshold    time.Duration

	SetupResponse Packet
}

func (dev *Device) Init(bus *Bus, addr uint8, name string, byteOrder binary.ByteOrder) {
	dev.cmdLk.Lock()
	defer dev.cmdLk.Unlock()

	dev.Address = addr
	dev.ByteOrder = byteOrder
	dev.Log = bus.Log
	dev.bus = bus
	dev.name = name
	dev.errCode = ErrCodeNone
	dev.LastOk = atomic_clock.New()
	dev.LastOff = atomic_clock.Now()
	dev.lastReset = atomic_clock.New()

	if dev.DelayAfterReset == 0 {
		dev.DelayAfterReset = DefaultDelayAfterReset
	}
	if dev.DelayBeforeReset == 0 {
		dev.DelayBeforeReset = DefaultDelayBeforeReset
	}
	if dev.DelayIdle == 0 {
		dev.DelayIdle = DefaultDelayIdle
	}
	if dev.DelayNext == 0 {
		dev.DelayNext = DefaultDelayNext
	}
	if dev.DelayOffline == 0 {
		dev.DelayOffline = DefaultDelayOffline
	}
	if dev.IdleThreshold == 0 {
		dev.IdleThreshold = DefaultIdleThreshold
	}
	dev.SetupResponse = Packet{}
	dev.PacketReset = MustPacketFromBytes([]byte{dev.Address + 0}, true)
	dev.PacketSetup = MustPacketFromBytes([]byte{dev.Address + 1}, true)
	dev.PacketPoll = MustPacketFromBytes([]byte{dev.Address + 3}, true)
	dev.DoReset = engine.Func0{Name: fmt.Sprintf("%s.reset", dev.name), F: dev.Reset}
	dev.SetState(DeviceInited)

	if _, ok := bus.u.(*MockUart); ok {
		// testing
		dev.XXX_FIXME_SetAllDelays(1)
	}
}

func (dev *Device) Name() string { return dev.name }

func (dev *Device) TeleError(e error) { dev.bus.Error(e) }

func (dev *Device) ValidateErrorCode() error {
	value := atomic.LoadInt32(&dev.errCode)
	if value == ErrCodeNone {
		return nil
	}
	return errors.Errorf("%s unhandled errorcode=%d", dev.name, value)
}

func (dev *Device) ValidateOnline() error {
	st := dev.State()
	if st.Online() {
		return nil
	}
	return errors.Errorf("%s state=%s offline duration=%v", dev.name, st.String(), atomic_clock.Since(dev.LastOff))
}

// Command is known to be supported, MDB timeout means remote is offline.
// RESET if appropriate.
func (dev *Device) TxKnown(request Packet, response *Packet) error {
	dev.cmdLk.Lock()
	defer dev.cmdLk.Unlock()
	return dev.txKnown(request, response)
}

func (dev *Device) Tx(request Packet, response *Packet) error {
	dev.cmdLk.Lock()
	defer dev.cmdLk.Unlock()
	return dev.bus.Tx(request, response)
}

func (dev *Device) Rst() (err error) {
	dev.LastOff.SetNowIfZero() // consider device offline from now till successful response
	dev.lastReset.SetNow()
	dev.SetState(DeviceError)
	err = dev.Tx(dev.PacketReset, nil)
	time.Sleep(200 * time.Millisecond)
	if err == nil {
		err = dev.TxReadSetup()
		if dev.SetupResponse.l == 0 {
			err = errors.New("setup empty")
		}
		if err == nil {
			dev.SetState(DeviceOnline)
			return nil
		}
	}
	return err
}

// Please make sure it is called under cmdLk or don't use it.
func (dev *Device) Locked_TxKnown(request Packet, response *Packet) error {
	return dev.txKnown(request, response)
}

// Remote may ignore command with MDB timeout.
// state=Offline -> RESET
// state.Ok() required
func (dev *Device) TxMaybe(request Packet, response *Packet) error {
	dev.cmdLk.Lock()
	defer dev.cmdLk.Unlock()
	st := dev.State()
	err := dev.tx(request, response, txOptMaybe)
	return errors.Annotatef(err, "%s TxMaybe request=%x state=%s", dev.name, request.Bytes(), st.String())
}

func (dev *Device) TxCustom(request Packet, response *Packet, opt TxOpt) error {
	dev.cmdLk.Lock()
	defer dev.cmdLk.Unlock()
	st := dev.State()
	err := dev.tx(request, response, opt)
	return errors.Annotatef(err, "%s TxCustom request=%x state=%s", dev.name, request.Bytes(), st.String())
}

func (dev *Device) TxSetup() error {
	err := dev.TxKnown(dev.PacketSetup, &dev.SetupResponse)
	return errors.Annotatef(err, "%s SETUP", dev.name)
}

func (dev *Device) TxReadSetup() error {
	err := dev.Tx(dev.PacketSetup, &dev.SetupResponse)
	return errors.Annotatef(err, "%s SETUP", dev.name)
}

func (dev *Device) SetError(e error) {
	dev.SetState(DeviceError)
	dev.TeleError(e)
}

func (dev *Device) ErrorCode() int32 { return atomic.LoadInt32(&dev.errCode) }
func (dev *Device) SetErrorCode(code int32) {
	dev.errCode = code
}

// func (dev *Device) SetErrorCode(c int32) {
// 	// prev := atomic.SwapInt32(&dev.errCode, c)
// 	// if prev != ErrCodeNone && c != ErrCodeNone {
// 	// 	dev.Log.Infof("%s PLEASE REPORT SetErrorCode overwrite previous=%d", dev.name, prev)
// 	// }
// 	// if prev == ErrCodeNone && c != ErrCodeNone {
// 	// 	dev.SetError(fmt.Errorf("%s errcode=%d (%v)", dev.name, dev.errCode, dev.Tag))
// 	// }
// }

func (dev *Device) State() DeviceState       { return DeviceState(atomic.LoadUint32(&dev.state)) }
func (dev *Device) Ready() bool              { return dev.State() == DeviceReady }
func (dev *Device) SetState(new DeviceState) { atomic.StoreUint32(&dev.state, uint32(new)) }
func (dev *Device) SetReady()                { dev.SetState(DeviceReady) }
func (dev *Device) SetOnline()               { dev.SetState(DeviceOnline) }

func (dev *Device) Reset() error {
	dev.cmdLk.Lock()
	defer dev.cmdLk.Unlock()
	return dev.locked_reset()
}

// Keep particular devices "hot" to reduce useless POLL time.
func (dev *Device) Keepalive(interval time.Duration, stopch <-chan struct{}) {
	wait := interval

	for {
		// TODO try and benchmark time.After vs NewTimer vs NewTicker
		// dev.Log.Debugf("keepalive wait=%v", wait)
		if wait <= 0 {
			wait = 1
		}
		select {
		case <-stopch:
			return
		case <-time.After(wait):
		}
		dev.cmdLk.Lock()
		// // state could be updated during Lock()
		// if dev.State().Ok() {
		okAge := atomic_clock.Since(dev.LastOk)
		wait = interval - okAge
		// dev.Log.Debugf("keepalive locked okage=%v wait=%v", okAge, wait)
		if wait <= 0 {
			err := dev.txKnown(dev.PacketPoll, new(Packet))
			if !IsResponseTimeout(err) {
				dev.Log.Infof("%s Keepalive ignoring err=%v", dev.name, err)
			}
			wait = interval
		}
		dev.cmdLk.Unlock()
	}
}

type PollFunc func() (stop bool, err error)

// Call `fun` until `timeout` or it returns stop=true or error.
func (dev *Device) NewFunLoop(tag string, fun PollFunc, timeout time.Duration) engine.Doer {
	tag += "/poll-loop"
	return engine.Func{Name: tag, F: func(ctx context.Context) error {
		tbegin := time.Now()

		dev.cmdLk.Lock()
		defer dev.cmdLk.Unlock()
		for {
			// dev.Log.Debugf("%s timeout=%v elapsed=%v", tag, timeout, time.Since(tbegin))
			stop, err := fun()
			if err != nil {

				// dev.errCode = fmt.Sprintf("%d", err)
				return errors.Annotate(err, tag)
			} else if stop { // success
				return nil
			}
			// err==nil && stop==false -> try again
			if timeout == 0 {
				return errors.Errorf("tag=%s timeout=0 invalid", tag)
			}
			time.Sleep(dev.DelayNext)
			if time.Since(tbegin) > timeout {
				err = errors.Timeoutf(tag)
				dev.SetError(err)
				return err
			}
		}
	}}
}

type PollRequestFunc func(Packet) (stop bool, err error)

// Send `request` packets until `timeout` or `fun` returns stop=true or error.
func (dev *Device) NewPollLoop(tag string, request Packet, timeout time.Duration, fun PollRequestFunc) engine.Doer {
	iter := func() (bool, error) {
		response := Packet{}
		if err := dev.txKnown(request, &response); err != nil {
			return true, errors.Annotate(err, tag)
		}
		return fun(response)
	}
	return dev.NewFunLoop(tag, iter, timeout)
}

// Used by tests to avoid waiting.
func (dev *Device) XXX_FIXME_SetAllDelays(d time.Duration) {
	dev.DelayIdle = d
	dev.DelayNext = d
	dev.DelayBeforeReset = d
	dev.DelayAfterReset = d
	dev.DelayOffline = d
}

// cmdLk used to ensure no concurrent commands during delays
func (dev *Device) locked_reset() error {
	resetAge := atomic_clock.Since(dev.lastReset)
	if resetAge < dev.DelayOffline { // don't RESET too often
		dev.Log.Debugf("%s locked_reset delay=%v", dev.name, dev.DelayOffline-resetAge)
		time.Sleep(dev.DelayOffline - resetAge)
	}

	// st := dev.State()
	// dev.Log.Debugf("%s locked_reset begin state=%s", dev.name, st.String())
	dev.LastOff.SetNowIfZero() // consider device offline from now till successful response
	// dev.SetState(DeviceInited)
	time.Sleep(dev.DelayBeforeReset)
	err := dev.tx(dev.PacketReset, new(Packet), txOptReset)
	// dev.Log.Debugf("%s locked_reset after state=%s r.E=%v r.P=%s", dev.name, st.String(), r.E, r.P.Format())
	dev.lastReset.SetNow()
	atomic.StoreInt32(&dev.errCode, ErrCodeNone)
	if err != nil {
		err = errors.Annotatef(err, "%s RESET", dev.name)
		return err
	}
	// dev.Log.Infof("%s addr=%02x is working", dev.name, dev.Address)
	time.Sleep(dev.DelayAfterReset)
	return nil
}

func (dev *Device) txKnown(request Packet, response *Packet) error {
	st := dev.State()
	dev.Log.Debugf("%s txKnown request=%x state=%s", dev.name, request.Bytes(), st.String())
	return dev.tx(request, response, txOptKnown)
}

func (dev *Device) tx(request Packet, response *Packet, opt TxOpt) error {
	var err error
	st := dev.State()
	switch st {
	case DeviceInvalid:
		return errors.Annotatef(ErrStateInvalid, dev.name)

	case DeviceInited: // success path
		if !opt.NoReset {
			err = dev.locked_reset()
		}

	case DeviceOnline, DeviceReady: // success path

	case DeviceError: // FIXME TODO remove DeviceError state
		if opt.ResetError && !opt.NoReset {
			err = dev.locked_reset()
		}

	case DeviceOffline:
		dev.Log.Debugf("%s tx request=%x state=%s offline duration=%v", dev.name, request.Bytes(), st.String(), atomic_clock.Since(dev.LastOff))
		if opt.ResetOffline && !opt.NoReset {
			err = dev.locked_reset()
		}

	default:
		panic(fmt.Sprintf("code error %s tx request=%x unknown state=%v", dev.name, request.Bytes(), st))
	}
	if opt.RequireOK {
		if st2 := dev.State(); !st2.Ok() {
			err = ErrStateInvalid
		}
	}

	if err == nil {
		err = dev.bus.Tx(request, response)
	}
	if err == nil {
		// dev.Log.Debugf("%s since last ok %v", dev.name, atomic_clock.Since(dev.LastOk))
		dev.LastOk.SetNow()
		dev.LastOff.Set(0)
		// Upgrade any state except Ready to Online
		// Ready->Online would loose calibration.
		if st != DeviceReady {
			dev.SetState(DeviceOnline)
		}
		atomic.StoreInt32(&dev.errCode, ErrCodeNone)
	} else if IsResponseTimeout(err) {
		if opt.TimeoutOffline {
			dev.LastOff.SetNowIfZero()
			dev.SetState(DeviceOffline)
			err = errors.Wrap(err, types.DeviceOfflineError{Device: dev})
		}
	} else { // other error
		err = errors.Annotatef(err, "%s tx request=%x state=%s", dev.name, request.Bytes(), st.String())
		dev.SetError(err)
	}
	dev.Log.Debugf("%s tx request=%x -> ok=%t state %s -> %s err=%v",
		dev.name, request.Bytes(), err == nil, st, dev.State(), err)
	return err
}

// "Idle mode" polling, runs forever until receive on `stopch`.
// Switches between fast/idle delays.
// Used by bill/coin devices.
type PollDelay struct {
	lastActive time.Time
	lastDelay  time.Duration
}

func (pd *PollDelay) Delay(dev *Device, active bool, err bool, stopch <-chan struct{}) bool {
	delay := dev.DelayNext
	// delay := dev.DelayIdle
	if err {
		delay = dev.DelayIdle
		// delay = dev.DelayNext
	} else if active {
		pd.lastActive = time.Now()
	} else if pd.lastDelay != dev.DelayIdle { // save time syscall while idle continues
		if time.Since(pd.lastActive) > dev.IdleThreshold {
			delay = dev.DelayIdle
		}
	}
	pd.lastDelay = delay

	select {
	case <-stopch:
		return false
	case <-time.After(delay):
		return true
	}
}

type TxOpt struct {
	TimeoutOffline bool
	RequireOK      bool
	NoReset        bool
	ResetError     bool
	ResetOffline   bool
}

var (
	txOptKnown = TxOpt{
		TimeoutOffline: true,
		ResetOffline:   true,
		ResetError:     true,
	}
	txOptMaybe = TxOpt{
		RequireOK:    true,
		ResetOffline: true,
	}
	txOptReset = TxOpt{
		TimeoutOffline: true,
		NoReset:        true,
	}
)
