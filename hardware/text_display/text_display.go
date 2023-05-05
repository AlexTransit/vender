package text_display

import (
	"bytes"
	"fmt"
	"unsafe"

	"sync"
	"sync/atomic"
	"time"

	"github.com/AlexTransit/vender/internal/types"
	"github.com/juju/errors"
	"github.com/paulrosania/go-charset/charset"
	_ "github.com/paulrosania/go-charset/data"
	"github.com/temoto/alive/v2"
)

const MaxWidth = 40

var spaceBytes = bytes.Repeat([]byte{' '}, MaxWidth)

type TextDisplay struct { //nolint:maligned
	alive *alive.Alive
	mu    sync.Mutex
	dev   Devicer
	tr    atomic.Value
	width uint32
	state State

	tickd time.Duration
	tick  uint32
	upd   chan<- State
}

type TextDisplayConfig struct {
	Codepage    string
	ScrollDelay time.Duration
	Width       uint32
}

type Devicer interface {
	Clear()
	// Control() Control
	// SetControl(new Control) Control
	CursorYX(y, x uint8) bool
	Write(b []byte)
}

func NewTextDisplay(opt *TextDisplayConfig) (*TextDisplay, error) {
	if opt == nil {
		panic("code error TODO make default TextDisplayConfig")
	}
	td := &TextDisplay{
		alive: alive.NewAlive(),
		tickd: opt.ScrollDelay,
		width: uint32(opt.Width),
	}

	if opt.Codepage != "" {
		if err := td.SetCodepage(opt.Codepage); err != nil {
			return nil, errors.Trace(err)
		}
	}

	return td, nil
}

func (td *TextDisplay) SetCodepage(cp string) error {
	td.mu.Lock()
	defer td.mu.Unlock()

	tr, err := charset.TranslatorTo(cp)
	if err != nil {
		return err
	}
	td.tr.Store(tr)
	return nil
}
func (td *TextDisplay) SetDevice(dev Devicer) {
	td.mu.Lock()
	defer td.mu.Unlock()

	td.dev = dev
}
func (td *TextDisplay) SetScrollDelay(d time.Duration) {
	td.mu.Lock()
	defer td.mu.Unlock()

	td.tickd = d
}

func (td *TextDisplay) Clear() {
	td.mu.Lock()
	defer td.mu.Unlock()

	td.state.Clear()
	td.flush()
}

func (td *TextDisplay) Message(s1, s2 string, wait func()) {
	next := State{
		L1: td.Translate(s1),
		L2: td.Translate(s2),
	}

	td.mu.Lock()
	prev := td.state
	td.state = next
	// atomic.StoreUint32(&td.tick, 0)
	td.flush()
	td.mu.Unlock()

	wait()

	td.mu.Lock()
	td.state = prev
	td.flush()
	td.mu.Unlock()
}

// nil: don't change
// len=0: set empty
func (td *TextDisplay) SetLinesBytes(b1, b2 []byte) {
	td.mu.Lock()
	defer td.mu.Unlock()

	if b1 != nil {
		td.state.L1 = b1
	}
	if b2 != nil {
		td.state.L2 = b2
	}
	atomic.StoreUint32(&td.tick, 0)
	td.flush()
}

func (td *TextDisplay) SetLines(line1, line2 string) {
	td.SetLinesBytes(String2ByteSlice(line1), String2ByteSlice(line2))
	// td.SetLinesBytes(
	// 	td.Translate(line1),
	// 	td.Translate(line2))
	if types.VMC.HW.Display.L1 != line1 {
		types.VMC.HW.Display.L1 = line1
		types.Log.NoticeF(fmt.Sprintf("Display.L1=%s", line1))
		// types.TeleN.Log(fmt.Sprintf("Display.L1=%s", line1))
	}
	if types.VMC.HW.Display.L2 != line2 {
		types.VMC.HW.Display.L2 = line2
		types.Log.NoticeF(fmt.Sprintf("Display.L2=%s", line2))
	}
}

func String2ByteSlice(str string) []byte {
	if str == "" {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(str), len(str))
}

func (td *TextDisplay) Tick() {
	td.mu.Lock()
	defer td.mu.Unlock()

	atomic.AddUint32(&td.tick, 1)
	td.flush()
}

func (td *TextDisplay) Run() {
	td.mu.Lock()
	delay := td.tickd
	td.mu.Unlock()
	if delay == 0 {
		return
	}
	tmr := time.NewTicker(delay)
	stopch := td.alive.StopChan()

	for td.alive.IsRunning() {
		select {
		case <-tmr.C:
			td.Tick()
		case <-stopch:
			tmr.Stop()
			return
		}
	}
}

