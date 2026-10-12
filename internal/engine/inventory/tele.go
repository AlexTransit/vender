package inventory

import (
	"sort"

	tele_api "github.com/AlexTransit/vender/tele"
)

func (inv *Inventory) Tele() *tele_api.Inventory {
	return inv.lockedTele()
}

// locked_tele читает value каждого склада через Value() (берётся только Stock.State.mu).
// Инвентарный lock здесь не нужен: срез Stocks неизменен после Init, а отдельные
// значения читаются под блокировкой состояния.
// Порядок захвата блокировок в пакете: сначала inv.mu (кратковременно), затем State.mu -
// так исключается взаимоблокировка с InventorySave.
func (inv *Inventory) lockedTele() *tele_api.Inventory {
	pb := &tele_api.Inventory{Stocks: make([]*tele_api.Inventory_StockItem, 0, 16)}

	for i := range inv.Stocks {
		s := &inv.Stocks[i]
		if s.Ingredient == nil {
			continue
		}
		value := s.Value()
		if value != 0 {
			si := &tele_api.Inventory_StockItem{
				Code: uint32(s.Code),
				// XXX TODO retype Value to float
				Value: int32(value),
				// Valuef: s.Value(),
			}
			si.Name = s.Ingredient.Name
			pb.Stocks = append(pb.Stocks, si)
		}
	}
	// Predictable ordering is not really needed, currently used only for testing
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
