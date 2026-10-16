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

// StockState - защищаемое состояние склада (остаток).
// Вынесено в отдельную структуру с указателем, потому что Stock копируется
// по значению при конфигурации (HCL-декодирование, XXX_Stocks, append в Stocks):
// sync.Mutex/atomic внутри копируемой структуры нарушил бы синхронизацию (go vet copylocks).
type StockState struct {
	mu    sync.Mutex
	value float32
}

// XXX временная карта
// при чтении когфига, все перекладывается в карту. ( перезапись значений )
// потом из карты переноситья в рабочий масив

// Inventory - main struct for inventory system. Contains stocks and ingredients. Also contains methods for loading and saving inventory, and for getting stock and ingredient by name.
type Inventory struct {
	ReportInv int
	log       *log2.Log
	mu        sync.RWMutex
	// Файл для хранения инвентаря. В нем хранится количество каждого склада в виде int32 (для возможности сохранения в CMOS). Код склада соответствует позиции в файле.
	// Example: если у вас 3 склада, то в файле будет 12 байт. Первые 4 байта - количество первого склада, следующие 4 байта - количество второго склада, и т.д.
	File string `hcl:"stock_file,optional"`
	// список бункеров. название склада и код одинаковые.
	// название ингридиента. должно соответствовать названию в блоке ингридиентов. нужно для связи склада и ингридиента. пока движок не переделан - это уникальное значение
	Stocks []Stock `hcl:"stock,block"`
	pools  []*Pool
	// список ингридиентов. название ингридиента должно быть уникальным. нужно для связи склада и ингридиента. пока движок не переделан - это уникальное значение
	Ingredient     []Ingredient `hcl:"ingredient,block"`
	XXX_Stocks     map[string]Stock
	XXX_Ingredient map[string]Ingredient
}

// if the minimum ingredient is not specified (equal to zero), then the consumption check is disabled
// если минимум ингридиента не указан (равен нулю), тогда проверка расхода выключена
type Ingredient struct {
	// name - название ингридиента ( иникальное значение)
	Name string `hcl:"name,label"`
	// min - минимальный остаток ( если остаток меньше минимума, то будет отказ в отгрузке.
	//       если минимум не указан то огружает в минус. может быть отрицательным числом)
	Min int `hcl:"min,optional"`
	// расход на единицу. какое количество ингридиента расходуется при отгрузке 1 единицы товара.
	SpendRate float32 `hcl:"spend_rate,optional"`
	// level - уровень (плотность) ингридиента
	// указыватеся как "x(y)"
	// x - метка на бункере, y - вес
	// пример: "0.5(200) 1(360) 2(680) 3.1(1020)"
	Level string `hcl:"level,optional"`
	// tuning_key - название кнопки коррекции отгрузки. подробне в описании кнопки коррекции
	//    кнопка коррекции. имеет название и значение по умолчанию (обычно 4) можно указать максимальное значение.
	//    единица значения равна 25%.
	//    например: базовое = 4 и это 100% если указать 2 то это 50%. если 8 то это 200%
	TuneKey string `hcl:"tuning_key,optional"`
	// cost - закупочная цена. нужна дял расчета себестоимости
	Cost       float64    `hcl:"cost,optional"`
	levelValue []struct { // used fixed comma x.xx
		lev int
		val int
	}
}

// структура для контекста
// should not use built-in type string as key for value; define your own type to avoid collisions (SA1029)
type CTXkey string

func (inv *Inventory) GetIngredientByName(ingredientName string) (*Ingredient, bool) {
	for i, v := range inv.Ingredient {
		if v.Name == ingredientName {
			return &inv.Ingredient[i], true // sucsess patch
		}
	}
	inv.log.Errorf("ingedient (%v) not found in config", ingredientName)
	return nil, false
}

func (inv *Inventory) GetStockByingredientName(name string) (*Stock, bool) {
	for i := range inv.Stocks {
		v := &inv.Stocks[i]
		if v.Ingredient == nil {
			continue
		}
		if v.Ingredient.Name == name {
			return v, true
		}
	}
	inv.log.Errorf("ingedient (%v) not included in any stock", name)
	return nil, false
}

