// Separate package to for hardware/mdb related config structure.
// Ugly workaround to import cycles.
package mdb_config

type Config struct { //nolint:maligned
	// RU: Конфигурация для купюроприемника.
	Bill BillStruct `hcl:"bill,block"`
	// RU: Конфигурация для монетоприемника.
	Coin       CoinStruct `hcl:"coin,block"`
	LogDebug   bool       `hcl:"log_debug,optional"`
	UartDevice string     `hcl:"uart_device,optional"`
	UartDriver string     `hcl:"uart_driver"` // file|mega|iodin|dummy
}

type BillStruct struct {
	ScalingFactor int `hcl:"scaling_factor"`
}

type CoinStruct struct {
	// RU: Стратегия выдачи сдачи. 0 = равномерная выдача (стараемся держать одинаковое количество монет в каждой тубе), 1 = сначала полная трубка (если туба полная то выдаем из нее. далее выдаем минимальным количеством монет), 2 = минимальное количество монет (выдаем минимальным количеством монет).
	DispenseStrategy int `hcl:"dispense_strategy,optional"` // 0 = uniform dispensing, 1 = first ful tube, 2 = minimal coins
}
