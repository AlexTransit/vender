package menu_config

import (
	"fmt"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/engine"
)

type XXX_MenuStruct struct {
	XXX_Items []MenuItem `hcl:"item,block"`
}
type MenuStruct struct {
	DefaultCream    uint8 `hcl:"default_cream,optional"`
	DefaultCreamMax uint8 `hcl:"default_cream_max,optional"`
	DefaultSugar    uint8 `hcl:"default_sugar,optional"`
	DefaultSugarMax uint8 `hcl:"default_sugar_max,optional"`
	Items           map[string]MenuItem
}

type MenuItem struct {
	Code      string `hcl:"code,label"`
	Disabled  bool   `hcl:"disabled,optional"`
	Name      string `hcl:"name,optional"`
	XXX_Price int    `hcl:"price"` // use scaled `Price`, this is for decoding config only
	Scenario  string `hcl:"scenario"`
	CreamMax  uint8  `hcl:"creamMax,optional"`
	SugarMax  uint8  `hcl:"sugarMax,optional"`

	Price currency.Amount
	Doer  engine.Doer
}

type UIMenuStruct struct {
	SelectedItem MenuItem
	Cream        uint8
	Sugar        uint8
	PaymenId     int64
	QRPayAmount  uint32
}

func (m *MenuItem) String() string { return fmt.Sprintf("menu.%s %s", m.Code, m.Code) }
