package tele

import (
	"context"
	"time"

	"github.com/AlexTransit/vender/log2"
	tele_api "github.com/AlexTransit/vender/tele"
	tele_config "github.com/AlexTransit/vender/tele/config"
	"github.com/golang/protobuf/proto"
	"github.com/juju/errors"
)

const (
	defaultStateInterval  = 5 * time.Minute
	DefaultNetworkTimeout = 30 * time.Second
)

// Tele contract:
// - Init() fails only with invalid config, network issues ignored
// - Transaction/Error/Service/etc public API calls block at most for disk write
//   network may be slow or absent, messages will be delivered in background
// - Close() will block until all messages are delivered
// - Telemetry/Response messages delivered at least once
// - Status messages may be lost
type tele struct { //nolint:maligned
	config       tele_config.Config
	log          *log2.Log
	transport    Transporter
	vmId         int32
	stat         tele_api.Stat
	currentState tele_api.State
}

func New() tele_api.Teler {
	return &tele{}
}
func NewWithTransporter(trans Transporter) tele_api.Teler {
	return &tele{transport: trans}
}

func (t *tele) Init(ctx context.Context, log *log2.Log, teleConfig tele_config.Config) error {
	t.config = teleConfig
	t.log = log
	if t.config.LogDebug {
		t.log.SetLevel(log2.LDebug)
	}

	t.vmId = int32(t.config.VmId)
	t.stat.Locked_Reset()

	if t.transport == nil { // production path
		t.transport = &transportMqtt{}
	}
	if err := t.transport.Init(ctx, log, teleConfig, t.onCommandMessage, t.messageForRobot); err != nil {
		return errors.Annotate(err, "tele transport")
	}
	if !t.config.Enabled {
		return nil
	}

	t.State(tele_api.State_Boot)

	return nil
}

func (t *tele) Close() {
	t.transport.CloseTele()
}

func (t *tele) CommandResponse(r *tele_api.Response) {
	payload, err := proto.Marshal(r)
	if err != nil {
		t.log.Errorf("CRITICAL command response Marshal tm=%#v err=%v", r, err)
		return
	}
	r.INTERNALTopic = "cr"
	t.transport.SendCommandResponse(r.INTERNALTopic, payload)
}

func (t *tele) Telemetry(tm *tele_api.Telemetry) {
	if tm.VmId == 0 {
		tm.VmId = t.vmId
	}
	if tm.Time == 0 {
		tm.Time = time.Now().UnixNano()
	}

	payload, err := proto.Marshal(tm)
	if err != nil {
		t.log.Errorf("CRITICAL telemetry Marshal tm=%#v err=%v", tm, err)
		return
	}
	t.transport.SendTelemetry(payload)
}

func (t *tele) RoboSend(sm *tele_api.FromRoboMessage) {
	t.marshalAndSendMessage(sm)
}

func (t *tele) RoboSendState(s tele_api.State) {
	if t.currentState == s {
		return
	}
	t.currentState = s
	rm := tele_api.FromRoboMessage{
		State:                s,
	}
	t.marshalAndSendMessage(&rm)
}

func (t *tele) marshalAndSendMessage(m proto.Message) {
	payload, err := proto.Marshal(m)
	if err != nil {
		t.log.Errorf("CRITICAL telemetry Marshal message(%#v) err=%v", m, err)
		return
	}
	t.log.Infof("telemetry messga: %v", m)
	t.transport.SendFromRobot(payload)
}
