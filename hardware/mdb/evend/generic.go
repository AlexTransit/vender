// Package evend incapsulates common parts of MDB protocol for eVend machine
// devices like conveyor, hopper, cup dispenser, elevator, etc.
package evend

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/log2"
	oerr "github.com/juju/errors"
)

// Mostly affects POLL response, see doc.
type evendProtocol uint8

const (
	proto1 evendProtocol = 1
	proto2 evendProtocol = 2
)

const (
	resetAddress     = 0x00
	infoAddress      = 0x01
	commandAddress   = 0x02
	statusAddress    = 0x03
	readDataAddress  = 0x04
	configAddress    = 0x05
	upgradeFwAddress = 0x06
)

const (
	genericPollMiss    = 0x04
	genericPollProblem = 0x08
	genericPollBusy    = 0x50

	DefaultReadyTimeout = 5 * time.Second
	DefaultResetDelay   = 2100 * time.Millisecond
)

type DeviceErrorCode byte

func (c DeviceErrorCode) Error() string { return fmt.Sprintf("evend errorcode=%d", c) }

type Generic struct {
	log          *log2.Log
	dev          mdb.Device
	name         string
	readyTimeout time.Duration
	proto        evendProtocol

	// For most devices 0x50 = busy
	// valve 0x10 = busy, 0x40 = hot water is colder than configured
	proto2BusyMask   byte
	proto2IgnoreMask byte
}

func (gen *Generic) Init(ctx context.Context, address uint8, name string, proto evendProtocol) {
	gen.name = "evend." + name

	if gen.proto2BusyMask == 0 {
		gen.proto2BusyMask = genericPollBusy
	}
	if gen.readyTimeout == 0 {
		gen.readyTimeout = DefaultReadyTimeout
	}
	gen.proto = proto

	if gen.dev.DelayBeforeReset == 0 {
		gen.dev.DelayBeforeReset = 2 * DefaultResetDelay
	}
	if gen.dev.DelayAfterReset == 0 {
		gen.dev.DelayAfterReset = DefaultResetDelay
	}
	g := state.GetGlobal(ctx)
	gen.log = g.Log
	mdbus, _ := g.Mdb()
	gen.dev.Init(mdbus, address, gen.name, binary.BigEndian)
}

func (gen *Generic) Name() string { return gen.name }

func (gen *Generic) NewErrPollProblem(p mdb.Packet) error {
	return fmt.Errorf("%s POLL=%x -> need to ask problem code", gen.name, p.Bytes())
}

func (gen *Generic) NewErrPollUnexpected(p mdb.Packet) error {
	return fmt.Errorf("%s POLL=%x unexpected", gen.name, p.Bytes())
}

func (gen *Generic) NewAction(tag string, args ...byte) engine.Doer {
	return engine.Func0{Name: tag, F: func() error {
		return gen.txAction(args)
	}}
}

func (gen *Generic) txAction(args []byte) error {
	bs := make([]byte, len(args)+1)
	bs[0] = gen.dev.Address + 2
	copy(bs[1:], args)
	request := mdb.MustPacketFromBytes(bs, true)
	response := mdb.Packet{}
	err := gen.dev.TxMaybe(request, &response) // FIXME check everything and change to TxKnown
	if err != nil {
		return err
	}
	gen.dev.Log.Debugf("%s action=%x response=(%d)%s", gen.name, args, response.Len(), response.Format())
	return nil
}

func (gen *Generic) Diagnostic() (byte, error) {
	tag := gen.name + ".diagnostic"

	bs := []byte{gen.dev.Address + 4, 0x02}
	request := mdb.MustPacketFromBytes(bs, true)
	response := mdb.Packet{}
	// Assumptions:
	// - (Known) all evend devices support diagnostic command +402
	// - (Locked) it's safe to call CommandErrorCode concurrently with other
	err := gen.dev.Locked_TxKnown(request, &response)
	if err != nil {
		gen.dev.SetError(err)
		return 0, oerr.Annotate(err, tag)
	}
	rs := response.Bytes()
	if len(rs) < 1 {
		err = fmt.Errorf("%s request=%x response=%x", tag, request.Bytes(), rs)
		gen.dev.SetError(err)
		return 0, err
	}
	// gen.dev.SetErrorCode(int32(rs[0]))
	return rs[0], nil
}

