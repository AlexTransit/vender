package ui

import (
	"context"
	"fmt"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
)

type Menu map[string]MenuItem

type MenuItem struct {
	Name  string
	D     engine.Doer
	Price currency.Amount
	Code  string
}

func (mi *MenuItem) String() string {
	return fmt.Sprintf("menu code=%s price=%d(raw) name='%s'", mi.Code, mi.Price, mi.Name)
}

// FIXME alexm. меню переделать. посмотреть что массив нигде не используется и в конфиге сразу заполнить карту.
func FillMenu(ctx context.Context) {
	items := state.GetGlobal(ctx).Config.Engine.Menu.Items
	for _, x := range items {
		ic := types.UI.Menu[x.Code]
		ic.Name = x.Name
		ic.D = x.Doer
		ic.Price = x.Price
		if x.CreamMax != 0 {
			ic.CreamMax = uint8(x.CreamMax)
		}
		if x.SugarMax != 0 {
			ic.SugarMax = uint8(x.SugarMax)
		}
		ic.Code = x.Code
		types.UI.Menu[x.Code] = ic
	}
}

func Cook(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	state.VmcLock(ctx)

	itemCtx := money.SetCurrentPrice(ctx, types.UI.FrontResult.Item.Price)
	if tuneCream := ScaleTuneRate(&types.UI.FrontResult.Cream, CreamMax(), DefaultCream); tuneCream != 1 {
		const name = "cream"
		var err error
		g.Log.Debugf("ui-front tuning stock=%s tune=%v", name, tuneCream)
		if itemCtx, err = g.Inventory.WithTuning(itemCtx, "cream", tuneCream); err != nil {
			g.Log.Errorf("ui-front tuning stock=%s err=%v", name, err)
		}
	}
	if tuneSugar := ScaleTuneRate(&types.UI.FrontResult.Sugar, SugarMax(), DefaultSugar); tuneSugar != 1 {
		const name = "sugar"
		var err error
		g.Log.Debugf("ui-front tuning stock=%s tune=%v", name, tuneSugar)
		if itemCtx, err = g.Inventory.WithTuning(itemCtx, "sugar", tuneSugar); err != nil {
			g.Log.Errorf("ui-front tuning stock=%s err=%v", name, err)
		}
	}

	err := g.Engine.Exec(itemCtx, types.UI.FrontResult.Item.D)
	if err == nil {
		if g.Tele.RoboConnected() {
			types.VMC.ReportInv += 1
			// AlexM autoreporter move to config
			if types.VMC.ReportInv > 10 {
				types.VMC.ReportInv = 0
				_ = g.Tele.Report(ctx, false)
			}
		}
	} else {
		g.Log.Err(err)
	}
	if invErr := g.Inventory.InventorySave(); invErr != nil {
		g.Tele.Error(invErr)
	}
	g.Log.Debugf("ui-front selected=%s end err=%v", types.UI.FrontResult.Item.String(), err)
	return err
}

func CreamMax() uint8 {
	if types.UI.FrontResult.Item.CreamMax == 0 {
		return MaxCream
	}
	return types.UI.FrontResult.Item.CreamMax
}

func SugarMax() uint8 {
	if types.UI.FrontResult.Item.SugarMax == 0 {
		return MaxSugar
	}
	return types.UI.FrontResult.Item.SugarMax
}

func menuMaxPrice() (currency.Amount, error) {
	max := currency.Amount(0)
	empty := true
	for _, item := range types.UI.Menu {
		valErr := item.D.Validate()
		if valErr == nil {
			empty = false
			if item.Price > max {
				max = item.Price
			}
		}
	}
	if empty {
		return 0, fmt.Errorf("menu len=%d no valid items", len(types.UI.Menu))
	}
	return max, nil
}