func (inv *Inventory) Init(ctx context.Context, e *engine.Engine, log *log2.Log) (errs error) {
	inv.log = log
	inv.prepareInventory()
	fp := filepath.Dir(inv.File)
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		errs = os.MkdirAll(fp, os.ModePerm)
	}
	// sort bunkers array by code.
	sort.Slice(inv.Stocks, func(a, b int) bool {
		return inv.Stocks[a].Code < inv.Stocks[b].Code
	})
	seenCodes := make(map[int]string, len(inv.Stocks))
	for _, s := range inv.Stocks {
		switch {
		case s.Code <= 0:
			errs = errors.Join(errs, fmt.Errorf("stock=%s invalid code=%d", s.Label, s.Code))
		case s.Code > len(inv.Stocks):
			errs = errors.Join(errs, fmt.Errorf("stock=%s code=%d out of range max=%d", s.Label, s.Code, len(inv.Stocks)))
		default:
			if prev, ok := seenCodes[s.Code]; ok {
				errs = errors.Join(errs, fmt.Errorf("duplicate stock code=%d labels=%s,%s", s.Code, prev, s.Label))
			} else {
				seenCodes[s.Code] = s.Label
			}
		}
	}
	for i := range inv.Ingredient {
		inv.Ingredient[i].fillLevels()
	}
	if errs != nil {
		return errs
	}
	stockDoers := make([]engine.Doer, len(inv.Stocks))
	for i := range inv.Stocks {
		s := &inv.Stocks[i]
		if s.Ingredient == nil {
			inv.log.Errorf("in stock:%s ingridient not present", s.Label)
			continue
		}
		doSpendArg := engine.FuncArg{
			Name: fmt.Sprintf("stock.%s.spend(?)", s.Ingredient.Name),
			F:    s.spendArg,
		}
		e.Register(doSpendArg.Name, doSpendArg)
		if s.RegisterAdd == "" {
			continue
		}
		addName := fmt.Sprintf("add.%s(?)", s.Ingredient.Name)
		doAdd, err := e.ParseText(addName, s.RegisterAdd)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("stock(%s) register_add(%s) parse error(%v)", s.Ingredient.Name, s.RegisterAdd, err))
			continue
		}
		_, ok, err := engine.ArgApply(doAdd, 0)
		switch {
		case err == nil && !ok:
			errs = errors.Join(errs, fmt.Errorf("stock=%s register_add=%s no free argument", s.Ingredient.Name, s.RegisterAdd))
		case (err == nil && ok) || engine.IsNotResolved(err):
			stockDoers[i] = doAdd
		default:
			errs = errors.Join(errs, fmt.Errorf("stock=%s register_add=%s error(%v)", s.Ingredient.Name, s.RegisterAdd, err))
		}
	}
	inv.pools = newPools(inv.Stocks, stockDoers)
	for _, p := range inv.pools {
		if p.hasDoers() {
			e.Register(fmt.Sprintf("add.%s(?)", p.Ingredient.Name), p.Doer())
		}
	}
	inv.InventoryLoad()
	return errs
}

func (inv *Inventory) prepareInventory() {
	// put overrided stock to stock
	for _, v := range inv.XXX_Ingredient {
		inv.Ingredient = append(inv.Ingredient, v)
	}
	inv.XXX_Ingredient = nil

	for _, v := range inv.XXX_Stocks {
		s := v
		s.Log = inv.log
		var ok bool
		if s.Ingredient, ok = inv.GetIngredientByName(v.XXX_Ingredient); ok {
			// инициализируем защищаемое состояние ПОСЛЕ всех копирований:
			// дальше value меняется только через методы с блокировкой State.mu
			s.State = &StockState{}
			inv.Stocks = append(inv.Stocks, s)
		}
	}
	inv.XXX_Stocks = nil
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
	inv.mu.RLock()
	numInventory := len(inv.Stocks)
	inv.mu.RUnlock()
	size := 0
	if err == nil && stat != nil {
		size = int(stat.Size())
	}
	if err != nil || size != numInventory*4 {
		inv.log.Errorf("load inventory file stat. len(%d) error(%v)", size, err)
		return
	}

	td := make([]int32, numInventory)
	err = binary.Read(f, binary.BigEndian, &td)
	if err != nil {
		inv.log.Errorf("read inventory file error(%v)", err)
		return
	}
	inv.mu.Lock()
	defer inv.mu.Unlock()
	for i := range inv.Stocks {
		cl := &inv.Stocks[i]
		// защита от выхода за границы: код должен быть в диапазоне [1..numInventory],
		// иначе panic на боевом устройстве при поврежденном конфиге
		if cl.Code <= 0 || cl.Code > numInventory {
			inv.log.Errorf("load inventory skip stock=%s invalid code=%d", cl.Label, cl.Code)
			continue
		}
		inventoryValue := td[cl.Code-1]
		// ВАЖНО: value каждого склада защищён Stock.State.mu, а НЕ inv.mu —
		// иначе прямая запись cl.State.value под inv.mu гонялась бы с Value()/Set(),
		// которые берут только State.mu (детектируется -race).
		// Порядок захвата здесь строго инвертирован относительно Tele/InventorySave
		// (они отпускают inv.mu до State.mu), поэтому deadlock невозможен.
		cl.Set(float32(inventoryValue))
	}
}

