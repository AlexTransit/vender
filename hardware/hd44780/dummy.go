package hd44780

type DummyLCD struct{}

func NewDummy() *DummyLCD { return &DummyLCD{} }

func (d *DummyLCD) Clear() {}

func (d *DummyLCD) CursorYX(_, _ uint8) bool { return true }

func (d *DummyLCD) Write(_ []byte) {}
