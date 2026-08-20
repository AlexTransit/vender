package ui_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/money"
	state_new "github.com/AlexTransit/vender/internal/state/new"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/stretchr/testify/require"
)

// TestCashPayAndCook — полный путь: внесение наличных → выбор напитка → приготовление.
// Цены заданы в рублях: HCL "price = 7" после ScaleI(scale=100) даёт 700 —
// столько же, сколько инжектируется монетой (в тех же raw-единицах).
func TestCashPayAndCook(t *testing.T) {
	ctx, g := state_new.NewTestContext(t, "", testConfigWith(`
inventory {}
engine {
	profile {}
	menu {
		item "1" {
			name     = "coffee"
			price    = 7
			scenario = ""
		}
	}
}
`))
	moneysys := new(money.MoneySystem)
	require.NoError(t, moneysys.Start(ctx))

	env := &tenv{ctx: ctx, g: g, uiState: make(chan types.UiState, 4)}
	uiTestSetup(t, env, types.StateFrontBegin, types.StateFrontEnd)
	g.Inventory.FillAll(1000)
	go env.ui.Loop(ctx)

	env.requireState(t, types.StateFrontSelect)

	require.NoError(t, moneysys.XXX_InjectCoin(ctx, 700))
	env.requireDisplay(t, fmt.Sprintf("%s7", g.Config.UI_config.Front.MsgCredit), " ")

	env.emit(env._Key('1'))
	env.requireDisplay(
		t,
		fmt.Sprintf("%s7", g.Config.UI_config.Front.MsgCredit),
		fmt.Sprintf(g.Config.UI_config.Front.MsgInputCode, "1"),
	)

	env.emit(env._Key(byte(_KeyAccept)))
	env.requireState(t, types.StateFrontAccept)

	env.requireState(t, types.StateFrontEnd)
	env.g.Alive.Wait()
}

// TestCashInsufficientCredit — недостаточно денег → сообщение → добавить → успех.
func TestCashInsufficientCredit(t *testing.T) {
	ctx, g := state_new.NewTestContext(t, "", testConfigWith(`
inventory {}
engine {
	profile {}
	menu {
		item "2" {
			creamMax = 4
			sugarMax = 4
			name     = "latte"
			price    = 10
			scenario = "money.commit"
		}
	}
}
`))
	moneysys := new(money.MoneySystem)
	require.NoError(t, moneysys.Start(ctx))

	env := &tenv{ctx: ctx, g: g, uiState: make(chan types.UiState, 4)}
	uiTestSetup(t, env, types.StateFrontBegin, types.StateFrontEnd)
	g.Inventory.FillAll(1000)
	go env.ui.Loop(ctx)

	env.requireState(t, types.StateFrontSelect)

	// Вносим 5 рублей — меньше цены (10 рублей)
	require.NoError(t, moneysys.XXX_InjectCoin(ctx, 500))
	env.requireDisplay(t, fmt.Sprintf("%s5", g.Config.UI_config.Front.MsgCredit), " ")

	env.emit(env._Key('2'))
	env.requireDisplay(
		t,
		fmt.Sprintf("%s5", g.Config.UI_config.Front.MsgCredit),
		fmt.Sprintf(g.Config.UI_config.Front.MsgInputCode, "2"),
	)
	env.emit(env._Key(byte(_KeyAccept)))

	// Недостаточно — продакшен-код выводит на второй строке код+цену
	env.requireDisplay(
		t,
		g.Config.UI_config.Front.MsgMenuInsufficientCreditL1,
		fmt.Sprintf(g.Config.UI_config.Front.MsgInputCode+" "+g.Config.UI_config.Front.MsgPrice, "2", "10"),
	)

	// Добавляем ещё 5 рублей
	require.NoError(t, moneysys.XXX_InjectCoin(ctx, 500))
	env.requireState(t, types.StateFrontAccept)
	env.requireState(t, types.StateFrontEnd)

	env.g.Alive.Wait()
}

