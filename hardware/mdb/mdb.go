package mdb

import (
	"fmt"
	"time"

	"github.com/AlexTransit/vender/log2"
	oerr "github.com/juju/errors"
)

const (
	DefaultBusResetKeep  = 200 * time.Millisecond
	DefaultBusResetSleep = 500 * time.Millisecond
)

var (
	ErrNak        = fmt.Errorf("MDB NAK")
	ErrBusy       = fmt.Errorf("MDB busy")
	ErrTimeoutMDB = fmt.Errorf("MDB timeout")
)

type Uarter interface {
	Break(d, sleep time.Duration) error
	Close() error
	Open(options string) error
	Tx(request, response []byte) (int, error)
}

type FeatureNotSupported string

func (fns FeatureNotSupported) Error() string { return string(fns) }

type Bus struct {
	Error func(error)
	Log   *log2.Log
	u     Uarter
}

func NewBus(u Uarter, log *log2.Log, errfun func(error)) *Bus {
	return &Bus{
		Error: errfun,
		Log:   log,
		u:     u,
	}
}

func (b *Bus) ResetDefault() error {
	return b.Reset(DefaultBusResetKeep, DefaultBusResetSleep)
}

func (b *Bus) Reset(keep, sleep time.Duration) error {
	b.Log.Debugf("mdb.bus.Reset keep=%v sleep=%v", keep, sleep)
	return b.u.Break(keep, sleep)
}

func (b *Bus) Tx(request Packet, response *Packet) (err error) {
	if request.l == 0 {
		return nil
	}
	rp := &Packet{}
	if response != nil {
		if response.readonly {
			return ErrPacketReadonly
		}
		rp = response
	}

	reqBs := request.Bytes()
	rp.l, err = b.u.Tx(reqBs, rp.b[:])
	if err != nil {
		err = fmt.Errorf("error=%v mdb.Tx send=%x recv=%x", err, reqBs, rp.Bytes())
	}
	// if response != nil && rp.l == 0 { // need answer
	// 	err = fmt.Errorf("device not anwer")
	// } 
	return err
}

func (b *Bus) TxOld(request Packet, response *Packet) error {
	if response == nil {
		response = &Packet{}
		b.Log.Debugf("mdb.Tx request=%x response=nil -> allocate temporary", request.Bytes())
	}
	if response.readonly {
		return ErrPacketReadonly
	}
	if request.l == 0 {
		return nil
	}

	rbs := request.Bytes()
	n, err := b.u.Tx(rbs, response.b[:])
	response.l = n

	if err != nil {
		b.Log.Errorf("mega transmit error:%v", err)
		return fmt.Errorf("error=%v mdb.Tx send=%x recv=%x", err, rbs, response.Bytes())
	}
	// explicit level check to save costly .Format()
	if b.Log.Enabled(log2.LOG_DEBUG) {
		b.Log.Debugf("mdb.Tx (%02d) %s -> (%02d) %s",
			request.Len(), request.Format(), response.Len(), response.Format())
	}
	return nil
}

func IsResponseTimeout(e error) bool {
	return e != nil && oerr.Cause(e) == ErrTimeoutMDB
}
