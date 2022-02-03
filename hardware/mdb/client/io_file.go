package mdb_client

import (
	"bufio"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
	"golang.org/x/sys/unix"
)

type fileUart struct {
	Log *log2.Log
	f   *os.File // set nil in test
	fd  uintptr
	r   io.Reader // override in test
	w   io.Writer // override in test
	br  *bufio.Reader
	t2  termios2
	lk  sync.Mutex
}

func NewFileUart(l *log2.Log) *fileUart {
	return &fileUart{
		Log: l,
		br:  bufio.NewReader(nil),
	}
}

func (fu *fileUart) set9(b bool) error {
	last_parodd := (fu.t2.c_cflag & syscall.PARODD) == syscall.PARODD
	if b == last_parodd {
		return nil
	}
	if b {
		fu.t2.c_cflag |= syscall.PARODD
	} else {
		fu.t2.c_cflag &= ^tcflag_t(syscall.PARODD)
	}
	if fu.f == nil { // used in tests
		return nil
	}
	// must use ioctl with drain - cTCSETSW2?
	// but it makes 9bit switch very slow
	err := ioctl(fu.fd, uintptr(cTCSETS2), uintptr(unsafe.Pointer(&fu.t2)))
	return errors.Trace(err)
}

func (fu *fileUart) write9(p []byte, start9 bool) (n int, err error) {
	// fu.Log.Debugf("mdb.write9 p=%x start9=%t", p, start9)
	var n2 int
	switch len(p) {
	case 0:
		return 0, nil
	case 1:
		if err = fu.set9(start9); err != nil {
			return 0, errors.Trace(err)
		}
		if n, err = fu.w.Write(p[:1]); err != nil {
			return 0, errors.Trace(err)
		}
		fallthrough
	default:
		if err = fu.set9(false); err != nil {
			return n, errors.Trace(err)
		}
		if n2, err = fu.w.Write(p[1:]); err != nil {
			return n, errors.Trace(err)
		}
	}
	return n + n2, nil
}

func (fu *fileUart) Break(d, sleep time.Duration) (err error) {
	const tag = "fileUart.Break"
	ms := int(d / time.Millisecond)
	fu.lk.Lock()
	defer fu.lk.Unlock()
	if err = fu.resetRead(); err != nil {
		return errors.Annotate(err, tag)
	}
	if err = ioctl(fu.fd, uintptr(cTCSBRKP), uintptr(ms/100)); err != nil {
		return errors.Annotate(err, tag)
	}
	time.Sleep(sleep)
	return nil
}

func (fu *fileUart) Close() error {
	fu.f = nil
	fu.r = nil
	fu.w = nil
	return errors.Trace(fu.f.Close())
}

func (fu *fileUart) Open(path string) (err error) {
	if fu.f != nil {
		fu.Close() // skip error
	}
	fu.f, err = os.OpenFile(path, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NDELAY, 0600)
	if err != nil {
		return errors.Annotate(err, "fileUart.Open:OpenFile")
	}
	fu.fd = fu.f.Fd()
	fu.r = fdReader{fd: fu.fd, timeout: 20 * time.Millisecond}
	fu.br.Reset(fu.r)
	fu.w = fu.f

	fu.t2 = termios2{
		c_iflag:  unix.IGNBRK | unix.INPCK | unix.PARMRK,
		c_lflag:  0,
		c_cflag:  cCMSPAR | syscall.CLOCAL | syscall.CREAD | unix.CSTART | syscall.CS8 | unix.PARENB | unix.PARMRK | unix.IGNPAR,
		c_ispeed: speed_t(unix.B9600),
		c_ospeed: speed_t(unix.B9600),
	}
	fu.t2.c_cc[syscall.VMIN] = cc_t(0)
	err = ioctl(fu.fd, uintptr(cTCSETSF2), uintptr(unsafe.Pointer(&fu.t2)))
	if err != nil {
		fu.Close()
		return errors.Annotate(err, "fileUart.Open:ioctl")
	}
	var ser serial_info
	err = ioctl(fu.fd, uintptr(cTIOCGSERIAL), uintptr(unsafe.Pointer(&ser)))
	if err != nil {
		fu.Log.Errorf("get serial fail err=%v", err)
	} else {
		ser.flags |= cASYNC_LOW_LATENCY
		err = ioctl(fu.fd, uintptr(cTIOCSSERIAL), uintptr(unsafe.Pointer(&ser)))
		if err != nil {
			fu.Log.Errorf("set serial fail err=%v", err)
		}
	}
	return nil
}

func (fu *fileUart) Tx(request, response []byte) (n int, err error) {
	if len(request) == 0 {
		return 0, errors.New("Tx request empty")
	}

	// TODO feed IO operations to loop in always running goroutine
	// that would also eliminate lock
	fu.lk.Lock()
	defer fu.lk.Unlock()

	saveGCPercent := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(saveGCPercent)

	// FIXME crutch to avoid slow set9 with drain
	time.Sleep(20 * time.Millisecond)
	// TODO
	// fu.f.SetDeadline(time.Now().Add(time.Second))
	// defer fu.f.SetDeadline(time.Time{})

	chkoutb := []byte{checksum(request)}
	if _, err = fu.write9(request, true); err != nil {
		return 0, errors.Trace(err)
	}
	if _, err = fu.write9(chkoutb, false); err != nil {
		return 0, errors.Trace(err)
	}

	// ack must arrive <5ms after recv
	// begin critical path
	if err = fu.resetRead(); err != nil {
		return 0, errors.Trace(err)
	}
	n, err = bufferReadPacket(fu.br, response)
	if err != nil {
		return 0, errors.Trace(err)
	}
	chkin := response[n-1]
	n--
	chkcomp := checksum(response[:n])
	if chkin != chkcomp {
		// fu.Log.Debugf("mdb.fileUart.Tx InvalidChecksum frompacket=%x actual=%x", chkin, chkcomp)
		return n, errors.Trace(mdb.InvalidChecksum{Received: chkin, Actual: chkcomp})
	}
	if n > 0 {
		_, err = fu.write9(mdb.PacketAck.Bytes(), false)
	}
	// end critical path
	return n, errors.Trace(err)
}

