// Separate package to for hardware/evend related config structure.
// Ugly workaround to import cycles.
package evend_config

type Config struct { //nolint:maligned
	// RU: блок вызачи стаканов.
	// EN: Cup dispensing block.
	Cup CupStruct `hcl:"cup,block"`
	// RU: блок управления подъемником.
	// EN: Elevator control block.
	Elevator ElevatorStruct `hcl:"elevator,block"`
	// RU: блок управления эспрессо.
	// EN: Espresso control block.
	Espresso EspressoStruct `hcl:"espresso,block"`
	// RU: блок управления клапанами.
	// EN: Valve control block.
	Valve ValveStruct `hcl:"valve,block"`
}

type CupStruct struct { //nolint:maligned
	// RU: Время в секундах, через которое стакан считается не выданным и устройство может начать выдавать новый стакан.
	// EN: Time in seconds after which a cup is considered not dispensed and the device can start dispensing a new cup.
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
