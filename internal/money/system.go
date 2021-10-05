package money

import (
	"context"
	"fmt"
	"sync"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/mdb/bill"
	"github.com/AlexTransit/vender/hardware/mdb/coin"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/log2"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/golang/protobuf/proto"
	"github.com/juju/errors"
)

type MoneySystem struct { //nolint:maligned
	Log   *log2.Log
	lk    sync.RWMutex
	dirty currency.Amount // uncommited

	bill        bill.Biller
	billCashbox currency.NominalGroup
	billCredit  currency.NominalGroup

	coin        coin.Coiner
	coinCashbox currency.NominalGroup
	coinCredit  currency.NominalGroup

	giftCredit currency.Amount
}

func GetGlobal(ctx context.Context) *MoneySystem {
	return state.GetGlobal(ctx).XXX_money.Load().(*MoneySystem)
}

func (ms *MoneySystem) AddDirty(dirty currency.Amount) {
	ms.dirty += dirty
}

func (ms *MoneySystem) SetDirty(dirty currency.Amount) {
	ms.dirty = dirty
}

func (ms *MoneySystem) GetDirty() currency.Amount {
	return ms.dirty
}

func (ms *MoneySystem) ResetMoney() {
	ms.locked_zero()
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
	errs := make([]error, 0, 2)
	if dev, err := g.GetDevice(devNameBill); err == nil {
		ms.bill = dev.(bill.Biller)
	} else if errors.IsNotFound(err) {
		ms.Log.Debugf("device=%s is not enabled in config", devNameBill)
	} else {
		errs = append(errs, errors.Annotatef(err, "device=%s", devNameBill))
	}
	if dev, err := g.GetDevice(devNameCoin); err == nil {
		ms.coin = dev.(coin.Coiner)
	} else if errors.IsNotFound(err) {
		ms.Log.Debugf("device=%s is not enabled in config", devNameCoin)
	} else {
		errs = append(errs, errors.Annotatef(err, "device=%s", devNameCoin))
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
			credit := ms.Credit(ctx)
			err := ms.WithdrawCommit(ctx, credit)
			return errors.Annotatef(err, "consume=%s", credit.FormatCtx(ctx))
		},
	)
	g.Engine.RegisterNewFunc(
		"money.commit",
		func(ctx context.Context) error {
			curPrice := GetCurrentPrice(ctx)
			err := ms.WithdrawCommit(ctx, curPrice)
			return errors.Annotatef(err, "curPrice=%s", curPrice.FormatCtx(ctx))
		},
	)
	g.Engine.RegisterNewFunc("money.abort", ms.Abort)

	doAccept := engine.FuncArg{
		Name: "money.accept(?)",
		F: func(ctx context.Context, arg engine.Arg) error {
			ms.AcceptCredit(ctx, g.Config.ScaleU(uint32(arg)), nil, nil)
			return nil
		},
	}
	g.Engine.Register(doAccept.Name, doAccept)

	doGive := engine.FuncArg{
		Name: "money.give(?)",
		F: func(ctx context.Context, arg engine.Arg) error {
			dispensed := currency.NominalGroup{}
			d := ms.coin.NewGive(g.Config.ScaleU(uint32(arg)), false, &dispensed)
			err := g.Engine.Exec(ctx, d)
			ms.Log.Infof("dispensed=%s", dispensed.String())
			return err
		}}
	g.Engine.Register(doGive.Name, doGive)
	g.Engine.Register("money.dispense(?)", doGive) // FIXME remove deprecated

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
	errs = append(errs, ms.Abort(ctx))
	errs = append(errs, g.Engine.Exec(ctx, ms.bill.AcceptMax(0)))
	errs = append(errs, g.Engine.Exec(ctx, ms.coin.AcceptMax(0)))
	return errors.Annotate(helpers.FoldErrors(errs), tag)
}

// TeleCashbox Stored in one-way cashbox Telemetry_Money
func (ms *MoneySystem) TeleCashbox(ctx context.Context) *tele_api.Telemetry_Money {
	pb := &tele_api.Telemetry_Money{
		Bills: make(map[uint32]uint32, bill.TypeCount),
		Coins: make(map[uint32]uint32, coin.TypeCount),
	}
	ms.lk.Lock()
	defer ms.lk.Unlock()
	ms.billCashbox.ToMapUint32(pb.Bills)
	ms.coinCashbox.ToMapUint32(pb.Coins)
	ms.Log.Debugf("TeleCashbox pb=%s", proto.CompactTextString(pb))
	return pb
}

// TeleChange Dispensable Telemetry_Money
func (ms *MoneySystem) TeleChange(ctx context.Context) *tele_api.Telemetry_Money {
	pb := &tele_api.Telemetry_Money{
		// TODO support bill recycler Bills: make(map[uint32]uint32, bill.TypeCount),
		Coins: make(map[uint32]uint32, coin.TypeCount),
	}
	if err := ms.coin.TubeStatus(); err != nil {
		state.GetGlobal(ctx).Error(errors.Annotate(err, "TeleChange"))
	}
	ms.coin.Tubes().ToMapUint32(pb.Coins)
	ms.Log.Debugf("TeleChange pb=%s", proto.CompactTextString(pb))
	return pb
}

const currentPriceKey = "run/current-price"

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
	return context.WithValue(ctx, currentPriceKey, p)
}

func (ms *MoneySystem) XXX_InjectCoin(n currency.Nominal) error {
	ms.lk.Lock()
	defer ms.lk.Unlock()
	ms.Log.Debugf("XXX_InjectCoin n=%d", n)
	ms.coinCredit.MustAdd(n, 1)
	ms.dirty += currency.Amount(n)
	return nil
}
