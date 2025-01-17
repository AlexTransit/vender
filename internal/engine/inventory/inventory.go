package inventory

// инвентарь - писок сладов ( бункеров ) и ингридиентов
// у ингридиента есть параметры:
// name - название ингридиента ( иникальное значение)
// min - минимальный остаток ( если остаток меньше минимума, то будет отказ в отгрузке.
//       если минимум не указан =0 то осчтаток может быть отрицательным
// level - уровень (плотность) ингридиента
//         указыватеся как "x(y)"
//         x - метка на бункере, y - вес
//         пример: "0.5(200) 1(360) 2(680) 3.1(1020)"
// tuning_key - название кнопки коррекции отгрузки. подробне в описании кнопки коррекции
//    кнопка коррекции. имеет название и значение по умолчанию (обычно 4) можно указать максимальное значение.
//    единица значения равна 25%.
//    например: базовое = 4 и это 100% если указать 2 то это 50%. если 8 то это 200%
// cost - закупочная цена. нужна дял расчета себестоимости

// склад
// label - метка склада.
//         уникальная строка
// code - опциональный чистовой код. используется для сортировки складов
// ingredient - название ингридиента ( что в бункере) пока движок не переделан - это уникальное значение

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/log2"
)

// XXX временная карта
// при чтении когфига, все перекладывается в карту. ( перезапись значений )
// потом из карты переноситья в рабочий масив

type Inventory struct {
	ReportInv      int
	log            *log2.Log
	mu             sync.RWMutex
	File           string       `hcl:"stock_file,optional"`
	Stocks         []Stock      `hcl:"stock,block"`
	Ingredient     []Ingredient `hcl:"ingredient,block"`
	XXX_Stocks     map[string]Stock
	XXX_Ingredient map[string]Ingredient
}

// if the minimum ingredient is not specified (equal to zero), then the consumption check is disabled
// если минимум ингридиента не указан (равен нулю), тогда проверка расхода выключена
type Ingredient struct {
	Name       string     `hcl:"name,label"`
	Min        int        `hcl:"min,optional"`
	SpendRate  float32    `hcl:"spend_rate,optional"`
	Level      string     `hcl:"level,optional"`
	TuneKey    string     `hcl:"tuning_key,optional"`
	Cost       float64    `hcl:"cost,optional"`
	levelValue []struct { // used fixed comma x.xx
		lev int
		val int
	}
}

// структура для контекста
// should not use built-in type string as key for value; define your own type to avoid collisions (SA1029)
type CTXkey string

func (inv *Inventory) GetIngredientByName(ingredientName string) *Ingredient {
	for i, v := range inv.Ingredient {
		if v.Name == ingredientName {
			return &inv.Ingredient[i] // sucsess patch
		}
	}
	inv.log.Errorf("ingredient:%s not found in config", ingredientName)
	return nil
}

func (inv *Inventory) GetStockByingredientName(name string) *Stock {
	for i, v := range inv.Stocks {
		if v.Ingredient.Name == name {
			return &inv.Stocks[i]
		}
	}
	inv.log.Errorf("ingedisent (%v) not found in config", name)
	panic("ingredient not found")
	// return nil
}

func (inv *Inventory) Init(ctx context.Context, e *engine.Engine) (errs error) {
	inv.log = log2.ContextValueLogger(ctx)
	inv.mu.Lock()
	defer inv.mu.Unlock()
	// errs := make([]error, 0)
	// sd := root + "/inventory"
	fp := filepath.Dir(inv.File)
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		errs = os.MkdirAll(fp, os.ModePerm)
	}
	// sort bunkers array by code.
	sort.Slice(inv.Stocks, func(a, b int) bool {
		return inv.Stocks[a].Code < inv.Stocks[b].Code
	})
	for i := range inv.Ingredient {
		inv.Ingredient[i].fillLevels()
	}
	for i, s := range inv.Stocks {
		if s.Ingredient == nil {
			inv.log.Errorf("in stock:%s ingridient not present", s.Label)
			continue
		}
		doSpendArg := engine.FuncArg{
			Name: fmt.Sprintf("stock.%s.spend(?)", s.Ingredient.Name),
			F:    s.spendArg,
		}
		addName := fmt.Sprintf("add.%s(?)", s.Ingredient.Name)
		if s.RegisterAdd != "" {
			doAdd, err := e.ParseText(addName, s.RegisterAdd)
			if err != nil {
				return errors.Join(errs, fmt.Errorf("stock(%s) register_add(%s) parse error(%v)", s.Ingredient.Name, s.RegisterAdd, err))
			}
			_, ok, err := engine.ArgApply(doAdd, 0)
			switch {
			case err == nil && !ok:
				return errors.Join(errs, fmt.Errorf("stock=%s register_add=%s no free argument", s.Ingredient.Name, s.RegisterAdd))

			case (err == nil && ok) || engine.IsNotResolved(err): // success path
				e.Register(addName, inv.Stocks[i].Wrap(doAdd))

			case err != nil:
				return errors.Join(err, fmt.Errorf("stock=%s register_add=%s error(%v)", s.Ingredient.Name, s.RegisterAdd, err))
			}

		}
		e.Register(doSpendArg.Name, doSpendArg)
	}
	return errs
}

// store file
// инвентарь храниться как int32 ( для возможности сохраннения в CMOS)
// код соответствует позиции в файле
func (inv *Inventory) InventoryLoad() {
	f, err := os.OpenFile(inv.File, os.O_RDONLY|os.O_SYNC|os.O_CREATE, 0o644)
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
	file, err := os.OpenFile(inv.File, os.O_WRONLY|os.O_SYNC|os.O_CREATE, 0o644)
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

func (inv *Inventory) WithTuning(ctx context.Context, ingredientName string, adj float32) (context.Context, error) {
	if s := inv.GetStockByingredientName(ingredientName); s != nil {
		if s.Ingredient.TuneKey != "" {
			tk := CTXkey(s.Ingredient.TuneKey)
			ctx = context.WithValue(ctx, tk, adj)
		}
	}
	return ctx, nil
}
