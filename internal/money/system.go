package money

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/mdb/bill"
	"github.com/AlexTransit/vender/hardware/mdb/coin"
	"github.com/temoto/alive/v2"

	// "github.com/AlexTransit/vender/hardware/money"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"

	// "github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/log2"
	tele_api "github.com/AlexTransit/vender/tele"

	// "github.com/golang/protobuf/proto"
	oerr "github.com/juju/errors"
)

type MoneySystem struct { //nolint:maligned
	Log   *log2.Log
	lk    sync.RWMutex
	dirty currency.Amount // uncommited

	bill         bill.Biller
	billCashbox  currency.NominalGroup
	billCredit   currency.NominalGroup
	billReinited bool
	// enableBillChanger bool

	coin        coin.Coiner
	coinCashbox currency.NominalGroup
	coinCredit  currency.NominalGroup

	giftCredit currency.Amount
}

func GetGlobal(ctx context.Context) *MoneySystem {
	return state.GetGlobal(ctx).XXX_money.Load().(*MoneySystem)
}

func (ms *MoneySystem) Start(ctx context.Context) error {
	g := state.GetGlobal(ctx)

	ms.lk.Lock()
	defer ms.lk.Unlock()
	ms.Log = g.Log
	g.XXX_money.Store(ms)
	const devNameBill = "bill"
	const devNameCoin = "coin"
	ms.bill = bill.Stub{}
	ms.coin = coin.Stub{}
	// ms.enableBillChanger = g.Config.Money.EnableChangeBillToCoin
	errs := make([]error, 0, 2)
	if dev, err := g.GetDevice(devNameBill); err == nil {
		ms.bill = dev.(bill.Biller)
	} else if oerr.IsNotFound(err) {
		ms.Log.Debugf("device=%s is not enabled in config", devNameBill)
	} else {
		errs = append(errs, oerr.Annotatef(err, "device=%s", devNameBill))
	}
	if dev, err := g.GetDevice(devNameCoin); err == nil {
		ms.coin = dev.(coin.Coiner)
	} else if oerr.IsNotFound(err) {
		ms.Log.Debugf("device=%s is not enabled in config", devNameCoin)
	} else {
		errs = append(errs, oerr.Annotatef(err, "device=%s", devNameCoin))
	}
	if e := helpers.FoldErrors(errs); e != nil {
		return e
	}

	ms.billCashbox.SetValid(ms.bill.SupportedNominals())
	ms.billCredit.SetValid(ms.bill.SupportedNominals())
	ms.coinCashbox.SetValid(ms.coin.SupportedNominals())
	ms.coinCredit.SetValid(ms.coin.SupportedNominals())

	g.Engine.RegisterNewFunc(
		"money.cashbox_zero",
		func(ctx context.Context) error {
			ms.lk.Lock()
			defer ms.lk.Unlock()
			ms.billCashbox.Clear()
			ms.coinCashbox.Clear()
			return nil
		},
	)
	g.Engine.RegisterNewFunc(
		"money.consume!",
		func(ctx context.Context) error {
			credit := ms.GetCredit()
			err := ms.WithdrawCommit(ctx, credit)
			return oerr.Annotatef(err, "consume=%s", credit.FormatCtx(ctx))
		},
	)
	g.Engine.RegisterNewFunc(
		"bill.stop",
		func(ctx context.Context) error {
			ms.bill.SendCommand(bill.Stop)
			return nil
		},
	)
	g.Engine.RegisterNewFunc(
		"bill.reject",
		func(ctx context.Context) error {
			ms.bill.SendCommand(bill.Reject)
			return nil
		},
	)
	g.Engine.RegisterNewFunc(
		"bill.accept",
		func(ctx context.Context) error {
			ms.bill.SendCommand(bill.Accept)
			return nil
		},
	)

	g.Engine.RegisterNewFunc(
		"money.commit",
		func(ctx context.Context) error {
			curPrice := GetCurrentPrice(ctx)
			err := ms.WithdrawCommit(ctx, curPrice)
			return oerr.Annotatef(err, "curPrice=%s", curPrice.FormatCtx(ctx))
		},
	)
	// g.Engine.RegisterNewFunc("money.abort", ms.ReturnMoney)
	g.Engine.RegisterNewFunc("money.abort",
		func(ctx context.Context) error {
			return ms.ReturnMoney()
		})

	doAccept := engine.FuncArg{
		Name: "money.accept(?)",
		F: func(ctx context.Context, arg engine.Arg) error {
			alive := alive.NewAlive()
			alive.Add(2)
			ch := make(chan types.Event)
			ms.AcceptCredit(ctx, g.Config.ScaleU(uint32(arg)), alive, ch)
			time.Sleep(10 * time.Second)
			alive.Stop()
			alive.Wait()
			ms.coin.TubeStatus()
			return nil
		},
	}
	g.Engine.Register(doAccept.Name, doAccept)

	g.Engine.RegisterNewFuncAgr("money.dispense(?)", func(ctx context.Context, arg engine.Arg) error {
		return ms.coin.Dispense(g.Config.ScaleU(uint32(arg)))
	})

	g.Engine.RegisterNewFuncAgr("money.return(?)", func(ctx context.Context, arg engine.Arg) error {
		return ms.coin.ReturnMoney(g.Config.ScaleU(uint32(arg)))
	})

	doSetGiftCredit := engine.FuncArg{
		Name: "money.set_gift_credit(?)",
		F: func(ctx context.Context, arg engine.Arg) error {
			amount := g.Config.ScaleU(uint32(arg))
			ms.SetGiftCredit(ctx, amount)
			return nil
		},
	}
	g.Engine.Register(doSetGiftCredit.Name, doSetGiftCredit)

	return nil
}

