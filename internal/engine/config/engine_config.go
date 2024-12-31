package engine_config

import (
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/menu/menu_config"
)

type Config struct {
	XXX_Aliases    []Alias `hcl:"alias,block"`
	Aliases        map[string]Alias
	OnBoot         []string `hcl:"on_boot,optional"`
	FirstInit      []string `hcl:"first_init,optional"`
	OnMenuError    []string `hcl:"on_menu_error,optional"`
	OnServiceBegin []string `hcl:"on_service_begin,optional"`
	OnServiceEnd   []string `hcl:"on_service_end,optional"`
	OnFrontBegin   []string `hcl:"on_front_begin,optional"`
	OnBroken       []string `hcl:"on_broken,optional"`
	OnShutdown     []string `hcl:"on_shutdown,optional"`
	// Inventory      inventory.Inventory        // `hcl:"inventory,block"`
	Profile     ProfileStruct              `hcl:"profile,block"`
	XXX_Menu    menu_config.XXX_MenuStruct `hcl:"menu,block"`
	Menu        menu_config.MenuStruct
	NeedRestart bool
}

type ProfileStruct struct {
	Regexp    string `hcl:"regexp,optional"`
	MinUs     int    `hcl:"min_us,optional"`
	LogFormat string `hcl:"log_format,optional"`
}

type Alias struct {
	Name     string `hcl:"name,label"`
	Scenario string `hcl:"scenario"`
	Doer     engine.Doer
}
