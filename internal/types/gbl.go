package types

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/log2"

	// "github.com/AlexTransit/vender/log2"
	tele_api "github.com/AlexTransit/vender/tele"
)

// var Log = *log2.NewStderr(log2.LDebug)
var (
	VMC   *VMCType = nil
	UI    *UItype  = nil
	TeleN tele_api.Teler
	Log   *log2.Log
)

type VMCType struct {
	Version     string
	Lock        bool
	NeedRestart bool
	InputEnable bool
	UiState     uint32
	ReportInv   uint32
	Client      struct {
		WorkTime time.Time
		Input    string
		Light    bool
	}
	HW struct {
		Display struct {
			L1            string
			L2            string
			GdisplayValid bool
			Gdisplay      string
		}
	}
	MonSys MonSysStruct
}
type MonSysStruct struct {
	Dirty currency.Amount
}

type UItype struct { //nolint:maligned
	FrontResult UIMenuResult
	Menu        map[string]MenuItemType
}

type UIMenuResult struct {
	Item        MenuItemType
	Cream       uint8
	Sugar       uint8
	PaymenId    int64
	QRPayAmount uint32
}

func (mit *MenuItemType) String() string {
	return fmt.Sprintf("menu code=%s price=%d(raw) name='%s'", mit.Code, mit.Price, mit.Name)
}

type MenuItemType struct {
	Disabled bool
	Name     string
	D        Doer
	Price    currency.Amount
	Code     string
	CreamMax uint8
	SugarMax uint8
}

type Doer interface {
	Validate() error
	Do(context.Context) error
	String() string // for logs
}

func init() {
	VMC = new(VMCType)
	UI = new(UItype)
}

func ShowEnvs() string {
	s := fmt.Sprintf("GBL=%+v", VMC)
	// Log.Info(s)
	return s
}

// преобразование тюнинга в байт
// 0 = дефолтные значение. если менялось то +1 для телеметрии
// convert tuning to byte
// 0 = default value. if changed then +1 for telemetry
func TuneValueToByte(currentValue uint8, defaultValue uint8) []byte {
	if currentValue == defaultValue {
		return nil
	}
	return []byte{currentValue + 1}
}

func (evk *VMCType) EvendKeyboardInput(v bool) {
	if evk.InputEnable == v {
		return
	}
	// Log.Infof("evend keyboard: %v", v)
	evk.InputEnable = v
}

func TeleError(s string) {
	// Log.Info(s)
	TeleN.ErrorStr(s)
}
