package ui

import (
	"context"
	"fmt"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/juju/errors"
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

func FillMenu(ctx context.Context) {
	config := state.GetGlobal(ctx).Config

	for _, x := range config.Engine.Menu.Items {
		types.UI.Menu[x.Code] = types.MenuItemType{
			Name:  x.Name,
			D:     x.Doer,
			Price: x.Price,
			Code:  x.Code,
		}
	}
}
func Init(ctx context.Context) error {
	config := state.GetGlobal(ctx).Config

	for _, x := range config.Engine.Menu.Items {
		types.UI.Menu[x.Code] = types.MenuItemType{
			Name:  x.Name,
			D:     x.Doer,
			Price: x.Price,
			Code:  x.Code,
		}
	}
	return nil
}

func Cook(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	// moneysys := money.GetGlobal(ctx)
	state.VmcLock(ctx)
	defer state.VmcUnLock(ctx)

	itemCtx := money.SetCurrentPrice(ctx, types.UI.FrontResult.Item.Price)
	if tuneCream := ScaleTuneRate(types.UI.FrontResult.Cream, MaxCream, DefaultCream); tuneCream != 1 {
		const name = "cream"
		var err error
		g.Log.Debugf("ui-front tuning stock=%s tune=%v", name, tuneCream)
		if itemCtx, err = g.Inventory.WithTuning(itemCtx, name, tuneCream); err != nil {
			g.Log.Errorf("ui-front tuning stock=%s err=%v", name, err)
		}
	}
	if tuneSugar := ScaleTuneRate(types.UI.FrontResult.Sugar, MaxSugar, DefaultSugar); tuneSugar != 1 {
		const name = "sugar"
		var err error
		g.Log.Debugf("ui-front tuning stock=%s tune=%v", name, tuneSugar)
		if itemCtx, err = g.Inventory.WithTuning(itemCtx, name, tuneSugar); err != nil {
			g.Log.Errorf("ui-front tuning stock=%s err=%v", name, err)
		}
	}
	// AlexM FixMe наверно переделать вывод на текстовый экран
	g.MustTextDisplay().SetLines(g.Config.UI.Front.MsgMaking1, g.Config.UI.Front.MsgMaking2)
	g.ShowPicture(state.PictureMake)

	err := g.Engine.Exec(itemCtx, types.UI.FrontResult.Item.D)
	if invErr := g.Inventory.Persist.Store(); invErr != nil {
		g.Error(errors.Annotate(invErr, "critical inventory persist"))
	}
	g.Log.Debugf("ui-front selected=%s end err=%v", types.UI.FrontResult.Item.String(), err)
	return err

}
