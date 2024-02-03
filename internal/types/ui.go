package types

import "context"

type UIer interface {
	Loop(context.Context)
	// FrontSelectShowZero(context.Context)
	GetUiState() uint32
	Scheduler
}

type UiState uint32

const (
	StateDefault UiState = iota

	StateBoot   // 1 t=onstart +onstartOk=FrontHello +onstartError+retry=Boot +retryMax=Broken
	StateBroken // 2 t=tele/input +inputService=ServiceBegin
	StateLocked // 3 t=tele

	StateFrontBegin   // 4 t=checkVariables +=FrontHello
	StateFrontSelect  // 5 t=input/money/timeout +inputService=ServiceBegin +input=... +money=... +inputAccept=FrontAccept +timeout=FrontTimeout
	StatePrepare      // 6
	StateFrontTune    // 7 t=input/money/timeout +inputTune=FrontTune ->FrontSelect
	StateFrontAccept  // 8 t=engine.Exec(Item) +OK=FrontEnd +err=Broken
	StateFrontTimeout // 9 t=saveMoney ->FrontEnd
	StateFrontEnd     // 10 ->FrontBegin

	StateServiceBegin     // 11 t=input/timeout ->ServiceAuth
	StateServiceAuth      // 12 +inputAccept+OK=ServiceMenu
	StateServiceMenu      // 13
	StateServiceInventory // 14
	StateServiceTest
	StateServiceReboot
	StateServiceNetwork
	StateServiceMoneyLoad
	StateServiceReport
	StateServiceEnd // 20 +askReport=ServiceReport ->FrontBegin

	StateStop // 21

	StateFrontLock

	StateOnStart

	StateDoesNotChange
)
