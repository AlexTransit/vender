package inventory

import (
	"sort"

	tele_api "github.com/AlexTransit/vender/tele"
)

func (inv *Inventory) Tele() *tele_api.Inventory {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return inv.locked_tele()
}

func (inv *Inventory) locked_tele() *tele_api.Inventory {
	pb := &tele_api.Inventory{Stocks: make([]*tele_api.Inventory_StockItem, 0, 16)}

	for _, s := range inv.Stocks {
		if s.value != 0 {
			si := &tele_api.Inventory_StockItem{
				Code: uint32(s.Code),
				// XXX TODO retype Value to float
				Value: int32(s.value),
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
