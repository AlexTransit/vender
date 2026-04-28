// Separate package to for hardware/evend related config structure.
// Ugly workaround to import cycles.
package evend_config

type Config struct { //nolint:maligned
	// RU: блок управления эспрессо.
	// EN: Espresso control block.
	Espresso EspressoStruct `hcl:"espresso,block"`
	// RU: блок управления клапанами.
	// EN: Valve control block.
	Valve ValveStruct `hcl:"valve,block"`
}

type EspressoStruct struct { //nolint:maligned
	TimeoutSec int `hcl:"timeout_sec,optional"`
}

type ValveStruct struct { //nolint:maligned
	TemperatureHot int `hcl:"temperature_hot"`
}
