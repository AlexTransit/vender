// Telemetry client config, vending machine side.
// Separate package is workaround to import cycles.
package tele_config

type Config struct { //nolint:maligned
	Enabled        bool   `hcl:"enable"`
	VmId           int    `hcl:"vm_id"`
	LogDebug       bool   `hcl:"log_debug"`
	KeepaliveSec   int    `hcl:"keepalive_sec"`
	PingTimeoutSec int    `hcl:"ping_timeout_sec"`
	MqttBroker     string `hcl:"mqtt_broker"`
	MqttLogDebug   bool   `hcl:"mqtt_log_debug"`
	MqttPassword   string `hcl:"mqtt_password"` // secret
	StorePath      string `hcl:"store_path"`

	NetworkRestartTimeout int    `hcl:"network_restart_timeout_sec"`
	NetworkRestartScript  string `hcl:"network_restart_script"`
}
