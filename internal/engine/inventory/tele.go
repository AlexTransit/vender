package inventory

import (
	fmt "fmt"
	"sort"

	"github.com/AlexTransit/vender/helpers"
	tele_api "github.com/AlexTransit/vender/tele"
)

func (inv *Inventory) SetTele(src *tele_api.Inventory) (*tele_api.Inventory, error) {
	const tag = "inventory.SetTele"
	inv.mu.Lock()
	defer inv.mu.Unlock()

	inv.log.Debugf("%s src=%s", tag, src.String())
	if src == nil {
		return inv.locked_tele(), nil
	}

	// validate
	errs := make([]error, 0, len(src.Stocks))
	for _, new := range src.Stocks {
		if _, ok := inv.locked_get(new.Code, new.Name); !ok {
			err := fmt.Errorf("stock name=%s code=%d not found", new.Name, new.Code)
			inv.log.Errorf("%s %s", tag, err.Error())
			errs = append(errs, err)
		}
	}
	for _, new := range src.Stocks {
		if len(errs) != 0 {
			break
		}
		if stock, ok := inv.locked_get(new.Code, new.Name); ok {
			stock.Set(new.Valuef)
		}
	}

	return inv.locked_tele(), helpers.FoldErrors(errs)
}

func (inv *Inventory) Tele() *tele_api.Inventory {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return inv.locked_tele()
}

func (inv *Inventory) locked_tele() *tele_api.Inventory {
	pb := &tele_api.Inventory{Stocks: make([]*tele_api.Inventory_StockItem, 0, 16)}

	for _, s := range inv.byName {
		if s.Enabled() {
			si := &tele_api.Inventory_StockItem{
				Code: s.Code,
				// XXX TODO retype Value to float
				Value: int32(s.Value()),
				// Valuef: s.Value(),
			}
			if inv.config.TeleAddName {
				si.Name = s.Name
			}
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