func bufferReadPacket(src *bufio.Reader, dst []byte) (n int, err error) {
	var b byte
	var part []byte

	for {
		part, err = src.ReadSlice(0xff)
		if (err == io.EOF && len(part) == 0) || err != nil {
			return n, errors.Trace(err)
		}
		// fu.Log.Debugf("bufferReadPacket readFF=%x", part)
		pl := len(part)
		// TODO check n+pl overflow
		n += copy(dst[n:], part[:pl-1])
		// fu.Log.Debugf("bufferReadPacket append %02d dst=%x", pl-1, dst[:n])
		if b, err = src.ReadByte(); err != nil {
			return n, errors.Trace(err)
		}
		// fu.Log.Debugf("bufferReadPacket readByte=%02x", b)
		switch b {
		case 0x00:
			if b, err = src.ReadByte(); err != nil {
				return n, errors.Trace(err)
			}
			// fu.Log.Debugf("bufferReadPacket seq=ff00 chk=%02x", b)
			dst[n] = b
			n++
			// fu.Log.Debugf("bufferReadPacket dst=%x next=copy,return", dst[:n])
			return n, nil
		case 0xff:
			dst[n] = b
			n++
			// fu.Log.Debugf("bufferReadPacket seq=ffff dst=%x", dst[:n])
		default:
			err = errors.NotValidf("bufferReadPacket unknown sequence ff %x", b)
			return n, err
		}
	}
}

func (fu *fileUart) resetRead() (err error) {
	fu.br.Reset(fu.r)
	if err = fu.set9(false); err != nil {
		return errors.Trace(err)
	}
	return nil
}

const (
	//lint:ignore U1000 unused
	cBOTHER = 0x1000
	cCMSPAR = 0x40000000

	cNCCS              = 19
	cASYNC_LOW_LATENCY = 1 << 13

	cFIONREAD    = 0x541b
	cTCSBRKP     = 0x5425
	cTIOCGSERIAL = 0x541E
	cTIOCSSERIAL = 0x541F
	cTCSETS2     = 0x402c542b
	//lint:ignore U1000 unused
	cTCSETSW2 = 0x402c542c // flush output TODO verify
	cTCSETSF2 = 0x402c542d // flush both input,output TODO verify
)

type cc_t byte
type speed_t uint32
type tcflag_t uint32
type termios2 struct {
	c_iflag tcflag_t // input mode flags
	//lint:ignore U1000 unused
	c_oflag tcflag_t // output mode flags
	c_cflag tcflag_t // control mode flags
	c_lflag tcflag_t // local mode flags
	//lint:ignore U1000 unused
	c_line   cc_t        // line discipline
	c_cc     [cNCCS]cc_t // control characters
	c_ispeed speed_t     // input speed
	c_ospeed speed_t     // output speed
}
type serial_info struct {
	_type          int32  //lint:ignore U1000 unused
	line           int32  //lint:ignore U1000 unused
	port           uint32 //lint:ignore U1000 unused
	irq            int32  //lint:ignore U1000 unused
	flags          int32
	xmit_fifo_size int32     //lint:ignore U1000 unused
	_pad           [200]byte //lint:ignore U1000 unused
}

type fdReader struct {
	fd      uintptr
	timeout time.Duration
}

func (fu fdReader) Read(p []byte) (n int, err error) {
	err = io_wait_read(fu.fd, 1, fu.timeout)
	if err != nil {
		return 0, errors.Trace(err)
	}
	// TODO bench optimist read, then io_wait if needed
	n, err = syscall.Read(int(fu.fd), p)
	if err != nil {
		err = errors.Trace(err)
	}
	return n, err
}

func checksum(bs []byte) byte {
	var chk byte
	for _, b := range bs {
		chk += b
	}
	return chk
}

func ioctl(fd uintptr, op, arg uintptr) (err error) {
	if fd+1 == 0 { // mock for test
		return nil
	}
retry:
	r, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, op, arg)
	if errno == syscall.EINTR {
		goto retry
	}
	if errno != 0 {
		err = os.NewSyscallError("SYS_IOCTL", errno)
	} else if r != 0 {
		err = errors.New("unknown error from SYS_IOCTL")
	}
	// if err != nil {
	// 	log.Printf("mdb.ioctl op=%x arg=%x err=%s", op, arg, err)
	// }
	return errors.Annotate(err, "ioctl")
}

func io_wait_read(fd uintptr, min int, wait time.Duration) error {
	var err error
	var out int
	tbegin := time.Now()
	tfinal := tbegin.Add(wait)
	for {
		err = ioctl(fd, uintptr(cFIONREAD), uintptr(unsafe.Pointer(&out)))
		if err != nil {
			return errors.Trace(err)
		}
		if out >= min {
			return nil
		}
		time.Sleep(wait / 16)
		if time.Now().After(tfinal) {
			return errors.Timeoutf("mdb io_wait_read timeout")
		}
	}
}
