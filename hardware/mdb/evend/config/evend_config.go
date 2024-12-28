// Separate package to for hardware/evend related config structure.
// Ugly workaround to import cycles.
package evend_config

type Config struct { //nolint:maligned
	Cup      CupStruct      `hcl:"cup,block"`
	Elevator ElevatorStruct `hcl:"elevator,block"`
	Espresso EspressoStruct `hcl:"espresso,block"`
	Valve    ValveStruct    `hcl:"valve,block"`
}

type CupStruct struct { //nolint:maligned
	TimeoutSec int `hcl:"timeout_sec"`
}

type ElevatorStruct struct { //nolint:maligned
	MoveTimeoutSec int `hcl:"move_timeout_sec,optional"`
}

type EspressoStruct struct { //nolint:maligned
	TimeoutSec int `hcl:"timeout_sec,optional"`
}

type ValveStruct struct { //nolint:maligned
	TemperatureHot int `hcl:"temperature_hot"`
}
