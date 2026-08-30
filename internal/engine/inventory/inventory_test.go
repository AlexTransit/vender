package inventory_test

import (
	"context"
	"testing"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	"github.com/AlexTransit/vender/log2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestInventory builds a real Engine + Inventory pair the same way
// production code does (state.Global.initEngine + Inventory.Init),
// without pulling in the full state/UI stack.
func newTestInventory(t *testing.T, ing inventory.Ingredient, stock inventory.Stock) (*inventory.Inventory, *engine.Engine, context.Context) {
	t.Helper()

	log := log2.NewTest(t, log2.LOG_DEBUG)
	e := engine.NewEngine(log)
	ctx := context.WithValue(context.Background(), engine.ContextKey, e)

	inv := &inventory.Inventory{
		File: t.TempDir() + "/inventory",
		XXX_Ingredient: map[string]inventory.Ingredient{
			ing.Name: ing,
		},
		XXX_Stocks: map[string]inventory.Stock{
			stock.Label: stock,
		},
	}
	require.NoError(t, inv.Init(ctx, e, log))
	return inv, e, ctx
}

// TestStockLowBlocksSpend — пустой склад (ниже минимума) должен блокировать отгрузку.
func TestStockLowBlocksSpend(t *testing.T) {
	t.Parallel()

	inv, e, ctx := newTestInventory(t,
		inventory.Ingredient{Name: "cream", Min: 50, SpendRate: 1},
		inventory.Stock{Label: "cream", Code: 1, XXX_Ingredient: "cream", RegisterAdd: "ignore(?)"},
	)

	d := e.Resolve("add.cream(10)")
	require.NotNil(t, d)

	err := d.Validate()
	require.Error(t, err, "expected 'low' validation error on empty stock")
	assert.Contains(t, err.Error(), "cream low")

	// Do() must refuse too, not just Validate()
	err = e.Exec(ctx, d)
	require.Error(t, err)

	s, ok := inv.GetStockByingredientName("cream")
	require.True(t, ok)
	assert.Equal(t, float32(0), s.Value(), "value must not change on a refused spend")
}

// TestStockFillAllThenSpend — заполнение склада через Inventory.FillAll разблокирует
// отгрузку, а выполнение действия списывает нужное количество.
func TestStockFillAllThenSpend(t *testing.T) {
	t.Parallel()

	inv, e, ctx := newTestInventory(t,
		inventory.Ingredient{Name: "sugar", Min: 50, SpendRate: 1},
		inventory.Stock{Label: "sugar", Code: 1, XXX_Ingredient: "sugar", RegisterAdd: "ignore(?)"},
	)

	inv.FillAll(1000)

	d := e.Resolve("add.sugar(10)")
	require.NotNil(t, d)
	require.NoError(t, d.Validate())
	require.NoError(t, e.Exec(ctx, d))

	s, ok := inv.GetStockByingredientName("sugar")
	require.True(t, ok)
	assert.Equal(t, float32(990), s.Value(), "spend rate=1 over amount=10 must subtract 10")
}

// TestStockSpendAction — прямое списание через "stock.<name>.spend(?)",
// используется, например, датчиками веса/расхода в железе.
func TestStockSpendAction(t *testing.T) {
	t.Parallel()

	inv, e, _ := newTestInventory(t,
		inventory.Ingredient{Name: "cream", Min: 0, SpendRate: 2},
		inventory.Stock{Label: "cream", Code: 1, XXX_Ingredient: "cream"},
	)

	inv.FillAll(100)

	d := e.Resolve("stock.cream.spend(5)")
	require.NotNil(t, d)
	require.NoError(t, e.Exec(context.Background(), d))

	s, ok := inv.GetStockByingredientName("cream")
	require.True(t, ok)
	// translate(5, rate=2) = 10
	assert.Equal(t, float32(90), s.Value())
}

// TestStockHasBoundary — Has() должен блокировать отгрузку ровно на границе минимума.
func TestStockHasBoundary(t *testing.T) {
	t.Parallel()

	inv, _, _ := newTestInventory(t,
		inventory.Ingredient{Name: "cream", Min: 50, SpendRate: 1},
		inventory.Stock{Label: "cream", Code: 1, XXX_Ingredient: "cream"},
	)
	s, ok := inv.GetStockByingredientName("cream")
	require.True(t, ok)

	inv.FillAll(60)
	assert.True(t, s.Has(10), "60-10=50 == min(50), should be allowed")
	assert.False(t, s.Has(11), "60-11=49 < min(50), should be refused")
}
