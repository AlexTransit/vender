// Место в репозитории: internal/engine/alias_test.go (package engine)
//
// Тесты воспроизводят логику g.initEngine() (internal/state/global.go) без
// пакета state/config_global/HCL: строим Doer через e.ParseText + AddErrorAction
// точно так же, как это делает initEngine для каждого Alias.
package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/log2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestEngine готовит *Engine и context с engine.ContextKey, достаточные
// для Seq.Do/RepeatN.Do/RestartError.Do (они дергают только engine.GetGlobal,
// пакет state не нужен).
func newTestEngine(t *testing.T) (*Engine, context.Context) {
	t.Helper()
	log := log2.NewTest(t, log2.LOG_DEBUG)
	e := NewEngine(log)
	ctx := context.WithValue(context.Background(), ContextKey, e)
	return e, ctx
}

type onErrorSpec struct {
	Scenario string
	SkipMain bool
}

// registerAlias — точное повторение тела цикла из initEngine() для одного
// alias, без обвязки config_global/HCL.
func registerAlias(t *testing.T, e *Engine, name, scenario string, onError map[string]onErrorSpec) Doer {
	t.Helper()
	d, err := e.ParseText(name, scenario)
	require.NoError(t, err, "parse scenario of %s", name)
	for code, spec := range onError {
		fixName := fmt.Sprintf("%s-Err:%s", name, code)
		fixDoer, err := e.ParseText(fixName, spec.Scenario)
		require.NoError(t, err, "parse onError[%s] of %s", code, name)
		d.AddErrorAction(code, fixDoer, spec.SkipMain)
	}
	e.Register(name, d)
	return d
}

// --- alias "mu": scenario="mp(0)"; onError "\d+" { scenario="error(...)" } ---
// (skip не указан в конфиге => SkipMain=false: фикс-сценарий повторяет
// главный сценарий целиком, включая саму упавшую команду).

func registerMu(t *testing.T, e *Engine) {
	registerAlias(t, e, "mu", "mp(0)", map[string]onErrorSpec{
		`\d+`: {Scenario: "error(миксер_недоехал_вверх)"},
	})
}

func TestAlias_Mu_Success(t *testing.T) {
	t.Parallel()
	e, ctx := newTestEngine(t)

	var mpCalls, errCalls int
	e.RegisterNewFuncAgr("mp(?)", func(context.Context, Arg) error { mpCalls++; return nil })
	e.RegisterNewFuncAgr("error(?)", func(context.Context, Arg) error { errCalls++; return nil })
	registerMu(t, e)

	e.TestDo(t, ctx, "mu")

	assert.Equal(t, 1, mpCalls, "mp(0) должен выполниться один раз без ошибок")
	assert.Equal(t, 0, errCalls, "фикс-действие не должно вызываться при успехе")
}

// mp(0) один раз падает с кодом 36, при повторной попытке (внутри фикс-сценария
// самого mu) — успех. Демонстрирует: SkipMain=false => фикс = [error(...), mp(0)].
func TestAlias_Mu_TransientErrorRecovers(t *testing.T) {
	t.Parallel()
	e, ctx := newTestEngine(t)

	mpCalls := 0
	e.RegisterNewFuncAgr("mp(?)", func(context.Context, Arg) error {
		mpCalls++
		if mpCalls == 1 {
			return &helpers.AppError{ErrorCode: 36, Err: errors.New("миксер не доехал")}
		}
		return nil
	})
	errCalls := 0
	e.RegisterNewFuncAgr("error(?)", func(context.Context, Arg) error { errCalls++; return nil })
	registerMu(t, e)

	e.TestDo(t, ctx, "mu")

	assert.Equal(t, 2, mpCalls, "первая попытка + одна повторная внутри фикса")
	assert.Equal(t, 1, errCalls, "фикс-действие вызывается ровно один раз")
}

