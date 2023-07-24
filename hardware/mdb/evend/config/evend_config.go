// Separate package to for hardware/evend related config structure.
// Ugly workaround to import cycles.
package evend_config

type Config struct { //nolint:maligned
	Cup struct { //nolint:maligned
		TimeoutSec int `hcl:"timeout_sec"`
	} `hcl:"cup"`
	Elevator struct { //nolint:maligned
		KeepaliveMs    int `hcl:"keepalive_ms"`
		MoveTimeoutSec int `hcl:"move_timeout_sec"`
	} `hcl:"elevator"`
	Espresso struct { //nolint:maligned
		TimeoutSec int `hcl:"timeout_sec"`
	} `hcl:"espresso"`
	Hopper struct { //nolint:maligned
		RunTimeoutMs int `hcl:"run_timeout_ms"`
	} `hcl:"hopper"`
	Mixer struct { //nolint:maligned
		KeepaliveMs    int `hcl:"keepalive_ms"`
		MoveTimeoutSec int `hcl:"move_timeout_sec"`
		ShakeTimeoutMs int `hcl:"shake_timeout_ms"`
	} `hcl:"mixer"`
	Valve struct { //nolint:maligned
		// TODO TemperatureCold int     `hcl:"temperature_cold"`
		TemperatureHot     int `hcl:"temperature_hot"`
		TemperatureValidMs int `hcl:"temperature_valid_ms"`
		PourTimeoutSec     int `hcl:"pour_timeout_sec"`
		CautionPartMl      int `hcl:"caution_part_ml"`
	} `hcl:"valve"`
}
