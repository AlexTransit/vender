package engine_config

import (
	"fmt"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/engine"
)

type Config struct {
	XXX_Aliases    []Alias `hcl:"alias,block"`
	Aliases        map[string]Alias
	OnBoot         []string      `hcl:"on_boot,optional"`
	FirstInit      []string      `hcl:"first_init,optional"`
	OnMenuError    []string      `hcl:"on_menu_error,optional"`
	OnServiceBegin []string      `hcl:"on_service_begin,optional"`
	OnServiceEnd   []string      `hcl:"on_service_end,optional"`
	OnFrontBegin   []string      `hcl:"on_front_begin,optional"`
	OnBroken       []string      `hcl:"on_broken,optional"`
	OnShutdown     []string      `hcl:"on_shutdown,optional"`
	Inventory      Inventory     `hcl:"inventory,block"`
	Profile        ProfileStruct `hcl:"profile,block"`
	Menu           MenuStruct    `hcl:"menu,block"`
}

type MenuStruct struct {
	XXX_Items []MenuItem `hcl:"item,block"`
	Items     map[string]MenuItem
}
type ProfileStruct struct {
	Regexp    string `hcl:"regexp,optional"`
	MinUs     int    `hcl:"min_us,optional"`
	LogFormat string `hcl:"log_format,optional"`
}

type Alias struct {
	Name     string `hcl:"name,label"`
	Scenario string `hcl:"scenario"`

	Doer engine.Doer
}

type MI struct {
	Code string `hcl:"codeaa"`
}

type MenuItem struct {
	Code      string `hcl:"code,label"`
	Disabled  bool   `hcl:"disabled,optional"`
	Name      string `hcl:"name,optional"`
	XXX_Price int    `hcl:"price"` // use scaled `Price`, this is for decoding config only
	Scenario  string `hcl:"scenario"`
	CreamMax  int    `hcl:"creamMax,optional"`
	SugarMax  int    `hcl:"sugarMax,optional"`

	Price currency.Amount
	Doer  engine.Doer
}

func (mi *MenuItem) String() string { return fmt.Sprintf("menu.%s %s", mi.Code, mi.Name) }

type Inventory struct { //nolint:maligned
	Persist     bool    `hcl:"persist,optional"`
	TeleAddName bool    `hcl:"tele_add_name,optional"` // send stock names to telemetry; false to save network usage
	XXX_Stocks  []Stock `hcl:"stock,block"`
	Stocks      map[string]Stock
}

type Stock struct { //nolint:maligned
	Name        string  `hcl:",label"`
	Code        int     `hcl:"code"`
	Check       bool    `hcl:"check,optional"`
	Min         float32 `hcl:"min,optional"`
	SpendRate   float32 `hcl:"spend_rate,optional"`
	RegisterAdd string  `hcl:"register_add,optional"`
	Level       string  `hcl:"level,optional"`
	TuneKey     string
	Value       float32
}

func (s *Stock) String() string {
	return fmt.Sprintf("inventory.%s #%d check=%t spend_rate=%f min=%f",
		s.Name, s.Code, s.Check, s.SpendRate, s.Min)
}
