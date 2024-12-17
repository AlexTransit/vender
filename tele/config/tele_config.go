// Telemetry client config, vending machine side.
// Separate package is workaround to import cycles.
package tele_config

type Config struct { //nolint:maligned
	Enabled        bool   `hcl:"enable,optional"`
	VmId           int    `hcl:"vm_id,optional"`
	LogDebug       bool   `hcl:"log_debug,optional"`
	KeepaliveSec   int    `hcl:"keepalive_sec,optional"`
	PingTimeoutSec int    `hcl:"ping_timeout_sec,optional"`
	MqttBroker     string `hcl:"mqtt_broker,optional"`
	MqttLogDebug   bool   `hcl:"mqtt_log_debug,optional"`
	MqttPassword   string `hcl:"mqtt_password,optional"` // secret
	StorePath      string `hcl:"store_path,optional"`

	NetworkRestartTimeout int    `hcl:"network_restart_timeout_sec,optional"`
	NetworkRestartScript  string `hcl:"network_restart_script,optional"`
}