// mp(0) падает постоянно: mu ловит \d+ один раз, ретраит саму себя один раз,
// не помогает — mu возвращает ИСХОДНУЮ ошибку (без бесконечного цикла).
func TestAlias_Mu_PermanentErrorPropagates(t *testing.T) {
	t.Parallel()
	e, ctx := newTestEngine(t)

	mpCalls := 0
	e.RegisterNewFuncAgr("mp(?)", func(context.Context, Arg) error {
		mpCalls++
		return &helpers.AppError{ErrorCode: 36, Err: errors.New("миксер не доехал")}
	})
	errCalls := 0
	e.RegisterNewFuncAgr("error(?)", func(context.Context, Arg) error { errCalls++; return nil })
	registerMu(t, e)

	d := e.Resolve("mu")
	require.NotNil(t, d)
	err := e.Exec(ctx, d)

	require.Error(t, err)
	var appErr *helpers.AppError
	require.True(t, errors.As(err, &appErr), "error should be *helpers.AppError, got %T: %v", err, err)
	assert.EqualValues(t, 36, appErr.Code())
	assert.Equal(t, 2, mpCalls, "исходная попытка + одна повторная внутри фикса, без зацикливания")
	assert.Equal(t, 1, errCalls)
}

// --- alias "mixer_cleaning": scenario="shaker_clean_position sleep(1s) mu";
// onError "36" { scenario="error.log(fix_cp0_mu) cp(0) mu"; skip=true } ---
//
// Ключевой момент вложенности: ошибка код=36 из mp(0) сначала пытается
// разрешиться ВНУТРЕННИМ фиксом самого mu (\d+, SkipMain=false). Только если
// и повторная попытка mu тоже не удалась, необработанная ошибка всплывает
// до mixer_cleaning, и уже ТАМ срабатывает её собственный фикс на код "36"
// (SkipMain=true => не повторяет shaker_clean_position/sleep, выполняет
// только "error.log(fix_cp0_mu) cp(0) mu").
func TestAlias_MixerCleaning_NestedErrorFix(t *testing.T) {
	t.Parallel()
	e, ctx := newTestEngine(t)

	mpCalls := 0
	e.RegisterNewFuncAgr("mp(?)", func(context.Context, Arg) error {
		mpCalls++
		if mpCalls <= 2 { // 1: в mixer_cleaning->mu, 2: внутренний ретрай mu
			return &helpers.AppError{ErrorCode: 36, Err: errors.New("миксер не доехал")}
		}
		return nil // 3-й вызов — уже после cp(0) в фиксе mixer_cleaning — успех
	})
	errCalls, errLogCalls, cpCalls, shakerCalls := 0, 0, 0, 0
	e.RegisterNewFuncAgr("error(?)", func(context.Context, Arg) error { errCalls++; return nil })
	e.RegisterNewFuncAgr("error.log(?)", func(context.Context, Arg) error { errLogCalls++; return nil })
	e.RegisterNewFuncAgr("cp(?)", func(context.Context, Arg) error { cpCalls++; return nil })
	e.RegisterNewFunc("shaker_clean_position", func(context.Context) error { shakerCalls++; return nil })

	registerMu(t, e)
	registerAlias(t, e, "mixer_cleaning", "shaker_clean_position sleep(1s) mu", map[string]onErrorSpec{
		"36": {Scenario: "error.log(fix_cp0_mu) cp(0) mu", SkipMain: true},
	})

	e.TestDo(t, ctx, "mixer_cleaning")

	assert.Equal(t, 1, shakerCalls, "shaker_clean_position не должен повторяться (SkipMain=true)")
	assert.Equal(t, 1, cpCalls, "cp(0) из фикса mixer_cleaning вызывается один раз")
	assert.Equal(t, 1, errCalls, "внутренний фикс mu отработал один раз (и не помог)")
	assert.Equal(t, 1, errLogCalls, "внешний фикс mixer_cleaning отработал один раз")
	assert.Equal(t, 3, mpCalls, "1) mu в основном сценарии 2) ретрай внутри mu 3) mu внутри фикса mixer_cleaning")
}
