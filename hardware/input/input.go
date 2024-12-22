// Abstract input events
package input

import (
	"fmt"

	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
)

type Source interface {
	Read() (types.InputEvent, error)
	String() string
}

type EventFunc func(types.InputEvent)

type Dispatch struct {
	Log *log2.Log
	Bus chan types.InputEvent
}

func (d *Dispatch) InputChain() *chan types.InputEvent {
	if d == nil {
		d = &Dispatch{
			Bus: make(chan types.InputEvent),
		}
	}
	ch := d.Bus
	return &ch
}

func (d *Dispatch) ReadEvendKeyboard(s Source) {
	for {
		event, err := s.Read()
		if err != nil {
			d.Log.Fatal(errors.ErrorStack(err))
		}
		var kn string
		switch event.Key {
		case EvendKeyAccept:
			kn = "Ok"
		case EvendKeyReject:
			kn = "C"
		case EvendKeyCreamLess:
			kn = "cream-"
		case EvendKeyCreamMore:
			kn = "cream+"
		case EvendKeySugarLess:
			kn = "sugar-"
		case EvendKeySugarMore:
			kn = "sugar+"
		case EvendKeyDot:
			kn = "."
		case 48, 49, 50, 51, 52, 53, 54, 55, 56, 57:
			kn = fmt.Sprintf("%d", event.Key-48)
		default:
			fmt.Printf(" key event (%v)", event)
		}

		if len(d.Bus) == 0 {
			if event.Source == DevInputEventTag || types.VMC.InputEnable {
				d.Log.Infof("key press (%s) ", kn)
				d.Bus <- event
			} else {
				d.Log.Infof("ignore key. input disabled. (%s) ", kn)
			}
		} else {
			d.Log.Infof("ignore key. previous button not handled. (%s) ", kn)
		}
	}
}

func (d *Dispatch) Emit(event types.InputEvent) {
	d.Bus <- event
}
