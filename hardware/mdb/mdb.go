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

func IsResponseTimeout(e error) bool {
	return e != nil && oerr.Cause(e) == ErrTimeoutMDB
}