// TestCashAbortBeforeCook — внесение денег и отмена (возврат).
func TestCashAbortBeforeCook(t *testing.T) {
	ctx, g := state_new.NewTestContext(t, "", testConfigWith(`
inventory {}
engine {
	profile {}
	menu {
		item "1" {
			price    = 7
			scenario = ""
		}
	}
}
`))
	moneysys := new(money.MoneySystem)
	require.NoError(t, moneysys.Start(ctx))

	env := &tenv{ctx: ctx, g: g, uiState: make(chan types.UiState, 4)}
	uiTestSetup(t, env, types.StateFrontBegin, types.StateFrontEnd)
	go env.ui.Loop(ctx)

	env.requireState(t, types.StateFrontSelect)

	require.NoError(t, moneysys.XXX_InjectCoin(ctx, 700))
	env.requireDisplay(t, fmt.Sprintf("%s7", g.Config.UI_config.Front.MsgCredit), " ")

	// money-abort должен идти с Source=MoneySourceTag, иначе IsMoneyAbort его
	// не распознает и событие уйдёт в IsReject (backspace) — см. emitMoneyAbort
	env.emitMoneyAbort()
	env.requireState(t, types.StateFrontEnd)

	env.g.Alive.Wait()
}

func TestDrinkOrderPayAndCook(t *testing.T) {
	ctx, g := state_new.NewTestContext(t, "", testConfigWith(`
inventory {
	stock "1" {
		code         = 1
		ingredient   = "sugar"
		register_add = "ignore(?)"
	}
	stock "2" {
		code         = 2
		ingredient   = "amaretto"
		register_add = "ignore(?)"
	}
}
engine {
	profile {}
	menu {
		item "2" {
			name     = "tea"
			price    = 5
			creamMax = 4
			sugarMax = 4
			scenario = "add.sugar(10) add.amaretto(10)"
		}
	}
}
`))
	moneysys := new(money.MoneySystem)
	require.NoError(t, moneysys.Start(ctx))

	env := &tenv{ctx: ctx, g: g, uiState: make(chan types.UiState, 4)}
	uiTestSetup(t, env, types.StateFrontBegin, types.StateFrontEnd)
	g.Inventory.FillAll(1000)
	go env.ui.Loop(ctx)

	env.requireState(t, types.StateFrontSelect)

	require.NoError(t, moneysys.XXX_InjectCoin(ctx, 500))
	env.requireDisplay(t, fmt.Sprintf("%s5", g.Config.UI_config.Front.MsgCredit), " ")

	env.emit(env._Key('2'))
	env.requireDisplay(
		t,
		fmt.Sprintf("%s5", g.Config.UI_config.Front.MsgCredit),
		fmt.Sprintf(g.Config.UI_config.Front.MsgInputCode, "2"),
	)

	env.emit(env._Key(byte(_KeyAccept)))
	env.requireState(t, types.StateFrontAccept)
	env.requireState(t, types.StateFrontEnd)

	env.g.Alive.Wait()
}

// TestDrinkOrderPayDispenceAndCook — выбор напитка → внесение наличных →
// выдача сдачи → приготовление.
func TestDrinkOrderPayDispenceAndCook(t *testing.T) {
	ctx, g := state_new.NewTestContext(t, "", testConfigWith(`
inventory {}
engine {
	profile {}
	menu {
		item "1" {
			name     = "coffee"
			price    = 7
			creamMax = 4
			sugarMax = 4
			scenario = "money.commit"
		}
	}
}
`))
	moneysys := new(money.MoneySystem)
	require.NoError(t, moneysys.Start(ctx))

	env := &tenv{ctx: ctx, g: g, uiState: make(chan types.UiState, 4)}
	uiTestSetup(t, env, types.StateFrontBegin, types.StateFrontEnd)
	g.Inventory.FillAll(1000)
	go env.ui.Loop(ctx)

	// Выбор заказа
	env.requireState(t, types.StateFrontSelect)
	env.emit(env._Key('1'))
	env.requireDisplay(
		t,
		fmt.Sprintf("%s0", g.Config.UI_config.Front.MsgCredit),
		fmt.Sprintf(g.Config.UI_config.Front.MsgInputCode, "1"),
	)

	// Внесение наличных больше стоимости напитка
	require.NoError(t, moneysys.XXX_InjectCoin(ctx, 1000))
	env.requireDisplay(
		t,
		fmt.Sprintf("%s10", g.Config.UI_config.Front.MsgCredit),
		fmt.Sprintf(g.Config.UI_config.Front.MsgInputCode, "1"),
	)
	credit := moneysys.GetCredit()
	require.Equal(t, currency.Amount(1000), credit)

	// Выдача сдачи
	env.emit(env._Key(byte(_KeyAccept)))
	env.requireState(t, types.StateFrontAccept)
	require.Eventually(t, func() bool {
		return moneysys.GetCredit() == 0
	}, time.Second, 10*time.Millisecond)

	// Проверка приготовления заказа
	env.requireState(t, types.StateFrontEnd)
	require.Equal(t, currency.Amount(0), moneysys.GetDirty())
	env.g.Alive.Wait()
}