// NewWaitReady proto1/2 agnostic, use it before action
func (gen *Generic) NewWaitReady(tag string) engine.Doer {
	tag += "/wait-ready"
	switch gen.proto {
	case proto1:
		fun := func(p mdb.Packet) (bool, error) {
			bs := p.Bytes()
			switch len(bs) {
			case 0: // success path
				return true, nil

			case 2: // device reported error code
				code := bs[1]
				if code == 0 {
					return true, nil
				}
				gen.dev.Log.Errorf("%s response=%x errorcode=%d", tag, bs, code)
				// gen.dev.SetErrorCode(int32(code))
				return true, DeviceErrorCode(code)

			default:
				err := fmt.Errorf("%s unknown response=%x", tag, bs)
				gen.dev.Log.Error(err)
				return false, err
			}
		}
		return gen.dev.NewPollLoop(tag, gen.dev.PacketPoll, gen.readyTimeout, fun)

	case proto2:
		fun := func(p mdb.Packet) (bool, error) {
			bs := p.Bytes()
			// gen.dev.Log.Debugf("%s POLL=%x", tag, bs)
			if stop, err := gen.proto2PollCommon(tag, bs); stop || err != nil {
				return stop, err
			}
			value := bs[0]
			value &^= gen.proto2IgnoreMask

			// 04 during WaitReady is "OK, poll few more"
			if value&genericPollMiss != 0 {
				// gen.dev.SetReady(false)
				value &^= genericPollMiss
			}

			// busy during WaitReady is problem (previous action did not finish cleanly)
			if value == gen.proto2BusyMask {
				// err := errors.Errorf("%s PLEASE REPORT WaitReady POLL=%x (busy) unexpected", tag, bs[0])
				// gen.dev.SetError(err)
				return false, nil
			}

			if value == 0 {
				// gen.dev.Log.Debugf("%s WaitReady value=%02x (%02x&^%02x) -> late repeat", tag, value, bs[0], gen.proto2IgnoreMask)
				return false, nil
			}

			// gen.dev.SetErrorCode(1)
			gen.dev.Log.Errorf("%s PLEASE REPORT WaitReady value=%02x (%02x&^%02x) -> unexpected", tag, value, bs[0], gen.proto2IgnoreMask)
			return true, gen.NewErrPollUnexpected(p)
		}
		return gen.dev.NewPollLoop(tag, gen.dev.PacketPoll, gen.readyTimeout, fun)

	default:
		panic("code error")
	}
}

// NewWaitDone proto1/2 agnostic, use it after action
func (gen *Generic) NewWaitDone(tag string, timeout time.Duration) engine.Doer {
	tag += "/wait-done"

	switch gen.proto {
	case proto1:
		return gen.newProto1PollWaitSuccess(tag, timeout)

	case proto2:
		fun := func(p mdb.Packet) (bool, error) {
			bs := p.Bytes()
			// gen.dev.Log.Debugf("%s POLL=%x", tag, bs)
			if stop, err := gen.proto2PollCommon(tag, bs); stop || err != nil {
				// gen.dev.Log.Debugf("%s ... return common stop=%t err=%v", tag, stop, err)
				return stop, err
			}
			value := bs[0]
			value &^= gen.proto2IgnoreMask

			// 04 during WaitDone is "oops, device reboot in operation"
			if value&genericPollMiss != 0 {
				gen.dev.SetState(mdb.DeviceOnline)
				return true, fmt.Errorf("%s POLL=%x ignore=%02x continous connection lost, (TODO decide reset?)", tag, bs, gen.proto2IgnoreMask)
			}

			// busy during WaitDone is correct path
			if value == gen.proto2BusyMask {
				// gen.dev.Log.Debugf("%s POLL=%x (busy) -> ok, repeat", tag, bs[0])
				return false, nil
			}

			gen.dev.Log.Debugf("%s poll-wait-done value=%02x (%02x&^%02x)", tag, value, bs[0], gen.proto2IgnoreMask)
			if value == 0 {
				gen.dev.Log.Debugf("%s PLEASE REPORT POLL=%x final=00", tag, bs[0])
				return true, nil
			}
			return true, gen.NewErrPollUnexpected(p)
		}
		return gen.dev.NewPollLoop(tag, gen.dev.PacketPoll, timeout, fun)

	default:
		panic("code error")
	}
}

