package evend

import (
	"context"
)

type DeviceElevator struct { //nolint:maligned
	MiherElevator
}

func (e *DeviceElevator) init(ctx context.Context) error {
	return e.InitMiherElevator(ctx, 0xd0, "elevator", proto1)
}