func (ms *MoneySystem) Stop(ctx context.Context) error {
	const tag = "money.Stop"
	g := state.GetGlobal(ctx)
	errs := make([]error, 0, 8)
	errs = append(errs, ms.ReturnMoney())
	// errs = append(errs, g.Engine.Exec(ctx, ms.bill.AcceptMax(0)))
	errs = append(errs, g.Engine.Exec(ctx, ms.coin.AcceptMax(0)))
	return oerr.Annotate(helpers.FoldErrors(errs), tag)
}

// TeleCashbox Stored in one-way cashbox Telemetry_Money
func (ms *MoneySystem) TeleCashbox(ctx context.Context) *tele_api.Telemetry_Money {
	pb := &tele_api.Telemetry_Money{
		Bills: make(map[uint32]uint32, 16),
		Coins: make(map[uint32]uint32, coin.TypeCount),
	}
	ms.lk.Lock()
	defer ms.lk.Unlock()
	ms.billCashbox.ToMapUint32(pb.Bills)
	ms.coinCashbox.ToMapUint32(pb.Coins)
	// ms.Log.Debugf("TeleCashbox pb=%s", proto.CompactTextString(pb))
	ms.Log.Debugf("TeleCashbox pb=%v", pb)
	return pb
}

// TeleChange Dispensable Telemetry_Money
func (ms *MoneySystem) TeleChange(ctx context.Context) *tele_api.Telemetry_Money {
	pb := &tele_api.Telemetry_Money{
		// TODO support bill recycler Bills: make(map[uint32]uint32, bill.TypeCount),
		Coins: make(map[uint32]uint32, coin.TypeCount),
	}
	if err := ms.coin.TubeStatus(); err != nil {
		state.GetGlobal(ctx).Error(oerr.Annotate(err, "TeleChange"))
	}
	ms.coin.Tubes().ToMapUint32(pb.Coins)
	// ms.Log.Debugf("TeleChange pb=%s", proto.CompactTextString(pb))
	ms.Log.Debugf("TeleChange pb=%v", pb)
	return pb
}

type key string

const (
	currentPriceKey key = "run/current-price"
)

func GetCurrentPrice(ctx context.Context) currency.Amount {
	v := ctx.Value(currentPriceKey)
	if v == nil {
		state.GetGlobal(ctx).Error(fmt.Errorf("code/config error money.GetCurrentPrice not set"))
		return 0
	}
	if p, ok := v.(currency.Amount); ok {
		return p
	}
	panic(fmt.Sprintf("code error ctx[currentPriceKey] expected=currency.Amount actual=%#v", v))
}

func SetCurrentPrice(ctx context.Context, p currency.Amount) context.Context {
	ck := context.WithValue(ctx, currentPriceKey, p)
	return ck
}

func (ms *MoneySystem) XXX_InjectCoin(n currency.Nominal) error {
	ms.lk.Lock()
	defer ms.lk.Unlock()
	ms.Log.Debugf("XXX_InjectCoin n=%d", n)
	ms.coinCredit.MustAdd(n, 1)
	ms.dirty += currency.Amount(n)
	return nil
}

func (ms *MoneySystem) AddDirty(dirty currency.Amount) {
	ms.dirty += dirty
	types.VMC.MonSys.Dirty = ms.dirty
}

func (ms *MoneySystem) SetDirty(dirty currency.Amount) {
	ms.dirty = dirty
	types.VMC.MonSys.Dirty = ms.dirty
}

func (ms *MoneySystem) GetDirty() currency.Amount {
	return ms.dirty
}

func (ms *MoneySystem) ResetMoney() {
	ms.locked_zero()
}
