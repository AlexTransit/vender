// Separate package to for hardware/mdb related config structure.
// Ugly workaround to import cycles.
package mdb_config

type Config struct { //nolint:maligned
	Bill struct {
		ScalingFactor int `hcl:"scaling_factor"`
	}
	Coin struct { //nolint:maligned
		DispenseStrategy int
	}
	LogDebug   bool   `hcl:"log_debug"`
	UartDevice string `hcl:"uart_device"`
	UartDriver string `hcl:"uart_driver"` // file|mega|iodin
}
