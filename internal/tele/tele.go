package tele

import (
	"context"
	"time"

	"github.com/AlexTransit/vender/log2"
	tele_api "github.com/AlexTransit/vender/tele"
	tele_config "github.com/AlexTransit/vender/tele/config"
	"github.com/golang/protobuf/proto"
	"github.com/juju/errors"
	"github.com/temoto/spq"
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
	config    tele_config.Config
	log       *log2.Log
	transport Transporter
	q         *spq.Queue
	// stateCh      chan tele_api.State
	stopCh       chan struct{}
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

	// t.stopCh = make(chan struct{})
	// t.stateCh = make(chan tele_api.State)
	t.vmId = int32(t.config.VmId)
	t.stat.Locked_Reset()

	// willPayload := []byte{byte(tele_api.State_Disconnected)}
	// test code sets .transport
	if t.transport == nil { // production path
		t.transport = &transportMqtt{}
	}
	if err := t.transport.Init(ctx, log, teleConfig, t.onCommandMessage); err != nil {
		return errors.Annotate(err, "tele transport")
	}
	if !t.config.Enabled {
		return nil
	}

	if t.config.PersistPath == "" {
		panic("code error must set t.config.PersistPath")
	}
	var err error
	t.q, err = spq.Open(t.config.PersistPath)
	if err != nil {
		return errors.Annotate(err, "tele queue")
	}

	go t.qworker()
	t.State(tele_api.State_Boot)

	return nil
}

func (t *tele) Close() {
	// close(t.stopCh)
	if t.q != nil {
		t.q.Close()
	}
	t.transport.CloseTele()
}

// denote value type in persistent queue bytes form
const (
	qCommandResponse byte = 1
	qTelemetry       byte = 2
)

func (t *tele) qworker() {
	for {
		box, err := t.q.Peek()
		switch err {
		case nil:
			// success path
			b := box.Bytes()
			// t.log.Debugf("q.peek %x", b)
			var del bool
			del, err = t.qhandle(b)
			if err != nil {
				t.log.Errorf("tele qhandle b=%x err=%v", b, err)
			}
			if del {
				if err = t.q.Delete(box); err != nil {
					t.log.Errorf("tele qhandle Delete b=%x err=%v", b, err)
				}
			} else {
				if err = t.q.DeletePush(box); err != nil {
					t.log.Errorf("tele qhandle DeletePush b=%x err=%v", b, err)
				}
			}

		case spq.ErrClosed:
			select {
			case <-t.stopCh: // success path
			default:
				t.log.Errorf("CRITICAL tele spq closed unexpectedly")
				// TODO try to send telemetry?
			}
			return

		default:
			t.log.Errorf("CRITICAL tele spq err=%v", err)
			// here will go yet unhandled shit like disk full
		}
	}
}

func (t *tele) qhandle(b []byte) (bool, error) {

	if len(b) == 0 {
		t.log.Errorf("tele spq peek=empty")
		// what else can we do?
		return true, nil
	}

	switch b[0] {
	case qCommandResponse:
		var r tele_api.Response
		if err := proto.Unmarshal(b[1:], &r); err != nil {
			return true, err
		}
		return t.qsendResponse(&r), nil

	case qTelemetry:
		var tm tele_api.Telemetry
		if err := proto.Unmarshal(b[1:], &tm); err != nil {
			return true, err
		}
		return t.qsendTelemetry(&tm), nil

	default:
		err := errors.Errorf("unknown kind=%d", b[0])
		return true, err
	}
}

func (t *tele) qpushCommandResponse(c *tele_api.Command, r *tele_api.Response) error {
	// c.ReplyTopic = "cr"
	// r.INTERNALTopic = c.ReplyTopic
	r.INTERNALTopic = "cr"

	return t.qpushTagProto(qCommandResponse, r)
}

func (t *tele) qpushTelemetry(tm *tele_api.Telemetry) error {
	if tm.VmId == 0 {
		tm.VmId = t.vmId
	}
	if tm.Time == 0 {
		tm.Time = time.Now().UnixNano()
	}
	t.stat.Lock()
	defer t.stat.Unlock()
	tm.Stat = &t.stat.Telemetry_Stat
	err := t.qpushTagProto(qTelemetry, tm)
	t.stat.Locked_Reset()
	return err
}

func (t *tele) qpushTagProto(tag byte, pb proto.Message) error {
	buf := proto.NewBuffer(make([]byte, 0, 1024))
	if err := buf.EncodeVarint(uint64(tag)); err != nil {
		return err
	}
	if err := buf.Marshal(pb); err != nil {
		return err
	}
	// t.log.Debugf("qpushTagProto %x", buf.Bytes())
	return t.q.Push(buf.Bytes())
}

func (t *tele) qsendResponse(r *tele_api.Response) bool {
	// do not serialize INTERNAL_topic field
	wireResponse := *r
	wireResponse.INTERNALTopic = ""
	payload, err := proto.Marshal(&wireResponse)
	if err != nil {
		t.log.Errorf("CRITICAL response Marshal r=%#v err=%v", r, err)
		return true // retry will not help
	}
	return t.transport.SendCommandResponse(r.INTERNALTopic, payload)
}

func (t *tele) qsendTelemetry(tm *tele_api.Telemetry) bool {
	payload, err := proto.Marshal(tm)
	if err != nil {
		t.log.Errorf("CRITICAL telemetry Marshal tm=%#v err=%v", tm, err)
		return true // retry will not help
	}
	// t.log.Debugf("SendTelemetry %x", payload)
	return t.transport.SendTelemetry(payload)
}
