package menu_config

import (
	"fmt"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/engine"
)

type MenuStruct struct {
	XXX_Items []MenuItem `hcl:"item,block"`
	Items     map[string]MenuItem
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
