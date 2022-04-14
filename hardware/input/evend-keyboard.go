package input

import (
	"io"

	"github.com/AlexTransit/vender/hardware/mega-client"
	"github.com/AlexTransit/vender/internal/types"
)

const EvendKeyMaskUp = 0x80
const EvendKeyboardSourceTag = "evend-keyboard"

const (
	EvendKeyAccept    types.InputKey = 13
	EvendKeyReject    types.InputKey = 27
	EvendKeyCreamLess types.InputKey = 'A'
	EvendKeyCreamMore types.InputKey = 'B'
	EvendKeySugarLess types.InputKey = 'C'
	EvendKeySugarMore types.InputKey = 'D'
	evendKeyDotInput  types.InputKey = 'E' // evend keyboard sends '.' as 'E'
	EvendKeyDot       types.InputKey = '.'
)

type EvendKeyboard struct{ c *mega.Client }

// compile-time interface compliance test
var _ Source = new(EvendKeyboard)

func NewEvendKeyboard(client *mega.Client) (*EvendKeyboard, error) {
	ek := &EvendKeyboard{c: client}
	ek.c.IncRef(EvendKeyboardSourceTag)

drain:
	for {
		select {
		case <-ek.c.TwiChan:
		default:
			break drain
		}
	}
	return ek, nil
}
func (ek *EvendKeyboard) Close() error {
	return ek.c.DecRef(EvendKeyboardSourceTag)
}

func (ek *EvendKeyboard) String() string { return EvendKeyboardSourceTag }

func (ek *EvendKeyboard) Read() (types.InputEvent, error) {
	for {
		v16, ok := <-ek.c.TwiChan
		if !ok {
			return types.InputEvent{}, io.EOF
		}
		key, up := types.InputKey(v16&^EvendKeyMaskUp), v16&EvendKeyMaskUp != 0
		// key replace table
		switch key {
		case evendKeyDotInput:
			key = EvendKeyDot
		}
		if !up {
			e := types.InputEvent{
				Source: EvendKeyboardSourceTag,
				Key:    key,
				Up:     up,
			}
			return e, nil
		}
	}
}

func IsAccept(e *types.InputEvent) bool {
	return e.Source == EvendKeyboardSourceTag && e.Key == EvendKeyAccept
}
func IsReject(e *types.InputEvent) bool {
	return e.Source == EvendKeyboardSourceTag && e.Key == EvendKeyReject
}
