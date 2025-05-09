package tele

import (
	"context"

	"github.com/AlexTransit/vender/log2"
	tele_config "github.com/AlexTransit/vender/tele/config"
)

// Teler interface Telemetry client, vending machine side.
// Not for external public usage.
type Teler interface {
	Init(context.Context, *log2.Log, tele_config.Config, string) error
	Close()
	Error(error)
	ErrorStr(string)
	Log(string)
	StatModify(func(*Stat))
	Report(ctx context.Context, serviceTag bool) error
	Transaction(*Telemetry_Transaction)
	CommandResponse(*Response)
	RoboSend(*FromRoboMessage)
	RoboSendState(s State)
	RoboConnected() bool
	GetState() State
}

type stub struct{}

func (stub) Init(context.Context, *log2.Log, tele_config.Config, string) error { return nil }
func (stub) Close()                                                            {}
func (stub) Error(error)                                                       {}
func (stub) ErrorStr(string)                                                   {}
func (stub) Log(string)                                                        {}
func (stub) StatModify(func(*Stat))                                            {}
func (stub) Report(ctx context.Context, serviceTag bool) error                 { return nil }
func (stub) Transaction(*Telemetry_Transaction)                                {}
func (stub) CommandResponse(*Response)                                         {}
func (stub) RoboSend(*FromRoboMessage)                                         {}
func (stub) RoboSendState(s State)                                             {}
func (stub) RoboConnected() bool                                               { return false }
func (stub) GetState() State                                                   { return 0 }

func NewStub() Teler { return stub{} }
