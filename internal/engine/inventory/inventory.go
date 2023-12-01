package inventory

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"sync"
	"time"

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
		err := os.MkdirAll(sd, 0o755)
		errs = append(errs, err)
	}
	// AlexM инит директории для ошибок. надо от сюда вынести.
	sde := root + "/errors"
	if _, err := os.Stat(sde); os.IsNotExist(err) {
		err := os.MkdirAll(sde, 0o755)
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
	return m, len(m)
}

func (inv *Inventory) InventoryLoad() {
	f, _ := os.Open(inv.file)
	if f == nil {
		return
	}
	stat, _ := f.Stat()
	defer f.Close()
	td := make([]int32, stat.Size()/4)
	binary.Read(f, binary.BigEndian, &td)
	for _, cl := range inv.byCode {
		inv.byCode[cl.Code].Set(float32(td[cl.Code-1]))
	}
}

func (inv *Inventory) InventorySave() error {
	buf := new(bytes.Buffer)
	// td := make([]int32, len(inv.byCode))
	td := make([]int32, 10)
	for _, cl := range inv.byCode {
		td[int32(cl.Code-1)] = int32(cl.value)
	}
	binary.Write(buf, binary.BigEndian, td)
	err := os.WriteFile(inv.file, buf.Bytes(), 0o666)
	// check writen data
	go func(memoryValue []int32) {
		time.Sleep(5 * time.Second)
		f, _ := os.Open(inv.file)
		// stat, _ := f.Stat()
		storedValue := make([]int32, 10)
		binary.Read(f, binary.BigEndian, &storedValue)
		for i, v := range storedValue {
			if memoryValue[i] != storedValue[i] {
				inv.log.Errorf("error stored stock inventory:%d memory value(%v) stored value(%v)", i, memoryValue[i], v)
			}
		}
	}(td)
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
