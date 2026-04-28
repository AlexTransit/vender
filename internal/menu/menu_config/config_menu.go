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
	DefaultCream    uint8 `hcl:"default_cream,optional"`
	DefaultCreamMax uint8 `hcl:"default_cream_max,optional"`
	DefaultSugar    uint8 `hcl:"default_sugar,optional"`
	DefaultSugarMax uint8 `hcl:"default_sugar_max,optional"`
	Items           map[string]MenuItem
}

type MenuItem struct {
	// RU: код напитка. должен быть уникальным для каждого напитка.
	Code string `hcl:"code,label"`
	// RU: отключить напиток. если true, то напиток не будет отображаться в меню и не будет доступен для приготовления.
	Disabled bool `hcl:"disabled,optional"`
	// RU: имя напитка.
	Name string `hcl:"name,optional"`
	// RU: цена напитка в копейках. например, 25 рублей это 2500 копеек. если цена 0, то напиток будет бесплатным.
	XXX_Price int `hcl:"price"` // use scaled `Price`, this is for decoding config only
	// RU: сценарий приготовления напитка. может содержать псевдонимы.
	Scenario string `hcl:"scenario"`
	// RU: максимальное количество сливок для этого напитка. если 0, то будет использоваться значение из DefaultCreamMax.
	CreamMax uint8 `hcl:"creamMax,optional"`
	// RU: максимальное количество сахара для этого напитка. если 0, то будет использоваться значение из DefaultSugarMax.
	SugarMax uint8 `hcl:"sugarMax,optional"`

	Price currency.Amount
	Doer  engine.Doer
}

type UIMenuStruct struct {
	SelectedItem  MenuItem
	Cream         uint8
	Sugar         uint8
	PaymenId      int64
	PaymentMethod tele_api.PaymentMethod
	PaymentType   tele_api.OwnerType
	QRPayAmount   uint32
}

func (m *MenuItem) String() string { return fmt.Sprintf("menu.%s %s", m.Code, m.Code) }
