package types

import (
	"github.com/AlexTransit/vender/currency"
)

//go:generate stringer -type=EventKind -trimprefix=Event
type EventKind uint8

const (
	EventInvalid EventKind = iota
	EventInput
	EventMoneyPreCredit
	EventMoneyCredit
	EventTime
	EventLock
	EventService
	EventStop
	EventFrontLock
	EventUiTimerStop
	EventBroken
	EventAccept
)

type Event struct {
	Input  InputEvent
	Kind   EventKind
	Amount currency.Amount
}

// func (e *Event) String() string {
// 	inner := ""
// 	switch e.Kind {
// 	case EventInput:
// 		inner = fmt.Sprintf(" source=%s key=%v up=%t", e.Input.Source, e.Input.Key, e.Input.Up)
// 	case EventMoneyCredit, EventMoneyPreCredit:
// 		// inner = fmt.Sprintf(" amount=%s err=%v", e.Amount.Format100I(), e.Money.Err)
// 		inner = fmt.Sprintf(" amount=%d", e.Amount)
// 	}
// 	return fmt.Sprintf("Event(%s%s)", e.Kind.String(), inner)
// }

type InputKey uint16

type InputEvent struct {
	Source string
	Key    InputKey
	Up     bool
}

func (e *InputEvent) IsZero() bool  { return e.Key == 0 }
func (e *InputEvent) IsDigit() bool { return e.Key >= '0' && e.Key <= '9' }
func (e *InputEvent) IsDot() bool   { return e.Key == '.' }

func (e *InputEvent) IsTuneKey() bool { return e.Key >= 65 && e.Key <= 68 } // cream+- sugar+-
