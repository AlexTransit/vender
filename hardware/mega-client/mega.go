package mega

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
	"github.com/temoto/alive/v2"
	gpio "github.com/temoto/gpio-cdev-go"
	"periph.io/x/periph/conn/physic"
)

const modName string = "mega-client"
const DefaultTimeout = 20 * time.Millisecond
const DefaultSpiSpeed = 200 * physic.KiloHertz
const busyDelay = 500 * time.Microsecond

var (
	ErrCriticalProtocol = errors.New("CRITICAL mega protocol error")
	ErrResponseEmpty    = errors.New("mega response empty")
	ErrRequestBusy      = errors.New("mega request busy")
)

type Client struct { //nolint:maligned
	refcount int32

	Log      *log2.Log
	TwiChan  chan uint16
	alive    *alive.Alive
	hw       hardware
	notifych chan struct{}
	stat     Stat
	txch     chan *tx

	// Do we have to redefine over-engineering yet?
	closed struct {
		mu  sync.Mutex
		yes bool
		err error
	}
}

type Stat struct {
	Request   uint32
	Error     uint32
	TwiListen uint32
	Reset     uint32
}

type tx struct {
	command  *Frame
	response *Frame
	wait     time.Duration
	err      error
	done     chan struct{}
}

func NewClient(config *Config, log *log2.Log) (*Client, error) {
	c := &Client{
		Log:      log,
		TwiChan:  make(chan uint16, TWI_LISTEN_MAX_LENGTH/2),
		alive:    alive.NewAlive(),
		notifych: make(chan struct{}),
		txch:     make(chan *tx),
	}
	if err := c.hw.open(config); err != nil {
		c.Close()
		return nil, errors.Annotate(err, "mega hw.open")
	}

	if err := c.handshake(); err != nil {
		c.Close()
		return nil, errors.Annotate(err, "mega handshake")
	}

	c.alive.Add(1)
	go c.notifyLoop()
	if !config.DontUseRawMode {
		c.alive.Add(1)
		go c.ioLoop()
	}

	return c, nil
}

// Thread-safe and idempotent.
func (c *Client) Close() error {
	c.closed.mu.Lock()
	defer c.closed.mu.Unlock()
	if !c.closed.yes {
		c.closed.yes = true
		c.alive.Stop()
		c.alive.Wait()
		close(c.TwiChan)
		close(c.notifych)
		close(c.txch)
		c.closed.err = c.hw.Close()
	}
	return c.closed.err
}

func (c *Client) IncRef(debug string) {
	c.Log.Debugf("%s incref by %s", modName, debug)
	atomic.AddInt32(&c.refcount, 1)
}
func (c *Client) DecRef(debug string) error {
	c.Log.Debugf("%s decref by %s", modName, debug)
	new := atomic.AddInt32(&c.refcount, -1)
	switch {
	case new > 0:
		return nil
	case new == 0:
		return c.Close()
	}
	panic(fmt.Sprintf("code error %s decref<0 debug=%s", modName, debug))
}

func (c *Client) DoStatus() (Frame, error) {
	return c.DoTimeout(COMMAND_STATUS, nil, DefaultTimeout)
}

func (c *Client) DoMdbBusReset(d time.Duration) (Frame, error) {
	buf := [2]byte{}
	binary.BigEndian.PutUint16(buf[:], uint16(d/time.Millisecond))
	return c.DoTimeout(COMMAND_MDB_BUS_RESET, buf[:], d+DefaultTimeout)
}

func (c *Client) DoMdbTxSimple(data []byte) (Frame, error) {
	const maxMdbReadTime = 40 * time.Millisecond
	return c.DoTimeout(COMMAND_MDB_TRANSACTION_SIMPLE, data, maxMdbReadTime+DefaultTimeout)
}

func (c *Client) DoTimeout(cmd Command_t, data []byte, timeout time.Duration) (Frame, error) {
	atomic.AddUint32(&c.stat.Request, 1)
	cmdFrame := NewCommand(cmd, data...)

	var response Frame
	err := c.Tx(&cmdFrame, &response, timeout)
	return response, err
}

func (c *Client) Stat() Stat {
	return c.stat
}

