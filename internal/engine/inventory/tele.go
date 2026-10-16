package inventory

import (
	"sort"
	"strconv"

	tele_api "github.com/AlexTransit/vender/tele"
)

func (inv *Inventory) Tele() *tele_api.Inventory {
	return inv.lockedTele()
}

// lockedTele читает value каждого склада через Value() (берётся только Stock.State.mu).
// Инвентарный lock не нужен: срез Stocks неизменен после Init.
func (inv *Inventory) lockedTele() *tele_api.Inventory {
	pb := &tele_api.Inventory{Stocks: make([]*tele_api.Inventory_StockItem, 0, len(inv.Stocks))}
	names := teleNames(inv.Stocks)

	for i := range inv.Stocks {
		s := &inv.Stocks[i]
		if s.Ingredient == nil {
			continue
		}
		pb.Stocks = append(pb.Stocks, &tele_api.Inventory_StockItem{
			Code:  uint32(s.Code),
			Name:  names[i],
			Value: int32(s.Value()),
		})
	}

	sort.Slice(pb.Stocks, func(a, b int) bool {
		xa := pb.Stocks[a]
		xb := pb.Stocks[b]
		if xa.Code != xb.Code {
			return xa.Code < xb.Code
		}
		return xa.Name < xb.Name
	})
	return pb
}

// teleNames возвращает имя для телеметрии по индексу склада.
// Для нескольких бункеров одного ингредиента первый по code получает базовое имя,
// следующие - с суффиксом 2, 3, ...
func teleNames(stocks []Stock) []string {
	order := make([]int, 0, len(stocks))
	for i := range stocks {
		if stocks[i].Ingredient != nil {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool {
		return stocks[order[a]].Code < stocks[order[b]].Code
	})

	names := make([]string, len(stocks))
	seen := make(map[string]int)
	for _, i := range order {
		base := stocks[i].Ingredient.Name
		seen[base]++
		if n := seen[base]; n > 1 {
			names[i] = base + strconv.Itoa(n)
		} else {
			names[i] = base
		}
	}
	return names
}
