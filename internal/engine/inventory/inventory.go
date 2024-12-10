package inventory

import (
	"context"
	"encoding/binary"
	"os"
	"sync"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	engine_config "github.com/AlexTransit/vender/internal/engine/config"
	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
)

type Inventory struct {
	// persist.Persist
	config *engine_config.Inventory
	log    *log2.Log
	mu     sync.RWMutex
	byName map[string]*Stock
	byCode map[uint32]*Stock
	file   string
}

func (inv *Inventory) Init(ctx context.Context, c *engine_config.Inventory, engine *engine.Engine, root string) error {
	inv.config = c
	bunkers, countBunkers := initOverWriteStocks(c)
	inv.config.Stocks = nil
	inv.log = log2.ContextValueLogger(ctx)

	inv.mu.Lock()
	defer inv.mu.Unlock()
	errs := make([]error, 0)
	sd := root + "/inventory"
	if _, err := os.Stat(sd); os.IsNotExist(err) {
		err := os.MkdirAll(sd, os.ModePerm)
		errs = append(errs, err)
	}
	// AlexM инит директории для ошибок. надо от сюда вынести.
	sde := root + "/errors"
	if _, err := os.Stat(sde); os.IsNotExist(err) {
		err := os.MkdirAll(sde, os.ModePerm)
		errs = append(errs, err)
	}
	inv.file = sd + "/store.file"

	inv.byName = make(map[string]*Stock, countBunkers)
	inv.byCode = make(map[uint32]*Stock, countBunkers)
	for _, stockConfig := range bunkers {

		stock, err := NewStock(stockConfig, engine)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		inv.byName[stock.Name] = stock
		inv.byCode[stock.Code] = stock
	}

	return helpers.FoldErrors(errs)
}

func initOverWriteStocks(c *engine_config.Inventory) (m map[uint32]engine_config.Stock, countBunkers int) {
	m = make(map[uint32]engine_config.Stock)
	for _, v := range c.Stocks {
		if v.Code == 0 {
			continue
		}
		n := uint32(v.Code)
		if m[n].Code == 0 {
			m[n] = v
			continue
		}
		ss := m[n]
		if v.Name != "" {
			ss.Name = v.Name
		}
		if v.Level != "" {
			ss.Level = v.Level
		}
		if v.Min != 0 {
			ss.Min = v.Min
		}
		if v.RegisterAdd != "" {
			ss.RegisterAdd = v.RegisterAdd
		}
		if v.SpendRate != 0 {
			ss.SpendRate = v.SpendRate
		}
		m[n] = ss
	}
	return m, len(m)
}

func (inv *Inventory) InventoryLoad() {
	f, err := os.OpenFile(inv.file, os.O_RDONLY|os.O_SYNC|os.O_CREATE, 0o644)
	if err != nil {
		inv.log.Errorf("problem load inventory error(%v)", err)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	fl := int(stat.Size())
	numInventory := len(inv.byCode)
	if err != nil || fl != numInventory*4 {
		inv.log.Errorf("load inventory file stat. len(%d) error(%v)", fl, err)
		return
	}

	td := make([]int32, numInventory)
	err = binary.Read(f, binary.BigEndian, &td)
	if err != nil {
		inv.log.Errorf("read inventory file error(%v)", err)
		return
	}
	for _, cl := range inv.byCode {
		inv.byCode[cl.Code].Set(float32(td[cl.Code-1]))
	}
}

func (inv *Inventory) InventorySave() error {
	file, err := os.OpenFile(inv.file, os.O_WRONLY|os.O_SYNC|os.O_CREATE, 0o644)
	if err != nil {
		inv.log.Errorf("save inventory fail. error open file(%v)", err)
		return err
	}
	defer file.Close()

	bs := make([]byte, len(inv.byCode)*4)
	for i, cl := range inv.byCode {
		pos := (i - 1) * 4
		binary.BigEndian.PutUint32(bs[pos:pos+4], uint32(cl.value))
	}
	_, err = file.Write(bs)
	if err != nil {
		inv.log.Errorf("save inventory fail. error write file(%v)", err)
		return err
	}
	return err
}

func (inv *Inventory) Get(name string) (*Stock, error) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	if s, ok := inv.locked_get(0, name); ok {
		return s, nil
	}
	return nil, errors.Errorf("stock=%s is not registered", name)
}

func (inv *Inventory) Iter(fun func(s *Stock)) {
	inv.mu.Lock()
	for _, stock := range inv.byCode {
		fun(stock)
	}
	inv.mu.Unlock()
}

func (inv *Inventory) WithTuning(ctx context.Context, stockName string, adj float32) (context.Context, error) {
	stock, err := inv.Get(stockName)
	if err != nil {
		return ctx, errors.Annotate(err, "WithTuning")
	}
	ctx = context.WithValue(ctx, stock.tuneKey, adj)
	return ctx, nil
}

func (inv *Inventory) locked_get(code uint32, name string) (*Stock, bool) {
	if name == "" {
		s, ok := inv.byCode[code]
		return s, ok
	}
	s, ok := inv.byName[name]
	return s, ok
}

func (inv *Inventory) DisableCheckInStock() (listDisabledIngrodoent []uint32) {
	for i, v := range inv.byCode {
		if inv.byCode[i].check {
			listDisabledIngrodoent = append(listDisabledIngrodoent, i)
		}
		inv.byCode[i].check = false
		inv.byName[v.Name].check = false
	}
	return listDisabledIngrodoent
}

func (inv *Inventory) EnableCheckInStock(listEnabledIngrodoent *[]uint32) {
	for _, v := range *listEnabledIngrodoent {
		inv.byCode[v].check = true
		// inv.byName[v.Name].check = false
	}
}