func (gen *Generic) newProto1PollWaitSuccess(tag string, timeout time.Duration) engine.Doer {
	success := []byte{0x0d, 0x00}
	fun := func(p mdb.Packet) (bool, error) {
		bs := p.Bytes()
		if len(bs) == 0 { // empty -> try again
			// gen.dev.Log.Debugf("%s POLL=empty", tag)
			return false, nil
		}
		if bytes.Equal(bs, success) {
			return true, nil
		}
		if bs[0] == 0x04 {
			code := bs[1]
			gen.dev.SetErrorCode(int32(code))
			return true, DeviceErrorCode(code)
		}
		return true, gen.NewErrPollUnexpected(p)
	}
	return gen.dev.NewPollLoop(tag, gen.dev.PacketPoll, timeout, fun)
}

func (gen *Generic) proto2PollCommon(tag string, bs []byte) (bool, error) {
	if len(bs) == 0 {
		return true, nil
	}
	if len(bs) > 1 {
		return true, fmt.Errorf("%s POLL=%x -> too long", tag, bs)
	}
	value := bs[0]
	value &^= gen.proto2IgnoreMask
	if bs[0] != 0 && value == 0 {
		gen.dev.Log.Debugf("%s proto2-common value=00 bs=%02x ignoring mask=%02x -> success", tag, bs[0], gen.proto2IgnoreMask)
		return true, nil
	}
	if value&genericPollProblem != 0 {
		code, err := gen.Diagnostic()
		if err != nil {
			err = fmt.Errorf("%s %v", tag, err)
			return true, err
		}
		return true, DeviceErrorCode(code)
	}
	return false, nil
}

//-------------------------------------------------------------------------

// timeout count every 200 ms
func (gen *Generic) Proto1PollWaitSuccess(count uint16, timeOut bool) (err error) {
	response := mdb.Packet{}
	for count > 0 {
		count--
		time.Sleep(200 * time.Millisecond)
		if err = gen.dev.Tx(gen.dev.PacketPoll, &response); err != nil {
			return err
		}
		rb := response.Bytes()
		if len(rb) == 0 {
			continue
		}
		switch rb[0] {
		case 0x0d: // complete execute
			gen.dev.Action = ""
			return
		case 0x04:
			errCode := rb[1]
			gen.dev.SetErrorCode(int32(errCode))
			e := fmt.Errorf("execute command(%s) error(%v)", gen.dev.Action, errCode)
			return e
		default:
			err = fmt.Errorf("unknow answer(%v) on command(%s)", rb, gen.dev.Action)
			return
		}
	}
	if timeOut {
		if gen.dev.Action != "" {
			err = fmt.Errorf("execute command (%v) timeout", gen.dev.Action)
		}
	}
	return err
}

