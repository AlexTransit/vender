package menu

import (
	"fmt"

	"github.com/AlexTransit/vender/internal/menu/menu_config"
)

type MI struct {
	Code string `hcl:"codeaa"`
}

type (
	MenuItem menu_config.MenuItem
	Menu     menu_config.MenuStruct
)

func (m *MenuItem) String() string { return fmt.Sprintf("menu.%s %s", m.Code, m.Code) }

func (m *Menu) init() {
}
