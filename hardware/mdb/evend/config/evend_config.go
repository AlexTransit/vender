// Separate package to for hardware/evend related config structure.
// Ugly workaround to import cycles.
package evend_config

type Config struct { //nolint:maligned
	Cup      CupStruct      `hcl:"cup,block"`
	Elevator ElevatorStruct `hcl:"elevator,block"`
	Espresso EspressoStruct `hcl:"espresso,block"`
	Hopper   HopperStruct   `hcl:"hopper,block"`
	Mixer    MixerStruct    `hcl:"mixer,block"`
	Valve    ValveStruct    `hcl:"valve,block"`
}

type CupStruct struct { //nolint:maligned
	TimeoutSec int `hcl:"timeout_sec"`
}

type ElevatorStruct struct { //nolint:maligned
	KeepaliveMs    int `hcl:"keepalive_ms,optional"`
	MoveTimeoutSec int `hcl:"move_timeout_sec,optional"`
}

type EspressoStruct struct { //nolint:maligned
	TimeoutSec int `hcl:"timeout_sec,optional"`
}
type HopperStruct struct { //nolint:maligned
	RunTimeoutMs int `hcl:"run_timeout_ms,optional"`
}

type MixerStruct struct { //nolint:maligned
	KeepaliveMs    int `hcl:"keepalive_ms"`
	MoveTimeoutSec int `hcl:"move_timeout_sec"`
	ShakeTimeoutMs int `hcl:"shake_timeout_ms"`
}

type ValveStruct struct { //nolint:maligned
	// TODO TemperatureCold int     `hcl:"temperature_cold"`
	TemperatureHot int `hcl:"temperature_hot"`
}
