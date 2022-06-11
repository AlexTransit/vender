package tele

import (
	"context"
	"fmt"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/log2"
	tele_config "github.com/AlexTransit/vender/tele/config"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type transportMqtt struct {
	enabled   bool
	log       *log2.Log
	onCommand func([]byte) bool
	inRobo    func([]byte) bool
	m         mqtt.Client
	mopt      *mqtt.ClientOptions
	// stopCh    chan struct{}

	topicPrefix    string
	topicConnect   string
	topicState     string
	topicTelemetry string
	topicCommand   string
	topicRoboIn    string
	topicRoboOut   string
}

func (tm *transportMqtt) Init(ctx context.Context, log *log2.Log, teleConfig tele_config.Config, onCommand CommandCallback, inRobo CommandCallback) error {
	if !teleConfig.Enabled {
		return nil
	}
	tm.enabled = true
	tm.log = log
	// AlexM FIXME add loglevel to config
	mqtt.ERROR = log
	mqtt.CRITICAL = log
	mqtt.WARN = log
	//	mqtt.DEBUG = log
	mqttClientId := fmt.Sprintf("vm%d", teleConfig.VmId)
	credFun := func() (string, string) {
		return mqttClientId, teleConfig.MqttPassword
	}

	tm.onCommand = func(payload []byte) bool {
		return onCommand(ctx, payload)
	}

	tm.inRobo = func(payload []byte) bool {
		return inRobo(ctx, payload)
	}
	tm.topicPrefix = mqttClientId // coincidence
	tm.topicConnect = fmt.Sprintf("%s/c", tm.topicPrefix)
	tm.topicState = fmt.Sprintf("%s/w/1s", tm.topicPrefix)
	tm.topicTelemetry = fmt.Sprintf("%s/w/1t", tm.topicPrefix)
	tm.topicCommand = fmt.Sprintf("%s/r/c", tm.topicPrefix)
	tm.topicRoboIn = fmt.Sprintf("%s/ri", tm.topicPrefix)
	tm.topicRoboOut = fmt.Sprintf("%s/ro", tm.topicPrefix)
	keepAlive := helpers.IntSecondConfigDefault(teleConfig.KeepaliveSec, 60)
	pingTimeout := helpers.IntSecondConfigDefault(teleConfig.PingTimeoutSec, 30)
	retryInterval := helpers.IntSecondConfigDefault(teleConfig.KeepaliveSec/2, 30)
	storePath := teleConfig.StorePath
	if teleConfig.StorePath == "" {
		storePath = "/home/vmc/telemessages"
	}
	// tlsconf := new(tls.Config)
	// if teleConfig.TlsCaFile != "" {
	// 	tlsconf.RootCAs = x509.NewCertPool()
	// 	cabytes, err := ioutil.ReadFile(teleConfig.TlsCaFile)
	// 	if err != nil {
	// 		tm.log.Errorf("tls not possible. certivicate file - not found")
	// 	}
	// 	tlsconf.RootCAs.AppendCertsFromPEM(cabytes)
	// }
	// if teleConfig.TlsPsk != "" {
	// 	copy(tlsconf.SessionTicketKey[:], helpers.MustHex(teleConfig.TlsPsk))
	// }
	tm.mopt = mqtt.NewClientOptions().
		AddBroker(teleConfig.MqttBroker).
		SetBinaryWill(tm.topicConnect, []byte{0x00}, 1, true).
		SetCleanSession(false).
		SetClientID(mqttClientId).
		SetCredentialsProvider(credFun).
		SetDefaultPublishHandler(tm.messageHandler).
		SetKeepAlive(keepAlive).
		SetPingTimeout(pingTimeout).
		SetOrderMatters(false).
		// SetTLSConfig(tlsconf).
		SetResumeSubs(true).SetCleanSession(false).
		SetStore(mqtt.NewFileStore(storePath)).
		SetConnectRetryInterval(retryInterval).
		SetOnConnectHandler(tm.onConnectHandler).
		SetConnectionLostHandler(tm.connectLostHandler).
		SetConnectRetry(true)
	tm.m = mqtt.NewClient(tm.mopt)
	sConnToken := tm.m.Connect()
	// if sConnToken.Wait() && sConnToken.Error() != nil {
	if sConnToken.Error() != nil {
		tm.log.Errorf("token.Error\n")
	}
	return nil
}

func (tm *transportMqtt) CloseTele() {
	tm.log.Infof("mqtt unsubscribe")
	if token := tm.m.Unsubscribe(tm.topicCommand); token.Wait() && token.Error() != nil {
		tm.log.Infof("mqtt unsubscribe error")
	}
}

func (tm *transportMqtt) publish2Telemetry(topic string, qos byte, retained bool, payload interface{}) {
	if !tm.enabled {
		return
	}
	tm.m.Publish(topic, qos, retained, payload)
}

func (tm *transportMqtt) SendState(payload []byte) bool {
	if !tm.enabled {
		return false
	}
	tm.log.Infof("transport sendstate payload=%x", payload)
	tm.publish2Telemetry(tm.topicState, 1, false, payload)
	return true
}

func (tm *transportMqtt) SendTelemetry(payload []byte) bool {
	tm.publish2Telemetry(tm.topicTelemetry, 1, false, payload)
	return true
}

func (tm *transportMqtt) SendCommandResponse(topicSuffix string, payload []byte) bool {
	topic := fmt.Sprintf("%s/%s", tm.topicPrefix, topicSuffix)
	tm.log.Infof("mqtt publish command response to topic=%s", topic)
	tm.publish2Telemetry(topic, 1, false, payload)
	return true
}
func (tm *transportMqtt) SendFromRobot(payload []byte) {
	// tm.log.Infof("mqtt publish message from robot to topic=%s", tm.topicRoboOut)
	tm.publish2Telemetry(tm.topicRoboOut, 1, false, payload)

}

func (tm *transportMqtt) messageHandler(c mqtt.Client, msg mqtt.Message) {
	payload := msg.Payload()
	// ALexM rewrite  onCommand = old
	tm.log.Debugf("mqtt income message (%x)", payload)
	if msg.Topic() == tm.topicRoboIn {
		tm.inRobo(payload)
	} else {
		tm.onCommand(payload)
	}
}

func (tm *transportMqtt) connectLostHandler(c mqtt.Client, err error) {
	tm.log.Infof("mqtt disconnect")
}

func (tm *transportMqtt) onConnectHandler(c mqtt.Client) {
	tm.log.Infof("mqtt connect")
	if token := c.Subscribe(tm.topicCommand, 1, nil); token.Wait() && token.Error() != nil {
		tm.log.Infof("mqtt subscribe error")
	} else {
		tm.log.Infof("mqtt subscribe Ok")
		c.Publish(tm.topicConnect, 1, true, []byte{0x01})
	}

	if token1 := c.Subscribe(tm.topicRoboIn, 1, nil); token1.Wait() && token1.Error() != nil {
		tm.log.Infof("mqtt subscribe error")
	}

}
