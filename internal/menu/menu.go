package menu_vmc

import (
	"context"
	"fmt"

	"github.com/AlexTransit/vender/currency"
	config_global "github.com/AlexTransit/vender/internal/config"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/watchdog"
)

func MenuMaxPrice() (currency.Amount, error) {
	max := currency.Amount(0)
	empty := true
	for _, item := range config_global.VMC.Engine.Menu.Items {
		valErr := item.Doer.Validate()
		if valErr == nil {
			empty = false
			if item.Price > max {
				max = item.Price
			}
		}
	}
	if empty {
		return 0, fmt.Errorf("menu len=%d no valid items", len(config_global.VMC.Engine.Menu.Items))
	}
	return max, nil
}

func Cook(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	watchdog.Refresh()
	itemCtx := money.SetCurrentPrice(ctx, config_global.VMC.User.SelectedItem.Price)
	if tuneCream := ScaleTuneRate(&config_global.VMC.User.Cream, config_global.CreamMax(), config_global.DefaultCream()); tuneCream != 1 {
		const name = "cream"
		var err error
		g.Log.Debugf("ui-front tuning stock=%s tune=%v", name, tuneCream)
		if itemCtx, err = g.Inventory.WithTuning(itemCtx, "cream", tuneCream); err != nil {
			g.Log.Errorf("ui-front tuning stock=%s err=%v", name, err)
		}
	}
	if tuneSugar := ScaleTuneRate(&config_global.VMC.User.Sugar, config_global.SugarMax(), config_global.DefaultSugar()); tuneSugar != 1 {
		const name = "sugar"
		var err error
		g.Log.Debugf("ui-front tuning stock=%s tune=%v", name, tuneSugar)
		if itemCtx, err = g.Inventory.WithTuning(itemCtx, "sugar", tuneSugar); err != nil {
			g.Log.Errorf("ui-front tuning stock=%s err=%v", name, err)
		}
	}

	err := g.Engine.Exec(itemCtx, config_global.VMC.User.SelectedItem.Doer)
	if err == nil {
		if g.Tele.RoboConnected() {
			config_global.VMC.Inventory.ReportInv++
			// AlexM autoreporter move to config
			if config_global.VMC.Inventory.ReportInv > 10 {
				config_global.VMC.Inventory.ReportInv = 0
				_ = g.Tele.Report(ctx, false)
			}
		}
	} else {
		g.Log.Err(err)
	}
	if invErr := g.Inventory.InventorySave(); invErr != nil {
		g.Tele.Error(invErr)
	}
	g.Log.Debugf("ui-front selected=%s end err=%v", config_global.VMC.User.SelectedItem.String(), err)
	return err
}

func ScaleTuneRate(value *uint8, max uint8, center uint8) float32 {
	if *value > max {
		*value = max
	}
	switch {
	case *value == center: // most common path
		return 1
	case *value == 0:
		return 0
	}
	if *value > 0 && *value < center {
		return 1 - (0.25 * float32(center-*value))
	}
	if *value > center && *value <= max {
		return 1 + (0.25 * float32(*value-center))
	}
	panic("code error")
}
