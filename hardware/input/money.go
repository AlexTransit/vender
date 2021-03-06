package input

import "github.com/AlexTransit/vender/internal/types"

const MoneySourceTag = "money"

const (
	MoneyKeyAbort types.InputKey = 27
)

func IsMoneyAbort(e *types.InputEvent) bool {
	return e.Source == MoneySourceTag && e.Key == MoneyKeyAbort
}
