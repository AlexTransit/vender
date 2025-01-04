package hd44780

import (
	"strconv"
	"time"

	"github.com/temoto/gpio-cdev-go"
)

type Command byte

const (
	CommandClear   Command = 0x01
	CommandReturn  Command = 0x02
	CommandControl Command = 0x08
	CommandAddress Command = 0x80
)

type Control byte

const (
	ControlOn         Control = 0x04
	ControlUnderscore Control = 0x02
	ControlBlink      Control = 0x01
)
const ddramWidth = 0x40

type LCD struct {
	control Control
	pinChip gpio.Chiper
	pins    gpio.Lineser
	pin_rs  gpio.LineSetFunc // command/data, aliases: A0, RS
	pin_rw  gpio.LineSetFunc // read/write
	pin_e   gpio.LineSetFunc // enable
	pin_d4  gpio.LineSetFunc
	pin_d5  gpio.LineSetFunc
	pin_d6  gpio.LineSetFunc
	pin_d7  gpio.LineSetFunc
}

type PinMap struct {
	RS string `hcl:"rs"`
	RW string `hcl:"rw"`
	E  string `hcl:"e"`
	D4 string `hcl:"d4"`
	D5 string `hcl:"d5"`
	D6 string `hcl:"d6"`
	D7 string `hcl:"d7"`
}

func (lcd *LCD) Init(chipName string, pinmap PinMap, page1 bool) error {
	var err error
	lcd.pinChip, err = gpio.Open(chipName, "lcd")
	if err != nil {
		return err
	}
	nRS := mustAtou32(pinmap.RS)
	nRW := mustAtou32(pinmap.RW)
	nE := mustAtou32(pinmap.E)
	nD4 := mustAtou32(pinmap.D4)
	nD5 := mustAtou32(pinmap.D5)
	nD6 := mustAtou32(pinmap.D6)
	nD7 := mustAtou32(pinmap.D7)
	lcd.pins, err = lcd.pinChip.OpenLines(
		gpio.GPIOHANDLE_REQUEST_OUTPUT, "lcd",
		nRS, nRW, nE, nD4, nD5, nD6, nD7,
	)
	if err != nil {
		return err
	}
	lcd.pin_rs = lcd.pins.SetFunc(nRS)
	lcd.pin_rw = lcd.pins.SetFunc(nRW)
	lcd.pin_e = lcd.pins.SetFunc(nE)
	lcd.pin_d4 = lcd.pins.SetFunc(nD4)
	lcd.pin_d5 = lcd.pins.SetFunc(nD5)
	lcd.pin_d6 = lcd.pins.SetFunc(nD6)
	lcd.pin_d7 = lcd.pins.SetFunc(nD7)

	lcd.init4(page1)
	return nil
}

func (lcd *LCD) setAllPins(b byte) {
	lcd.pin_rs(b)
	lcd.pin_rw(b)
	lcd.pin_e(b)
	lcd.pin_d4(b)
	lcd.pin_d5(b)
	lcd.pin_d6(b)
	lcd.pin_d7(b)
	lcd.pins.Flush() //nolint:errcheck
}

func (lcd *LCD) blinkE() {
	lcd.pin_e(1)
	// FIXME check error
	lcd.pins.Flush() //nolint:errcheck
	time.Sleep(1 * time.Microsecond)
	lcd.pin_e(0)
	// FIXME check error
	lcd.pins.Flush() //nolint:errcheck
	time.Sleep(1 * time.Microsecond)
}

func (lcd *LCD) send4(rs, d4, d5, d6, d7 byte) {
	// log.Printf("sn4 %v %v %v %v %v\n", rs, d7, d6, d5, d4)
	lcd.pin_rs(rs)
	lcd.pin_d4(d4)
	lcd.pin_d5(d5)
	lcd.pin_d6(d6)
	lcd.pin_d7(d7)
	lcd.blinkE()
}

func (lcd *LCD) init4(page1 bool) {
	time.Sleep(20 * time.Millisecond)

	// special sequence
	lcd.Command(0x33)
	lcd.Command(0x32)

	lcd.SetFunction(false, page1)
	lcd.SetControl(0) // off
	lcd.SetControl(ControlOn)
	lcd.Clear()
	lcd.SetEntryMode(true, false)
}

func bb(b, bit byte) byte {
	if b&(1<<bit) == 0 {
		return 0
	}
	return 1
}

func (lcd *LCD) Command(c Command) {
	b := byte(c)
	// log.Printf("cmd %0x\n", b)
	lcd.send4(0, bb(b, 4), bb(b, 5), bb(b, 6), bb(b, 7))
	time.Sleep(40 * time.Microsecond)
	lcd.send4(0, bb(b, 0), bb(b, 1), bb(b, 2), bb(b, 3))
	// TODO poll busy flag
	time.Sleep(40 * time.Microsecond)
	lcd.setAllPins(0)
}

func (lcd *LCD) Data(b byte) {
	// log.Printf("dat %0x\n", b)
	lcd.send4(1, bb(b, 4), bb(b, 5), bb(b, 6), bb(b, 7))
	time.Sleep(40 * time.Microsecond)
	lcd.send4(1, bb(b, 0), bb(b, 1), bb(b, 2), bb(b, 3))
	// TODO poll busy flag
	time.Sleep(40 * time.Microsecond)
	lcd.setAllPins(0)
}

func (lcd *LCD) Write(bs []byte) {
	for _, b := range bs {
		lcd.Data(b)
	}
}

func (lcd *LCD) Clear() {
	lcd.Command(CommandClear)
	// TODO poll busy flag
	time.Sleep(2 * time.Millisecond)
}

func (lcd *LCD) Return() {
	lcd.Command(CommandReturn)
}

func (lcd *LCD) SetEntryMode(right, shift bool) {
	var cmd Command = 0x04
	if right {
		cmd |= 0x02
	}
	if shift {
		cmd |= 0x01
	}
	lcd.Command(cmd)
}

func (lcd *LCD) Control() Control {
	return lcd.control
}

func (lcd *LCD) SetControl(new Control) Control {
	old := lcd.control
	lcd.control = new
	lcd.Command(CommandControl | Command(new))
	return old
}

func (lcd *LCD) SetFunction(bits8, page1 bool) {
	var cmd Command = 0x28
	if bits8 {
		cmd |= 0x10
	}
	if page1 {
		cmd |= 0x02
	}
	lcd.Command(cmd)
}

func (lcd *LCD) CursorYX(row uint8, column uint8) bool {
	if !(row > 0 && row <= 2) {
		return false
	}
	if !(column > 0 && column <= 16) {
		return false
	}
	addr := (row-1)*ddramWidth + (column - 1)
	lcd.Command(CommandAddress | Command(addr))
	return true
}

func mustAtou32(s string) uint32 {
	x, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		panic(err)
	}
	return uint32(x)
}
