// Separate package to for hardware/mdb related config structure.
// Ugly workaround to import cycles.
package mdb_config

type Config struct { //nolint:maligned
	Bill BillStruct `hcl:"bill,block"`

	Coin       CoinStruct `hcl:"coin,block"`
	LogDebug   bool       `hcl:"log_debug,optional"`
	UartDevice string     `hcl:"uart_device,optional"`
	UartDriver string     `hcl:"uart_driver"` // file|mega|iodin
}

type BillStruct struct {
	ScalingFactor int `hcl:"scaling_factor"`
}

type CoinStruct struct {
	DispenseStrategy int `hcl:"dispense_strategy,optional"`
}
