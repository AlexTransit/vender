package engine_config

import (
	"fmt"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/engine"
)

type Config struct {
	Aliases        []Alias  `hcl:"alias"`
	OnBoot         []string `hcl:"on_boot"`
	FirstInit      []string `hcl:"first_init"`
	OnMenuError    []string `hcl:"on_menu_error"`
	OnServiceBegin []string `hcl:"on_service_begin"`
	OnServiceEnd   []string `hcl:"on_service_end"`
	OnFrontBegin   []string `hcl:"on_front_begin"`
	OnBroken       []string `hcl:"on_broken"`
	Inventory      Inventory
	Menu           struct {
		Items []*MenuItem `hcl:"item"`
	}
	Profile struct {
		Regexp    string `hcl:"regexp"`
		MinUs     int    `hcl:"min_us"`
		LogFormat string `hcl:"log_format"`
	}
}

type Alias struct {
	Name     string `hcl:"name,key"`
	Scenario string `hcl:"scenario"`

	Doer engine.Doer `hcl:"-"`
}

type MenuItem struct {
	Disabled  bool   `hcl:"disabled"`
	Code      string `hcl:"code,key"`
	Name      string `hcl:"name"`
	XXX_Price int    `hcl:"price"` // use scaled `Price`, this is for decoding config only
	Scenario  string `hcl:"scenario"`
	CreamMax  int    `hcl:"creamMax"`
	SugarMax  int    `hcl:"sugarMax"`

	Price currency.Amount `hcl:"-"`
	Doer  engine.Doer     `hcl:"-"`
}

func (mi *MenuItem) String() string { return fmt.Sprintf("menu.%s %s", mi.Code, mi.Name) }

type Inventory struct { //nolint:maligned
	Persist     bool    `hcl:"persist"`
	Stocks      []Stock `hcl:"stock"`
	TeleAddName bool    `hcl:"tele_add_name"` // send stock names to telemetry; false to save network usage
}

type Stock struct { //nolint:maligned
	Name        string  `hcl:"name,key"`
	Code        int     `hcl:"code"`
	Check       bool    `hcl:"check"`
	Min         float32 `hcl:"min"`
	SpendRate   float32 `hcl:"spend_rate"`
	RegisterAdd string  `hcl:"register_add"`
	Level       string  `hcl:"level"`
	TuneKey     string
}

func (s *Stock) String() string {
	return fmt.Sprintf("inventory.%s #%d check=%t spend_rate=%f min=%f",
		s.Name, s.Code, s.Check, s.SpendRate, s.Min)
}
