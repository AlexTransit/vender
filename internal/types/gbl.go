package types

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/log2"
)

var Log = *log2.NewStderr(log2.LDebug)
var VMC *VMCType = nil
var UI *UItype = nil

type VMCType struct {
	Version string
	Lock    bool
	State   int32
	Client  struct {
		WorkTime time.Time
		Input    string
		Light    bool
	}
	HW struct {
		Input   bool
		Display struct {
			L1            string
			L2            string
			GdisplayValid bool
			Gdisplay      string
		}
		Elevator struct {
			Position uint8
		}
		Temperature int
	}
	MonSys MonSysStruct
}
type MonSysStruct struct {
	Dirty   currency.Amount
	BillOn  bool
	BillRun bool
}

type UItype struct { //nolint:maligned
	FrontResult UIMenuResult
	Menu        map[string]MenuItemType
}

type UIMenuResult struct {
	Item     MenuItemType
	Cream    uint8
	Sugar    uint8
	Accepted bool
}

func (mit *MenuItemType) String() string {
	return fmt.Sprintf("menu code=%s price=%d(raw) name='%s'", mit.Code, mit.Price, mit.Name)
}

type MenuItemType struct {
	Name  string
	D     Doer
	Price currency.Amount
	Code  string
}

type Doer interface {
	Validate() error
	Do(context.Context) error
	String() string // for logs
}

func init() {
	Log.SetFlags(0)
	VMC = new(VMCType)
	UI = new(UItype)
}

func SetLight(v bool) {
	if VMC.Client.Light == v {
		return
	}
	VMC.Client.Light = v
	Log.Infof("light = %v", v)
}

func ShowEnvs() string {
	s := fmt.Sprintf("GBL=%+v", VMC)
	Log.Info(s)
	return s
}

// преобразование тюнинга в байт
// 0 = дефолтные значение. если менялось то +1 для телеметрии
// convert tuning to byte
// 0 = default value. if changed then +1 for telemetry
func TuneValueToByte(currentValue uint8, defaultValue int) []byte {
	if currentValue == uint8(defaultValue) {
		return nil
	}
	return []byte{currentValue + 1}
}