func (c *Client) Tx(command, response *Frame, timeout time.Duration) error {
	done := make(chan struct{})
	tx := &tx{command: command, response: response, wait: timeout, done: done}
	c.txch <- tx
	<-tx.done
	return tx.err
}

func (c *Client) XXX_RawTx(command []byte) ([]byte, error) {
	buf := make([]byte, BUFFER_SIZE+totalOverheads)
	if len(command) > BUFFER_SIZE {
		return buf, errors.New("command buffer overflow")
	}
	copy(buf, command)
	err := c.hw.spiTx(buf, buf)
	return buf, err
}

func (c *Client) handshake() error {
	var err error
	var stop bool
	var try uint8

	// retry reasons:
	// - mega had response buffered
	// - mega had command buffered
	// - handshake sent RESET
	for try = 1; try <= 5; try++ {
		var f Frame
		stop, err = c.handshakeStep(&f)
		if stop {
			break
		}
	}
	c.Log.Debugf("%s handshake try=%d err=%v", modName, try, err)
	return err
}

func (c *Client) handshakeStep(f *Frame) (bool, error) {
	err := c.ioReadParse(f)
	switch err {
	case nil:
		switch f.ResponseKind() {
		case RESPONSE_RESET: // success path
			c.Log.Debugf("%s handshake read=RESET the best option", modName)
			return true, nil
		default:
			c.Log.Errorf("%s handshake unexpected response=%s", modName, f.ResponseString())
			return false, nil
		}

	case ErrResponseEmpty: // success path mega is inited earlier
		c.Log.Debugf("%s handshake read=empty", modName)
		// TODO command reset
		return true, nil

	default:
		return false, err
	}
}

func (c *Client) ioLoop() {
	defer c.alive.Done()
	stopch := c.alive.StopChan()

	for c.alive.IsRunning() {
		select {
		case tx := <-c.txch:
			// c.Log.Debugf("ioLoop tx command=%s wait=%v", tx.command.CommandString(), tx.wait)
			tx.err = c.ioTx(tx)
			if tx.err != nil {
				atomic.AddUint32(&c.stat.Error, 1)
			}
			close(tx.done)
			// c.Log.Debugf("ioLoop tx done err=%v", tx.err)

		case <-c.notifych:
			// c.Log.Debugf("ioLoop notified without tx")
			c.alive.Add(1)
			bgrecv := Frame{}
			err := c.ioReadParse(&bgrecv)
			c.Log.Debugf("ioLoop bgrecv=%s", bgrecv.ResponseString())
			switch err {
			case nil: // success path
				kind := bgrecv.ResponseKind()
				switch {
				// already processed in parse()
				case kind == RESPONSE_TWI_LISTEN || kind == RESPONSE_RESET:

				case kind == RESPONSE_OK && bgrecv.Fields.MdbResult == MDB_RESULT_UART_READ_UNEXPECTED:
					//AlexM FIXME попал сюда при тесте. ( циклично слал) и это точно не помехи
					// если на coinco отключить бошку и ресетнуть, то получим все типа помехим
					c.Log.Infof("mega stray1 king(%v) MdbResult(%v) response(%s)", kind, bgrecv.Fields.MdbResult, bgrecv.ResponseString())

				default:
					// So far this always has been a symptom of critical protocol error
					c.Log.Infof("mega stray2 king(%v) MdbResult(%v) response(%s)", kind, bgrecv.Fields.MdbResult, bgrecv.ResponseString())
					// c.Log.Infof("%s stray packet %s", modName, bgrecv.ResponseString())
				}
			case ErrResponseEmpty:
				// XXX TODO FIXME error is still present, it only wastes time, not critical
				// c.Log.Errorf("%s FIXME tx=no notified=yes read=empty", modName)
			default:
				c.Log.Error(errors.Annotatef(err, "%s stray error", modName))
			}

		case <-stopch:
			return
		}
	}
}

