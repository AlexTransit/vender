package types

import (
	"context"
	"fmt"
)

var ErrInterrupted = fmt.Errorf("scheduler interrupted, ignore like EPIPE")

type TaskFunc = func(context.Context) error

type Scheduler interface {
	// Schedule(context.Context, tele_api.Priority, TaskFunc) <-chan error
	ScheduleSync(context.Context, TaskFunc) error
}
