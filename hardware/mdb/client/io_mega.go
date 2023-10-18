package mdb_client

import (
	"sync"
	"time"

	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/hardware/mega-client"
	"github.com/juju/errors"
)

const (
	DelayErr = 50 * time.Millisecond
)

type megaUart struct {
	c  *mega.Client
	lk sync.Mutex
}

func NewMegaUart(client *mega.Client) mdb.Uarter {
	return &megaUart{c: client}
}
func (mu *megaUart) Open(_ string) error {
	mu.c.IncRef("mdb-uart")
	return nil
	// _, err := mu.c.DoTimeout(mega.COMMAND_STATUS, nil, 5*time.Second)
	// return err
}
func (mu *megaUart) Close() error {
	return mu.c.DecRef("mdb-uart")
}

func responseError(r mega.Mdb_result_t, arg byte) error {
	switch r {
	case mega.MDB_RESULT_SUCCESS:
		return nil
	case mega.MDB_RESULT_BUSY:
		// err := errors.NewErr("MDB busy state=%s", mega.Mdb_state_t(p.Fields.MdbError).String())
		return mdb.ErrBusy
	case mega.MDB_RESULT_TIMEOUT:
		return mdb.ErrTimeoutMDB
	case mega.MDB_RESULT_NAK:
		return mdb.ErrNak
	default:
		err := errors.NewErr("mega MDB error result=%s arg=%02x", r.String(), arg)
		err.SetLocation(2)
		return &err
	}
}

func (mu *megaUart) Break(d, sleep time.Duration) error {
	mu.lk.Lock()
	defer mu.lk.Unlock()

	var f mega.Frame
	var err error
	for retry := 1; retry <= 3; retry++ {
		f, err = mu.c.DoMdbBusReset(d)
		switch errors.Cause(err) {
		case nil: // success path
			err = responseError(f.Fields.MdbResult, f.Fields.MdbError)
			if err == nil {
				time.Sleep(sleep)
				return nil
			}
			time.Sleep(DelayErr)

		case mega.ErrCriticalProtocol:
			mu.c.Log.Fatal(errors.ErrorStack(err))

		default:
			return err
		}
	}
	return err
}

func (mu *megaUart) Tx(request, response []byte) (n int, err error) {
	const tag = "mdb.mega.Tx"
	mu.lk.Lock()
	defer mu.lk.Unlock()
	var f mega.Frame
	f, err = mu.c.DoMdbTxSimple(request)
	switch errors.Cause(err) {
	case nil: // success path
		if f.Fields.MdbResult != mega.MDB_RESULT_SUCCESS {
			err = errors.Errorf("mdb request (%v)", f.Fields.MdbResult.String())
			return 0, err
		}
		mu.c.Log.Debugf("%s request=%x f=%s", tag, request, f.ResponseString())
		n = copy(response, f.Fields.MdbData)
		return n, nil
	case mega.ErrCriticalProtocol:
		// Alexm - падает все. бабло не возвращает.
		err = errors.Annotatef(err, "%s CRITICAL request=%x", tag, request)
		mu.c.Log.Error(err)
		return 0, err
	default:
		err = errors.Annotatef(err, "%s request=%x", tag, request)
		mu.c.Log.Error(err)
		return 0, err
	}
}
