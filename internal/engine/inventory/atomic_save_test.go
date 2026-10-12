package inventory

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexTransit/vender/log2"
)

// TestInventorySaveLoadConcurrent - регрессия на дедлок и гонку данных:
// Inventory.Init больше не удерживает inv.mu при вызове InventoryLoad
// (самоблокировка RWMutex вешала весь пакет), а InventorySave/InventoryLoad
// читают и пишут значения складов через Stock.State.mu. Параллельные выдача,
// телеметрия, сохранение и загрузка не должны приводить ни к взаимоблокировке,
// ни к повреждению файла остатков (атомарная запись write->fsync->rename).
func TestInventorySaveLoadConcurrent(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "stock.bin")
	inv := &Inventory{File: f, log: log2.NewWriter(io.Discard, log2.LOG_DEBUG)}
	ing := &Ingredient{Name: "a", SpendRate: 1}
	for i := 1; i <= 4; i++ {
		inv.Stocks = append(inv.Stocks, Stock{Label: "s", Code: i, Ingredient: ing, State: &StockState{}})
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(7)
	// постоянные сохранения и загрузки (как ui-service после выдачи и старт)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = inv.InventorySave()
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				inv.InventoryLoad()
			}
		}
	}()
	// параллельное изменение остатков (выдача товара)
	for k := 0; k < 4; k++ {
		i := k
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					inv.Stocks[i].Set(float32(i))
					_ = inv.Stocks[i].Value()
					inv.Stocks[i].SpendValue(1)
				}
			}
		}()
	}
	// чтение телеметрией (берет State.mu каждого склада)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = inv.Tele()
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stop)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock detected: concurrent Save/Load/Set did not finish")
	}

	// детерминированный round-trip: файл должен остаться целостным
	inv.Stocks[3].Set(7)
	if err := inv.InventorySave(); err != nil {
		t.Fatalf("InventorySave: %v", err)
	}
	inv.Stocks[3].Set(0)
	inv.InventoryLoad()
	if got := inv.Stocks[3].Value(); got != 7 {
		t.Fatalf("value after roundtrip = %v, want 7 (file corrupted?)", got)
	}

	// временных файлов (.inventory-*.tmp) остаться не должно
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".inventory-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}
