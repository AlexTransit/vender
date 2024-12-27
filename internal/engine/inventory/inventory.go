package inventory

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
)

type Inventory struct {
	log              *log2.Log
	Persist          bool    `hcl:"persist,optional"`
	TeleAddName      bool    `hcl:"tele_add_name,optional"`
	XXX_Stocks       []Stock `hcl:"stock,block"`
	Stocks           map[string]Stock
	StocksNameByCode map[int]string
	mu               sync.RWMutex
	file             string

	// config *inventory_config.Inventory
}

// type Inventory struct {
// 	// persist.Persist
// 	byName map[string]*Stock
// 	byCode map[uint32]*Stock
// }

func (inv *Inventory) Init(ctx context.Context, e *engine.Engine, root string) error {
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
	for _, s := range inv.Stocks {
		s.fillLevels()
		doSpend1 := engine.Func0{
			Name: fmt.Sprintf("stock.%s.spend1", s.Name),
			F:    s.spend1,
		}
		doSpendArg := engine.FuncArg{
			Name: fmt.Sprintf("stock.%s.spend(?)", s.Name),
			F:    s.spendArg,
		}
		addName := fmt.Sprintf("add.%s(?)", s.Name)
		if s.RegisterAdd != "" {
			doAdd, err := e.ParseText(addName, s.RegisterAdd)
			if err != nil {
				return fmt.Errorf("stock(%s) register_add(%s) parse error(%v)", s.Name, s.RegisterAdd, err)
			}
			_, ok, err := engine.ArgApply(doAdd, 0)
			switch {
			case err == nil && !ok:
				return errors.Errorf("stock=%s register_add=%s no free argument", s.Name, s.RegisterAdd)

			case (err == nil && ok) || engine.IsNotResolved(err): // success path
				e.Register(addName, s.Wrap(doAdd))

			case err != nil:
				return errors.Errorf("stock=%s register_add=%s error(%v)", s.Name, s.RegisterAdd, err)
			}

		}
		e.Register(doSpend1.Name, doSpend1)
		e.Register(doSpendArg.Name, doSpendArg)
	}
	return helpers.FoldErrors(errs)
}

// func (s *stock) fillLevels() {
// 	rm := `([0-9]*[.,]?[0-9]+)\(([0-9]*[.,]?[0-9]+)\)`
// 	parts := regexp.MustCompile(rm).FindAllStringSubmatch(c.Level, 50)
// 	s.level = make([]struct {
// 		lev int
// 		val int
// 	}, len(parts)+1)

// 	if len(parts) == 0 {
// 		return
// 	}

// 	for i, v := range parts {
// 		s.level[i+1].lev = stringToFixInt(v[1])
// 		s.level[i+1].val = stringToFixInt(v[2])
// 	}
// 	sort.Slice(s.level, func(i, j int) bool {
// 		return s.level[i].lev < s.level[j].lev
// 	})
// }

// store file
// инвентарь храниться как int32 ( для возможности сохраннения в CMOS)
// код соответствует позиции в файле
func (inv *Inventory) InventoryLoad() {
	f, err := os.OpenFile(inv.file, os.O_RDONLY|os.O_SYNC|os.O_CREATE, 0o644)
	if err != nil {
		inv.log.Errorf("problem load inventory error(%v)", err)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	fl := int(stat.Size())
	numInventory := len(inv.Stocks)
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
	for _, cl := range inv.Stocks {
		cl.Set(float32(td[cl.Code-1]))
	}
}

func (inv *Inventory) InventorySave() error {
	file, err := os.OpenFile(inv.file, os.O_WRONLY|os.O_SYNC|os.O_CREATE, 0o644)
	if err != nil {
		inv.log.Errorf("save inventory fail. error open file(%v)", err)
		return err
	}
	defer file.Close()

	bs := make([]byte, len(inv.Stocks)*4)
	for _, cl := range inv.Stocks {
		pos := (cl.Code - 1) * 4
		binary.BigEndian.PutUint32(bs[pos:pos+4], uint32(cl.value))
	}
	_, err = file.Write(bs)
	if err != nil {
		inv.log.Errorf("save inventory fail. error write file(%v)", err)
		return err
	}
	return err
}

// func (inv *Inventory) Get(name string) (s *Stock, err error) {
// 	inv.mu.RLock()
// 	defer inv.mu.RUnlock()
// 	a := inv.Stocks[name]

// 	// if inv.Stocks[name]
// 	// if s, ok := inv.locked_get(0, name); ok {
// 	// 	return s, nil
// 	// }
// 	return nil, errors.Errorf("stock=%s is not registered", name)
// }

func (inv *Inventory) Iter(fun func(s *Stock)) {
	inv.mu.Lock()
	for _, stock := range inv.Stocks {
		fun(&stock)
	}
	inv.mu.Unlock()
}

func (inv *Inventory) WithTuning(ctx context.Context, stockName string, adj float32) (context.Context, error) {
	// stock, err := inv.Get(stockName)
	// if err != nil {
	// 	return ctx, errors.Annotate(err, "WithTuning")
	// }
	// ctx = context.WithValue(ctx, stock.TuneKey, adj)
	tk := inv.Stocks[stockName].TuneKey
	if tk != "" {
		ctx = context.WithValue(ctx, tk, adj)
	}
	return ctx, nil
}

// func (inv *Inventory) locked_get(code uint32, name string) (*Stock, bool) {
// 	if name == "" {
// 		s, ok := inv.Stocks[code]
// 		return s, ok
// 	}
// 	s, ok := inv.byName[name]
// 	return s, ok
// }

// func (inv *Inventory) DisableCheckInStock() (listDisabledIngrodoent []uint32) {
// 	for i, v := range inv.byCode {
// 		if inv.byCode[i].check {
// 			listDisabledIngrodoent = append(listDisabledIngrodoent, i)
// 		}
// 		inv.byCode[i].check = false
// 		inv.byName[v.Name].check = false
// 	}
// 	return listDisabledIngrodoent
// }

// func (inv *Inventory) EnableCheckInStock(listEnabledIngrodoent *[]uint32) {
// 	for _, v := range *listEnabledIngrodoent {
// 		inv.byCode[v].check = true
// 		// inv.byName[v.Name].check = false
// 	}
// }