// InventorySave сохраняет остатки всех складов в файл атомарно.
//
// Атомарность нужна из-за работы на Orange Pi с SD-картой: обычная запись
// через O_TRUNC оставляет файл пустым/обрезанным, если питание пропадет
// в момент записи. Поэтому данные пишутся во временный файл рядом с целевым,
// сбрасываются на диск (fsync), затем временный файл переименовывается поверх
// целевого (rename атомарен в пределах одной файловой системы), и синхронизируется
// уже каталог. После этого старый файл не может оказаться "полусостоянием".
//
// Также исправлена ошибка порядка defer: Close вызывается только при успешно
// открытом файле, иначе было бы обращение к nil (panic).
func (inv *Inventory) InventorySave() error {
	inv.mu.RLock()
	numStocks := len(inv.Stocks)
	bs := make([]byte, numStocks*4)
	var errs error
	stocks := make([]*Stock, numStocks)
	for i := range inv.Stocks {
		stocks[i] = &inv.Stocks[i]
	}
	inv.mu.RUnlock()

	// State.mu каждого склада берём ПОСЛЕ отпускания inv.mu: обратный порядок
	// (State.mu -> inv.mu) нигде не используется, поэтому такой порядок захвата
	// защищает от взаимоблокировки
	for _, cl := range stocks {
		if cl.Code <= 0 || cl.Code > numStocks {
			// защита от выхода за границы массива при некорректном коде бункера
			errs = errors.Join(errs, fmt.Errorf("save inventory skip stock=%s invalid code=%d", cl.Label, cl.Code))
			continue
		}
		pos := (cl.Code - 1) * 4
		binary.BigEndian.PutUint32(bs[pos:pos+4], uint32(cl.Value()))
	}

	if errs != nil {
		inv.log.Errorf("%v", errs)
		// сохраняем то, что смогли: позиции некорректных складов останутся нулями,
		// но прерывать сохранение нельзя — иначе потеряем все остальные остатки
	}

	dir := filepath.Dir(inv.File)
	tmp, err := os.CreateTemp(dir, ".inventory-*.tmp")
	if err != nil {
		inv.log.Errorf("save inventory fail. error create temp file(%v)", err)
		return err
	}
	tmpName := tmp.Name()
	// при любой ошибке до rename временный файл удаляется, целевой не тронут
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(bs); err != nil {
		_ = tmp.Close()
		inv.log.Errorf("save inventory fail. error write temp file(%v)", err)
		return err
	}
	// fsync гарантирует, что данные реально легли на носитель до rename
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		inv.log.Errorf("save inventory fail. error sync temp file(%v)", err)
		return err
	}
	if err := tmp.Close(); err != nil {
		inv.log.Errorf("save inventory fail. error close temp file(%v)", err)
		return err
	}
	if err := os.Rename(tmpName, inv.File); err != nil {
		inv.log.Errorf("save inventory fail. error rename temp file(%v)", err)
		return err
	}
	// rename виден другим процессам только после синхронизации каталога
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync() // не все FS поддерживают fsync каталога, поэтому ошибку не считаем фатальной
		_ = d.Close()
	} else {
		inv.log.Errorf("save inventory warn. cannot open dir for sync(%v)", err)
	}
	return nil
}

// Iter вызывает fun для каждого склада.
//
// ВАЖНО: fun вызывается БЕЗ удержания inv.mu. Если fun обращается к методам
// Inventory (GetStockByingredientName, Tele, FillAll...), это безопасно; если бы
// мы держали inv.mu, такой повторный вход привел бы к самоблокировке.
// Сами поля Stocks[] во время итерации не изменяются (Init выполняется до старта горут).
func (inv *Inventory) Iter(fun func(s *Stock)) {
	for i := range inv.Stocks {
		fun(&inv.Stocks[i])
	}
}

// FillAll sets every stock's value to v.
// Used for menu scenario validation (see state.CheckMenuExecution) and in tests
// to bring stocks to a known level before exercising add.*/spend actions.
func (inv *Inventory) FillAll(v float32) {
	inv.Iter(func(s *Stock) { s.Set(v) })
}

func (inv *Inventory) WithTuning(ctx context.Context, ingredientName string, adj float32) (context.Context, error) {
	if s, ok := inv.GetStockByingredientName(ingredientName); ok {
		if s.Ingredient.TuneKey != "" {
			tk := CTXkey(s.Ingredient.TuneKey)
			ctx = context.WithValue(ctx, tk, adj)
		}
	}
	return ctx, nil
}