func (gen *Generic) Proto2PollWaitSuccess(count uint16, timeOut bool) (err error) {
	response := mdb.Packet{}
	var needReset bool
	for count > 0 {
		count--
		time.Sleep(200 * time.Millisecond)
		if err = gen.dev.Tx(gen.dev.PacketPoll, &response); err != nil {
			return err
		}
		// AlexM FIXME
		// for valve clear 2 bit. 8-bit = not configure 7-bit -temp not walid
		rb := response.Byte() << 2
		rb = rb >> 2
		if rb&0x08 == 0x08 || response.Len() == 0 || rb == 0 {
			// len=0 maybe sucsess :)
			return gen.ReadError()
		}
		if rb&0x20 == 0x20 {
			needReset = true
		}
		if rb&0x50 == 0x50 { // executing
			continue
		}
	}
	if needReset {
		gen.dev.Rst()
		return fmt.Errorf("device %v reseted", gen.dev.Name())
	}
	if timeOut {
		return errors.New("time out pool")
	}
	return nil
}

func (gen *Generic) WaitSuccess(count uint16, timeOut bool) error {
	if gen.proto == proto1 {
		return gen.Proto1PollWaitSuccess(count, timeOut)
	}
	return gen.Proto2PollWaitSuccess(count, timeOut)
}

func (gen *Generic) CommandNoWait(cmd ...byte) (err error) {
	if err = gen.Command(cmd...); err != nil {
		return
	}
	return gen.WaitSuccess(5, false)
}

func (gen *Generic) CommandWaitSuccess(count uint16, cmd ...byte) (err error) {
	if err = gen.Command(cmd...); err != nil {
		return
	}
	return gen.WaitSuccess(count, true)
}

func (gen *Generic) Command(args ...byte) (err error) {
	return gen.addressArgumentsTx(commandAddress, &args)
}

func (gen *Generic) SetConfig(args ...byte) (err error) {
	for i := 1; i <= 2; i++ {
		e := gen.addressArgumentsTx(configAddress, &args)
		if e == nil {
			if err != nil {
				gen.log.Error(err)
			}
			return nil
		}
		err = errors.Join(err, e)
	}
	return err
}

func (gen *Generic) addressArgumentsTx(address byte, args *[]byte) error {
	bs := make([]byte, len(*args)+1)
	bs[0] = gen.dev.Address + address
	copy(bs[1:], *args)
	return gen.tx(&bs, nil)
}

func (gen *Generic) ReadError_proto2() (errb byte) {
	response, err := gen.ReadData(0x02)
	if err != nil || len(response) == 0 {
		return 255
	}
	return response[0]
}

func (gen *Generic) ReadError() (err error) {
	if errb := gen.ReadError_proto2(); errb != 0 {
		gen.dev.Rst()
		gen.dev.SetErrorCode(int32(errb))
		return fmt.Errorf("device:%s error:%v", gen.name, errb)
	}
	return nil
}

// read device data
// 0x02 - read error
// 0x11 - read temperature
func (gen *Generic) ReadData(readCommand byte) (rp []byte, err error) {
	bs := []byte{gen.dev.Address + 4, readCommand}
	request := mdb.MustPacketFromBytes(bs, true)
	response := mdb.Packet{}
	for i := 1; i <= 3; i++ {
		e := gen.dev.Tx(request, &response)
		if e == nil || response.Len() != 0 {
			rp = response.Bytes()
			if err != nil {
				gen.log.Errorf("%s read data (%v)", gen.name, err)
			}
			return
		}
		err = errors.Join(err, fmt.Errorf("(%d) %s %v", i, gen.name, e))
	}
	return nil, err
}

func (gen *Generic) tx(bs *[]byte, response ...*mdb.Packet) (err error) {
	request := mdb.MustPacketFromBytes(*bs, true)
	for i := 1; i <= 3; i++ {
		e := gen.dev.Tx(request, response[0])
		if e == nil {
			if err != nil {
				gen.log.Errorf("resend fix error (%v)", err)
			}
			return nil
		}
		err = errors.Join(err, fmt.Errorf("(%d) transmit %s (%v)", i, gen.name, e))
	}
	return
}
