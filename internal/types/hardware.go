package types

import (
	"fmt"
	// "github.com/AlexTransit/vender/internal/global"
)

type DeviceOfflineError struct {
	Device Devicer
}

func (oe DeviceOfflineError) Error() string {
	// _ = global.ChSetEnvB(oe.Device.Name()+".working", false)
	return fmt.Sprintf("%s is offline", oe.Device.Name())
}

type Devicer interface {
	Name() string
}