// sometimes returns slice into shared spaceBytes
// sometimes returns `b` (len>=width-1)
// sometimes allocates new buffer
func (td *TextDisplay) JustCenter(b []byte) []byte {
	l := len(b)
	w := int(atomic.LoadUint32(&td.width))

	// optimize short paths
	if l == 0 {
		return spaceBytes[:w]
	}
	if l >= w-1 {
		return b
	}
	padtotal := w - l
	n := padtotal / 2
	padleft := spaceBytes[:n]
	padright := spaceBytes[:n+padtotal%2] // account for odd length
	buf := make([]byte, 0, w)
	buf = append(append(append(buf, padleft...), b...), padright...)
	return buf
}

// returns `b` when len>=width
// otherwise pads with spaces
func (td *TextDisplay) PadRight(b []byte) []byte {
	return PadSpace(b, td.width)
}

func (td *TextDisplay) Translate(s string) []byte {
	if len(s) == 0 {
		return spaceBytes[:0]
	}

	// pad by default, \x00 marks place for cursor
	pad := true
	if s[len(s)-1] == '\x00' {
		pad = false
		s = s[:len(s)-1]
	}

	result := []byte(s)
	tr, ok := td.tr.Load().(charset.Translator)
	if ok && tr != nil {
		_, tb, err := tr.Translate(result, true)
		if err != nil {
			panic(err)
		}
		// translator reuses single internal buffer, make a copy
		result = append([]byte(nil), tb...)
	}

	if pad {
		result = td.PadRight(result)
	}
	return result
}

func (td *TextDisplay) SetUpdateChan(ch chan<- State) {
	td.upd = ch
}

func (td *TextDisplay) State() State { return td.state.Copy() }

func (td *TextDisplay) flush() {
	var buf1 [MaxWidth]byte
	var buf2 [MaxWidth]byte
	b1 := buf1[:td.width]
	b2 := buf2[:td.width]
	tick := atomic.LoadUint32(&td.tick)
	n1 := scrollWrap(b1, td.state.L1, tick)
	n2 := scrollWrap(b2, td.state.L2, tick)

	// === Option 1: clear
	// td.dev.Clear()
	// td.dev.Write(b1[:n1])
	// td.dev.CursorYX(2, 1)
	// td.dev.Write(b2[:n2])

	// === Option 2: rewrite without clear, looks smoother
	// no padding: "erase" modified area, for now - whole line
	if n1 < td.width {
		td.dev.CursorYX(1, 1)
		td.dev.Write(spaceBytes[:td.width])
	}
	if len(td.state.L1) > 0 {
		td.dev.CursorYX(1, 1)
		td.dev.Write(b1[:n1])
	}
	// no padding: "erase" modified area, for now - whole line
	if n2 < td.width {
		td.dev.CursorYX(2, 1)
		td.dev.Write(spaceBytes[:td.width])
	}
	if len(td.state.L2) > 0 {
		td.dev.CursorYX(2, 1)
		td.dev.Write(b2[:n2])
	}

	if td.upd != nil {
		td.upd <- td.state.Copy()
	}
}

type State struct {
	L1, L2 []byte
}

func (s *State) Clear() {
	s.L1 = nil
	s.L2 = nil
}

func (s State) Copy() State {
	return State{
		L1: append([]byte(nil), s.L1...),
		L2: append([]byte(nil), s.L2...),
	}
}

func (s State) Format(width uint32) string {
	return fmt.Sprintf("%s\n%s",
		PadSpace(s.L1, width),
		PadSpace(s.L2, width),
	)
}

func (s State) String() string {
	return fmt.Sprintf("%s\n%s", s.L1, s.L2)
}

func PadSpace(b []byte, width uint32) []byte {
	l := uint32(len(b))

	if l == 0 {
		return spaceBytes[:width]
	}
	if l >= width {
		return b
	}
	buf := make([]byte, 0, width)
	buf = append(append(buf, b...), spaceBytes[:width-l]...)
	return buf
}

// relies that len(buf) == display width
func scrollWrap(buf []byte, content []byte, tick uint32) uint32 {
	length := uint32(len(content))
	width := uint32(len(buf))
	gap := uint32(width / 2)
	n := 0
	if length <= width {
		n = copy(buf, content)
		copy(buf[n:], spaceBytes)
		return uint32(n)
	}

	offset := tick % (length + gap)
	if offset < length {
		n = copy(buf, content[offset:])
	} else {
		gap = gap - (offset - length)
	}
	n += copy(buf[n:], spaceBytes[:gap])
	n += copy(buf[n:], content[0:])
	return uint32(n)
}
