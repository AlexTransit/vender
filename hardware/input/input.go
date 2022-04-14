// Abstract input events
package input

import (
	"fmt"
	"sync"

	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
)

func Drain(ch <-chan types.InputEvent) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

type Source interface {
	Read() (types.InputEvent, error)
	String() string
}

type EventFunc func(types.InputEvent)
type sub struct {
	name string
	ch   chan<- types.InputEvent
	fun  EventFunc
	stop <-chan struct{}
}

type Dispatch struct {
	Log  *log2.Log
	bus  chan types.InputEvent
	mu   sync.Mutex
	subs map[string]*sub
	stop <-chan struct{}
}

func NewDispatch(log *log2.Log, stop <-chan struct{}) *Dispatch {
	return &Dispatch{
		Log:  log,
		bus:  make(chan types.InputEvent),
		subs: make(map[string]*sub, 16),
		stop: stop,
	}
}

func (d *Dispatch) Enable(e bool) {
	if types.VMC.HW.Input != e {
		types.VMC.HW.Input = e
		// types.Log.Infof("evendInput = %v", e)
	}
}

func (d *Dispatch) SubscribeChan(name string, substop <-chan struct{}) chan types.InputEvent {
	target := make(chan types.InputEvent)
	sub := &sub{
		name: name,
		ch:   target,
		stop: substop,
	}
	d.safeSubscribe(sub)
	return target
}

func (d *Dispatch) SubscribeFunc(name string, fun EventFunc, substop <-chan struct{}) {
	sub := &sub{
		name: name,
		fun:  fun,
		stop: substop,
	}
	d.safeSubscribe(sub)
}

func (d *Dispatch) Unsubscribe(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if sub, ok := d.subs[name]; ok {
		d.subClose(sub)
	} else {
		panic("code error input sub not found name=" + name)
	}
}

func (d *Dispatch) Run(sources []Source) {
	for _, source := range sources {
		go d.readSource(source)
	}

	for {
		select {
		case event := <-d.bus:
			handled := false
			d.mu.Lock()
			for _, sub := range d.subs {
				d.subFire(sub, event)
				handled = true
			}
			d.mu.Unlock()
			if !handled {
				// TODO emit sound/etc notification
				d.Log.Errorf("input is not handled event=%#v", event)
			}

		case <-d.stop:
			Drain(d.bus)
			return
		}
	}
}

func (d *Dispatch) Emit(event types.InputEvent) {
	select {
	case d.bus <- event:
		// d.Log.Debugf("input emit=%#v", event)
		// d.Log.Infof("key press code(%d) ", event.Key)
	case <-d.stop:
		return
	}
}

func (d *Dispatch) subFire(sub *sub, event types.InputEvent) {
	select {
	case <-sub.stop:
		d.subClose(sub)
		return
	default:
	}

	if sub.ch == nil && sub.fun == nil {
		panic(fmt.Sprintf("input sub=%s ch=nil fun=nil", sub.name))
	}
	if sub.fun != nil {
		sub.fun(event)
	}
	if sub.ch != nil {
		select {
		case sub.ch <- event:
		case <-sub.stop:
			d.subClose(sub)
		}
	}
}

func (d *Dispatch) subClose(s *sub) {
	if s.ch != nil {
		close(s.ch)
	}
	delete(d.subs, s.name)
}

func (d *Dispatch) safeSubscribe(s *sub) {
	d.mu.Lock()
	if existing, ok := d.subs[s.name]; ok {
		select {
		case <-s.stop:
			panic("code error input subscribe already closed name=" + s.name)
		case <-existing.stop:
			d.subClose(existing)
		default:
			panic("code error input duplicate subscribe name=" + s.name)
		}
	}
	d.subs[s.name] = s
	d.mu.Unlock()
}

func (d *Dispatch) readSource(source Source) {
	tag := source.String()
	for {
		event, err := source.Read()
		if err != nil {
			err = errors.Annotatef(err, "input source=%s", tag)
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
		}

		if types.VMC.HW.Input || event.Source == "dev-input-event" {
			d.Log.Infof("key press (%s) ", kn)
			d.Emit(event)
		} else {
			d.Log.Debugf("keyboard disable. ignore key (%s)", kn)
		}
	}
}
