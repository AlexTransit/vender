package menu_config

import (
	"fmt"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/engine"
	tele_api "github.com/AlexTransit/vender/tele"
)

type XXX_MenuStruct struct {
	XXX_Items []MenuItem `hcl:"item,block"`
}
type MenuStruct struct {
	Items           map[string]MenuItem
	DefaultCream    uint8 `hcl:"default_cream,optional"`
	DefaultCreamMax uint8 `hcl:"default_cream_max,optional"`
	DefaultSugar    uint8 `hcl:"default_sugar,optional"`
	DefaultSugarMax uint8 `hcl:"default_sugar_max,optional"`
}

type MenuItem struct {
	Code      string `hcl:"code,label"`
	Name      string `hcl:"name,optional"`
	Scenario  string `hcl:"scenario"`
	Doer      engine.Doer
	XXX_Price int `hcl:"price"` // use scaled `Price`, this is for decoding config only
	Price     currency.Amount
	Disabled  bool  `hcl:"disabled,optional"`
	CreamMax  uint8 `hcl:"creamMax,optional"`
	SugarMax  uint8 `hcl:"sugarMax,optional"`
}

type UIMenuStruct struct {
	SelectedItem  MenuItem
	PaymenId      int64
	PaymentMethod tele_api.PaymentMethod
	PaymentType   tele_api.OwnerType
	QRPayAmount   uint32
	Cream         uint8
	Sugar         uint8
}

func (m *MenuItem) String() string { return fmt.Sprintf("menu.%s %s", m.Code, m.Code) }
