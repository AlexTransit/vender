package inventory

import (
	"bytes"
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

var (
	ErrStockLow = errors.New("Stock is too low")
)

type Inventory struct {
	// persist.Persist
	config   *engine_config.Inventory
	log      *log2.Log
	mu       sync.RWMutex
	byName   map[string]*Stock
	byCode   map[uint32]*Stock
	codeList []uint32
	file     string
}

func (inv *Inventory) Init(ctx context.Context, c *engine_config.Inventory, engine *engine.Engine, root string) error {
	inv.config = c
	inv.initOverWriteStocks()
	inv.log = log2.ContextValueLogger(ctx)

	inv.mu.Lock()
	defer inv.mu.Unlock()
	errs := make([]error, 0)
	sd := root + "/inventory"
	if _, err := os.Stat(sd); os.IsNotExist(err) {
		err := os.MkdirAll(sd, 0700)
		errs = append(errs, err)
	}
	// AlexM инит директории для ошибок. надо от сюда вынести.
	sde := root + "/errors"
	if _, err := os.Stat(sde); os.IsNotExist(err) {
		err := os.MkdirAll(sde, 0700)
		errs = append(errs, err)
	}
	inv.file = sd + "/store.file"
	inv.byName = make(map[string]*Stock, len(c.Stocks))
	inv.byCode = make(map[uint32]*Stock, len(c.Stocks))
	inv.codeList = make([]uint32, len(c.Stocks))
	i := 0
	for _, stockConfig := range c.Stocks {
		if _, ok := inv.byName[stockConfig.Name]; ok {
			errs = append(errs, errors.Errorf("stock=%s already registered", stockConfig.Name))
			continue
		}

		stock, err := NewStock(stockConfig, engine)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		inv.byName[stock.Name] = stock
		inv.codeList[i] = stock.Code
		i++
		if first, ok := inv.byCode[stock.Code]; !ok {
			inv.byCode[stock.Code] = stock
		} else {
			inv.log.Errorf("stock=%s duplicate code=%d first=%s", stock.Name, stock.Code, first)
		}
	}

	return helpers.FoldErrors(errs)
}

func (inv *Inventory) initOverWriteStocks() {
	m := make(map[int]engine_config.Stock)
	for _, v := range inv.config.Stocks {
		if v.Code == 0 {
			continue
		}
		n := v.Code
		if m[n].Code == 0 {
			m[n] = v
			continue
		}
		ss := m[n]
		if v.Name != "" {
			ss.Name = v.Name
		}
		if v.HwRate != 0 {
			ss.HwRate = v.HwRate
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
	inv.config.Stocks = nil
	inv.config.Stocks = make([]engine_config.Stock, len(m))
	i := 0
	for _, v := range m {
		inv.config.Stocks[i] = v
		i++
	}
}

func (inv *Inventory) InventoryLoad() {
	f, _ := os.Open(inv.file)
	defer f.Close()
	count := len(inv.codeList)
	td := make([]int32, count)
	binary.Read(f, binary.BigEndian, &td)
	for i, cl := range inv.codeList {
		inv.byCode[cl].Set(float32(td[i]))
	}
}

func (inv *Inventory) InventorySave() error {
	count := len(inv.byCode)
	buf := new(bytes.Buffer)
	td := make([]int32, count)
	for i, cl := range inv.codeList {
		td[i] = int32(inv.byCode[cl].value)
	}
	binary.Write(buf, binary.BigEndian, td)

	return os.WriteFile(inv.file, buf.Bytes(), 0600)
}

// func (inv *Inventory) EnableAll()  { inv.Iter(func(s *Stock) { s.Enable() }) }
// func (inv *Inventory) DisableAll() { inv.Iter(func(s *Stock) { s.Disable() }) }

func (inv *Inventory) Get(name string) (*Stock, error) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	if s, ok := inv.locked_get(0, name); ok {
		return s, nil
	}
	return nil, errors.Errorf("stock=%s is not registered", name)
}

// func (inv *Inventory) MustGet(f interface{ Fatal(...interface{}) }, name string) *Stock {
// 	s, err := inv.Get(name)
// 	if err != nil {
// 		f.Fatal(err)
// 		return nil
// 	}
// 	return s
// }

func (inv *Inventory) Iter(fun func(s *Stock)) {
	inv.mu.Lock()
	for _, stock := range inv.byName {
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
