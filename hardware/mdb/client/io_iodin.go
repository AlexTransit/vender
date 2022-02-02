package mdb_client

import (
	"sync"
	"time"

	"github.com/juju/errors"
	iodin "github.com/temoto/iodin/client/go-iodin"
)

type iodinUart struct {
	c  *iodin.Client
	lk sync.Mutex
}

func NewIodinUart(c *iodin.Client) *iodinUart {
	c.IncRef("mdb")
	return &iodinUart{c: c}
}

func (iu *iodinUart) Close() error {
	err := iu.c.DecRef("mdb")
	iu.c = nil
	return err
}

func (iu *iodinUart) Break(d, sleep time.Duration) error {
	iu.lk.Lock()
	defer iu.lk.Unlock()

	ms := int(d / time.Millisecond)
	var r iodin.Response
	err := iu.c.Do(&iodin.Request{Command: iodin.Request_MDB_RESET, ArgUint: uint32(ms)}, &r)
	if err != nil {
		return errors.Trace(err)
	}
	time.Sleep(sleep)
	return nil
}

func (iu *iodinUart) Open(path string) error {
	var r iodin.Response
	err := iu.c.Do(&iodin.Request{Command: iodin.Request_MDB_OPEN, ArgBytes: []byte(path)}, &r)
	return errors.Trace(err)
}

func (iu *iodinUart) Tx(request, response []byte) (n int, err error) {
	iu.lk.Lock()
	defer iu.lk.Unlock()

	var r iodin.Response
	err = iu.c.Do(&iodin.Request{Command: iodin.Request_MDB_TX, ArgBytes: request}, &r)
	n = copy(response, r.DataBytes)
	return n, errors.Trace(err)
}
