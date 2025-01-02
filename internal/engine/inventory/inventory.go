package inventory

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
)

type Inventory struct {
	log        *log2.Log
	mu         sync.RWMutex
	file       string //`hcl:"stock_file,optional"`
	Stocks     []Stock
	Ingredient []Ingredient
}

// временная структура.
// конфигурация читается во временную
// в карту кладет значения включая корректировку из других конфигов
// при инициализации формируется рабочий инвентарь
type XXX_Inventory struct {
	Conf_Loaded_Stocks     []Conf_Stock      `hcl:"stock,block"`
	Conf_Loaded_Ingredient []Conf_Ingredient `hcl:"ingredient,block"`
	XXX_Stocks             map[int]Conf_Stock
	XXX_Ingredient         map[string]Conf_Ingredient
}

// if the minimum ingredient is not specified (equal to zero), then the consumption check is disabled
// если минимум ингридиента не указан (равен нулю), тогда проверка расхода выключена
type Ingredient struct {
	*Conf_Ingredient
	levelValue []struct { // used fixed comma x.xx
		lev int
		val int
	}
}

type Conf_Ingredient struct {
	Name      string  `hcl:"name,label"`
	Min       int     `hcl:"min,optional"`
	SpendRate float32 `hcl:"spend_rate,optional"`
	Level     string  `hcl:"level,optional"`
	TuneKey   string  `hcl:"tuning_key,optional"`
}

func (inv *Inventory) GetIngredientByName(ingredientName string) *Ingredient {
	for i, v := range inv.Ingredient {
		if v.Name == ingredientName {
			return &inv.Ingredient[i] // sucsess patch
		}
	}
	inv.log.Errorf("ingredient:%s not found in config", ingredientName)
	return nil
}

func (inv *Inventory) GetStockByName(name string) *Stock {
	for i, v := range inv.Stocks {
		if v.Ingredient.Name == name {
			return &inv.Stocks[i]
		}
	}
	return nil
}

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
	// sort bunkers array by code.
	sort.Slice(inv.Stocks, func(a, b int) bool {
		xa := inv.Stocks[a]
		xb := inv.Stocks[b]
		if xa.Code != xb.Code {
			return xa.Code < xb.Code
		}
		return xa.Name < xb.Name
	})
	for i := range inv.Ingredient {
		inv.Ingredient[i].fillLevels()
	}
	for i, s := range inv.Stocks {
		doSpend1 := engine.Func0{
			Name: fmt.Sprintf("stock.%s.spend1", s.Ingredient.Name),
			F:    s.spend1,
		}
		doSpendArg := engine.FuncArg{
			Name: fmt.Sprintf("stock.%s.spend(?)", s.Ingredient.Name),
			F:    s.spendArg,
		}
		addName := fmt.Sprintf("add.%s(?)", s.Ingredient.Name)
		if s.RegisterAdd != "" {
			doAdd, err := e.ParseText(addName, s.RegisterAdd)
			if err != nil {
				return fmt.Errorf("stock(%s) register_add(%s) parse error(%v)", s.Ingredient.Name, s.RegisterAdd, err)
			}
			_, ok, err := engine.ArgApply(doAdd, 0)
			switch {
			case err == nil && !ok:
				return errors.Errorf("stock=%s register_add=%s no free argument", s.Ingredient.Name, s.RegisterAdd)

			case (err == nil && ok) || engine.IsNotResolved(err): // success path
				e.Register(addName, inv.Stocks[i].Wrap(doAdd))

			case err != nil:
				return errors.Errorf("stock=%s register_add=%s error(%v)", s.Ingredient.Name, s.RegisterAdd, err)
			}

		}
		e.Register(doSpend1.Name, doSpend1)
		e.Register(doSpendArg.Name, doSpendArg)
	}
	return helpers.FoldErrors(errs)
}

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
	for i, cl := range inv.Stocks {
		inv.Stocks[i].Set(float32(td[cl.Code-1]))
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
	if s := inv.GetStockByName(stockName); s != nil {
		if tk := s.Ingredient.TuneKey; tk != "" {
			ctx = context.WithValue(ctx, tk, adj)
		}
	}
	// tk := inv.Stocks[stockName].TuneKey
	// if tk != "" {
	// 	ctx = context.WithValue(ctx, tk, adj)
	// }
	return ctx, nil
}

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
