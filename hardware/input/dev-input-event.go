package input

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/AlexTransit/vender/internal/types"
	"github.com/temoto/inputevent-go"
)

const DevInputEventTag = "dev-input-event"

type DevInputEventSource struct {
	f io.ReadCloser
}

// compile-time interface compliance test
var _ Source = new(DevInputEventSource)

func (ds *DevInputEventSource) String() string { return DevInputEventTag }

func NewDevInputEventSource(device string) (*DevInputEventSource, error) {
	if device == "" {
		const pgpio = "/dev/input/by-path/"
		entries, err := os.ReadDir(pgpio)
		if err != nil {
			return nil, errors.New("service key not work. set device name manualy")
		}
		for i := range entries {
			file := entries[i].Name()
			if strings.Contains(file, "gpio") {
				device = pgpio + file
				break
			}
		}
	}
	f, err := os.Open(device)
	if err != nil {
		return nil, err
	}
	return &DevInputEventSource{f: f}, nil
}

func (ds *DevInputEventSource) Read() (types.InputEvent, error) {
	for {
		ie, err := inputevent.ReadOne(ds.f)
		if err != nil {
			// g.Log.Errorf("%s err=%v", DevInputEventTag, err)
			return types.InputEvent{}, err
		}
		if ie.Type == inputevent.EV_KEY {
			// g.Log.Debugf("%s key=%v", DevInputEventTag, ie.Code)
			ev := types.InputEvent{
				Source: DevInputEventTag,
				Key:    types.InputKey(ie.Code),
				Up:     ie.Value == int32(inputevent.KeyStateUp),
			}
			return ev, nil
		}
	}
}
