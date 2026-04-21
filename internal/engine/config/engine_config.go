package engine_config

import (
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/menu/menu_config"
)

type Config struct {
	// RU: список псевдонимов. псевдоним соджержит имя сценарий и может содержать сценарии действий при ошибках. при вызове псевдонима выполняется сценарий действий. если при выполнении возникает ошибка, то выполняется сценарий действий для этой ошибки. если сценария для этой ошибки нет, то выполнение псевдонима прерывается и ошибка возвращается в вызывающий код. сценарии действий если произошла ошибка. Ключ - регулярное выражение кода ошибки ( '\' надо экранировать ). на каждую ошибку свой сценарий. не допускается пересечение регулярных выражений для разных ошибок.
	// Example: onError "3[78]" { scenario = "error_scenario" } - при ошибках с кодами 37,38 будет выполняться сценарий "error_scenario". onError "\\d+" { scenario = "error_scenario" } - для любой ошибки.
	XXX_Aliases []Alias `hcl:"alias,block"`
	Aliases     map[string]Alias
	// RU: список действий при загрузке системы.
	// Example: on_boot = ["text_boot sleep(2s)", "evend.cup.ensure", "evend.valve.set_temp_hot_config " ]
	OnBoot []string `hcl:"on_boot,optional"`
	// RU: список действий при первоначальной загрузке системы. проверяет папку Watchdog. если папки нет, то выполняет эти действия. после выполнения создает папку. если папка есть, то эти действия не выполняются.
	// Example: first_init = ["text_first_init evend.cup.ensure", "evend.valve.set_temp_hot_config " ]
	FirstInit []string `hcl:"first_init,optional"`
	// Deprecated:
	OnMenuError []string `hcl:"on_menu_error,optional"`
	// RU: список действий при входе в сервисное меню.
	// Example: on_service_begin = [ " evend.valve.set_temp_hot(0) ", " evend.cup.light_off " ]
	OnServiceBegin []string `hcl:"on_service_begin,optional"`
	// RU: список действий при выходе из сервисного меню.
	// Example:	on_service_end = [ " evend.valve.set_temp_hot_config " ]
	OnServiceEnd []string `hcl:"on_service_end,optional"`
	// RU: список действий при начале ожидания действий от клиента. выполняется поле окончания приготовления и после timeout UI.
	// Example: on_front_begin = ["text_reklama picture(/home/vmc/pic-coffe) evend.valve.set_temp_hot_config evend.cup.light_on_schedule ", ]
	OnFrontBegin []string `hcl:"on_front_begin,optional"`
	// RU: список действий если произошла ошибка.
	// Example: on_broken = ["text_broken picture(/home/vmc/pic-broken) picture(/home/vmc/pic-broken) money.abort evend.cup.light_off play(broken.mp3) " ]
	OnBroken []string `hcl:"on_broken,optional"`
	// RU: список при выходе из программы.
	// Example: on_shutdown = [ "text_poweroff picture(/home/vmc/pic-broken) money.abort evend.cup.light_off evend.valve.set_temp_hot(0) " ]
	OnShutdown []string      `hcl:"on_shutdown,optional"`
	Profile    ProfileStruct `hcl:"profile,block"`
	// RU: список меню.
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

	XXX_OnError []XXXErrorAction `hcl:"onError,block"`
	OnError     map[string]ErrorAction
	Doer        engine.Doer
}
type XXXErrorAction struct {
	ErrCode  string `hcl:",label"`
	Scenario string `hcl:"scenario"`
}

type ErrorAction struct {
	Scenario string
	Doer     engine.Doer
}
