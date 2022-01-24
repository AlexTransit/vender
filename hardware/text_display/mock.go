package text_display

func NewMockTextDisplay(opt *TextDisplayConfig) *TextDisplay {
	dev := new(MockDevicer)
	display, err := NewTextDisplay(opt)
	if err != nil {
		// t.Fatal(err)
		panic(err)
	}
	display.dev = dev
	return display
}

type MockDevicer struct {
	// c uint32
}

func (m *MockDevicer) Clear() {}

// func (m *MockDevicer) Control() Control {
// 	return Control(atomic.LoadUint32(&m.c))
// }

func (m *MockDevicer) CursorYX(y, x uint8) bool { return true }

// func (m *MockDevicer) SetControl(c Control) Control {
// 	return Control(atomic.SwapUint32(&m.c, uint32(c)))
// }

func (m *MockDevicer) Write(b []byte) {}