// TestDrinkOrderCookErrorReturnsBroken — выбор напитка → внесение наличных
// (больше цены, со сдачей) → приготовление падает с ошибкой → попытка
// вернуть стоимость напитка → переход в StateBroken.
//
// Сценарий напитка — "test.cook_fail!", специально зарегистрированное в
// этом тесте действие. Обычный engine.Func без своего V (валидатора) всегда
// проходит Validate() (это важно — иначе автомат откажет ещё на выборе
// напитка, до всякой оплаты, как это было бы с "not.valid" или с именем
// несуществующего действия), но гарантированно возвращает ошибку из Do() —
// то есть падает именно во время приготовления.
func TestDrinkOrderCookErrorReturnsBroken(t *testing.T) {
	ctx, g := state_new.NewTestContext(t, "", testConfigWith(`
inventory {}
engine {
	profile {}
	menu {
		item "1" {
			name     = "coffee"
			price    = 7
			creamMax = 4
			sugarMax = 4
			scenario = "test.cook_fail!"
		}
	}
}
`))
	g.Engine.RegisterNewFunc("test.cook_fail!", func(context.Context) error {
		return errors.New("simulated cook failure")
	})

	moneysys := new(money.MoneySystem)
	require.NoError(t, moneysys.Start(ctx))

	env := &tenv{ctx: ctx, g: g, uiState: make(chan types.UiState, 4)}
	// StateBroken — это блокирующий цикл ожидания EventService, сам он
	// никуда не переходит (см. state-machine.go) — останавливаем Alive
	// именно на нём, а не на StateFrontEnd, как в остальных тестах.
	uiTestSetup(t, env, types.StateFrontBegin, types.StateBroken)
	go env.ui.Loop(ctx)

	// Выбор напитка
	env.requireState(t, types.StateFrontSelect)
	env.emit(env._Key('1'))
	env.requireDisplay(
		t,
		fmt.Sprintf("%s0", g.Config.UI_config.Front.MsgCredit),
		fmt.Sprintf(g.Config.UI_config.Front.MsgInputCode, "1"),
	)

	// Вносим больше стоимости напитка — должна получиться сдача
	require.NoError(t, moneysys.XXX_InjectCoin(ctx, 1000))
	env.requireDisplay(
		t,
		fmt.Sprintf("%s10", g.Config.UI_config.Front.MsgCredit),
		fmt.Sprintf(g.Config.UI_config.Front.MsgInputCode, "1"),
	)
	require.Equal(t, currency.Amount(1000), moneysys.GetCredit())

	// Подтверждаем — WithdrawPrepare спишет цену (700), остаток (300) уйдёт
	// в сдачу (в тесте без реального CoinValidator она просто не выдастся
	// физически, но на кредите это уже не сказывается — GetCredit() всё
	// равно обнуляется синхронно внутри WithdrawPrepare).
	env.emit(env._Key(byte(_KeyAccept)))
	env.requireState(t, types.StateFrontAccept)
	require.Eventually(t, func() bool {
		return moneysys.GetCredit() == 0
	}, time.Second, 10*time.Millisecond)

	// Приготовление падает -> onFrontAccept вызывает moneysys.ReturnDirty()
	// (попытка вернуть стоимость напитка) и переводит автомат в Broken.
	env.requireState(t, types.StateBroken)

	// ReturnDirty() возвращает деньги только если они реально физически
	// выданы через CoinValidator. В этом тесте валидатора нет (офлайн-
	// окружение теста), поэтому ReturnDirty() отказывает с
	// ErrCoinAcceptorOffline и НЕ обнуляет dirty — сумма остаётся
	// "числящейся" за автоматом до момента, когда её реально смогут
	// вернуть. onFrontAccept при этом не проверяет ошибку ReturnDirty(),
	// так что сам факт неудачи возврата никуда не долетает — это отдельный
	// вопрос (стоит ли там логировать/эскалировать), тут не трогаем.
	require.Equal(t, currency.Amount(700), moneysys.GetDirty())

	env.g.Alive.Wait()
}
