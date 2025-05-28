// Telemetry client config, vending machine side.
// Separate package is workaround to import cycles.
package tele_config

type Config struct { //nolint:maligned
	MqttBroker            string `hcl:"mqtt_broker,optional"`
	MqttPassword          string `hcl:"mqtt_password,optional"` // secret
	StorePath             string `hcl:"store_path,optional"`
	NetworkRestartScript  string `hcl:"network_restart_script,optional"`
	VmId                  int    `hcl:"vm_id,optional"`
	KeepaliveSec          int    `hcl:"keepalive_sec,optional"`
	PingTimeoutSec        int    `hcl:"ping_timeout_sec,optional"`
	NetworkRestartTimeout int    `hcl:"network_restart_timeout_sec,optional"`
	Enabled               bool   `hcl:"enable,optional"`
	LogDebug              bool   `hcl:"log_debug,optional"`
	MqttLogDebug          bool   `hcl:"mqtt_log_debug,optional"`
}