// track write/wait/recv chain
func (c *Client) ioTx(tx *tx) error {
	c.alive.Add(1)
	defer c.alive.Done()
	saveGCPercent := debug.SetGCPercent(-1) // workaround for protocol error under GC stress
	defer debug.SetGCPercent(saveGCPercent)

	if tx.command != nil {
		err := c.ioWrite(tx.command)
		if err != nil {
			return errors.Annotatef(err, "command=%x", tx.command.Bytes())
		}
	}

	var err error
	for try := 1; try <= 13; try++ {
		notified := c.ioWait(tx.wait)
		err = c.ioReadParse(tx.response)
		// c.Log.Debugf("iotx try=%d parsed wait=%t notified=%t err=%v recv=%s", try, tx.wait != 0, notified, err, tx.response.ResponseString())
		switch err {
		case nil:
			// c.Log.Debugf("iotx parsed=%s", tx.response.ResponseString())
			switch tx.response.ResponseKind() {
			case RESPONSE_RESET:
				return errors.Annotatef(ErrCriticalProtocol, "mega reset during ioTx")
			case RESPONSE_TWI_LISTEN:
				// c.Log.Debugf("iotx captured background packet, must repeat")
				// wait and read again
			default:
				// success path when response is received
				return nil
			}

		case ErrResponseEmpty:
			switch {
			case tx.wait == 0 && !notified:
				// success path for read-only Tx() when no data is available
				return ErrResponseEmpty

			case tx.wait != 0 && !notified:
				// After this point we likely have lost command-response synchronisation.
				// Need to reset mega or skip
				return errors.Annotatef(ErrCriticalProtocol, "response timeout")

			default:
				// shouldn't ever happen
				c.Log.Errorf("mega TODO iotx try=%d wait=%v notified=%t read=empty", try, tx.wait, notified)
			}

		default: // other errors
			return errors.Wrap(err, ErrCriticalProtocol)
		}
		time.Sleep(time.Duration(try) * busyDelay)
	}
	return errors.Wrapf(err, ErrCriticalProtocol, "iotx too many tries")
}

func (c *Client) ioWait(timeout time.Duration) bool {
	// For wait=0 case, per Go spec, select would pick any case.
	// What we actually want with wait=0 is strong preference to reading, if available.
	if timeout == 0 {
		select {
		case <-c.notifych:
			return true
		default:
			return false
		}
	} else {
		tmr := time.NewTimer(timeout)
		defer tmr.Stop()
		select {
		case <-c.notifych:
			return true
		case <-tmr.C:
			return false
		}
	}
}

func (c *Client) notifyLoop() {
	defer c.alive.Done()

	if value, err := c.hw.notifier.Read(); err != nil {
		c.Log.Error(errors.Annotatef(err, "%s notifyLoop start Read()", modName))
	} else if value == 1 {
		c.Log.Debugf("%s notify=high on start", modName)
		c.notifych <- struct{}{}
	}

	// notifier.Wait timeout affects maximum time in Client.Close
	const timeout = 2 * time.Second

	// TODO replace with gpio.Eventer.Chan
	for c.alive.IsRunning() {
		edge, err := c.hw.notifier.Wait(timeout)
		if err == nil {
			if edge.ID == gpio.GPIOEVENT_EVENT_RISING_EDGE {
				c.notifych <- struct{}{}
			}
		} else if gpio.IsTimeout(err) {
			continue
		} else {
			c.Log.Error(errors.Annotatef(err, "%s notifyLoop Wait", modName))
			go c.Close()
			return
		}
	}
}

func (c *Client) ioWrite(f *Frame) error {
	// static allocation of maximum possible size
	var bufarr [BUFFER_SIZE + totalOverheads]byte
	// slice it down to shorten SPI session time
	bs := f.Bytes()
	buf := bufarr[:len(bs)+ /*recv ioAck*/ 6+totalOverheads]
	ackExpect := []byte{0x00, 0xff, f.crc, f.crc}
	var busy bool

	for try := 1; try <= 3; try++ {
		copy(buf, bs)
		busy = false
		// c.Log.Debugf("ioWrite out=%x", buf)
		err := c.hw.spiTx(buf, buf)
		if err != nil {
			return err
		}
		// c.Log.Debugf("ioWrite -in=%x", buf)

		var padStart int
		var errcode Errcode_t
		if padStart, errcode, err = parsePadding(buf, false); err != nil {
			return err
		}
		switch errcode {
		case 0:
		case ERROR_REQUEST_OVERWRITE:
			busy = true
			c.Log.Debugf("%s ioWrite: input buffer is busy -> sleep,retry", modName)
			time.Sleep(busyDelay)
			continue
		default:
			return errors.Errorf("FIXME errcode=%d buf=%x", errcode, buf)
		}
		if padStart < 6 {
			c.Log.Errorf("%s ioWrite: invalid ioAck buf=%x", modName, buf)
			continue
		}
		ackRemote := buf[padStart-6 : padStart-2]
		if !bytes.Equal(ackRemote, ackExpect) {
			c.Log.Errorf("%s ioWrite: invalid ioAck expected=%x actual=%x", modName, ackExpect, ackRemote)
			continue
		}

		if buf[0]&PROTOCOL_FLAG_REQUEST_BUSY != 0 {
			busy = true
			c.Log.Debugf("%s ioWrite: input buffer is busy -> sleep,retry", modName)
			time.Sleep(busyDelay)
			continue
		}

		break
	}
	if busy {
		return ErrRequestBusy
	}
	return nil
}

func (c *Client) ioReadParse(frame *Frame) error {
	var lenBuf [2]byte
	lenBuf[0] = ProtocolVersion
	// c.Log.Debugf("%s read length out=%x", modName, lenBuf)
	err := c.hw.spiTx(lenBuf[:], lenBuf[:])
	// c.Log.Debugf("%s read length -in=%x err=%v", modName, lenBuf, err)
	if err != nil {
		return err
	}
	var remoteLength uint8
	if _, _, remoteLength, err = parseHeader(lenBuf[:]); err != nil {
		return err
	}
	if remoteLength == 0 {
		return ErrResponseEmpty
	}

	var buf [BUFFER_SIZE + totalOverheads]byte
	bs := buf[:remoteLength+totalOverheads]
	bs[0] = ProtocolVersion
	// c.Log.Debugf("%s read out=%x", modName, bs)
	err = c.hw.spiTx(bs, bs)
	// c.Log.Debugf("%s read -in=%x err=%v", modName, bs, err)
	if err != nil {
		return err
	}

	err = c.parse(bs, frame)
	if err != nil {
		return err
	}
	err = c.ioAck(frame)
	return err
}

func (c *Client) ioAck(f *Frame) error {
	var buf [2 + totalOverheads]byte
	buf[0] = PROTOCOL_FLAG_REQUEST_BUSY | ProtocolVersion
	buf[1] = 2
	buf[2] = f.plen
	buf[3] = f.crc

	// c.Log.Debugf("%s ioAck out=%x", modName, buf)
	err := c.hw.spiTx(buf[:], buf[:])
	// c.Log.Debugf("%s ioAck -in=%x err=%v", modName, buf, err)
	if err != nil {
		return err
	}

	_, _, err = parsePadding(buf[:], true)
	return err
}

func (c *Client) parse(buf []byte, f *Frame) error {
	err := f.Parse(buf)
	if err != nil {
		atomic.AddUint32(&c.stat.Error, 1)
		c.Log.Error(errors.Annotatef(err, "%s Parse buf=%x", modName, buf))
		return err
	}
	if f.plen == 0 {
		return ErrResponseEmpty
	}
	err = f.ParseFields()
	if err != nil {
		atomic.AddUint32(&c.stat.Error, 1)
		c.Log.Error(errors.Annotatef(err, "%s ParseFields frame=%x", modName, f.Bytes()))
		return err
	}

	for i := 0; i+1 < len(f.Fields.TwiData); i += 2 {
		twitem := binary.BigEndian.Uint16(f.Fields.TwiData[i : i+2])
		if len(c.TwiChan) == 0 {
			c.TwiChan <- twitem
		} else {
			c.Log.Debugf("IGNORE twitem. TwiChan not empty")
		}
	}

	switch f.ResponseKind() {
	case RESPONSE_TWI_LISTEN:
		atomic.AddUint32(&c.stat.TwiListen, 1)
	case RESPONSE_RESET:
		atomic.AddUint32(&c.stat.Reset, 1)
		if ResetFlag(f.Fields.Mcusr)&ResetFlagWatchdog != 0 {
			atomic.AddUint32(&c.stat.Error, 1)
			c.Log.Errorf("mega restarted by watchdog, info=%s", f.Fields.String())
		} else {
			c.Log.Debugf("mega normal reset, info=%s", f.Fields.String())
		}
	}

	return nil
}
