package inventory

import (
	"context"
	"fmt"
	"math"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
)

// const tuneKeyFormat = "run/inventory-%s-tune"

type Stock struct {
	Log            *log2.Log
	ErrorSend      bool
	Label          string `hcl:",label"`
	Code           int    `hcl:"code,optional"`
	XXX_Ingredient string `hcl:"ingredient"`
	RegisterAdd    string `hcl:"register_add,optional"`
	Ingredient     *Ingredient
	value          float32
}

func (s *Stock) String() string {
	return fmt.Sprintf("inventory.%s #%d spend_rate=%f min=%d",
		s.Label, s.Code, s.Ingredient.SpendRate, s.Ingredient.Min)
}

func (s *Stock) GetSpendRate() float32 { return s.Ingredient.SpendRate }

func (s *Stock) SpendValue(value byte) {
	if s.Ingredient.SpendRate == 0 {
		return
	}
	s.value -= float32(value) / s.Ingredient.SpendRate
}

func (s *Stock) ShowLevel() string {
	currenValue := int(s.value) * 100
	valuePerDelay, i := s.valuePerDelay(currenValue, false)
	if valuePerDelay == 0 {
		return "0"
	}
	ost := currenValue - s.Ingredient.levelValue[i].val
	valOst := float64(s.Ingredient.levelValue[i].lev)/100 + math.Round(float64(ost/valuePerDelay))/100
	return fmt.Sprintf("%.2f", valOst)
}

func (s *Stock) SetLevel(level int) {
	valuePerDelay, i := s.valuePerDelay(level, true)
	ost := level - s.Ingredient.levelValue[i].lev
	l1 := s.Ingredient.levelValue[i].val
	l2 := ost * valuePerDelay
	s.value = float32((l1 + l2) / 100)
}

// returns the number per 0.01 division and the index of the smaller value
func (s *Stock) valuePerDelay(value int, valueIsLevel bool) (valuePerDelay int, index int) {
	countLevels := len(s.Ingredient.levelValue) - 1
	for index = countLevels; index >= 0; index-- {
		var v int
		if valueIsLevel {
			v = s.Ingredient.levelValue[index].lev
		} else {
			v = s.Ingredient.levelValue[index].val
		}
		if v < value {
			switch {
			case countLevels == index && index == 0: // levels not sets
				return 0, 0
			case countLevels == index && index > 0: // level > max rate
				valuePerDelay = (s.Ingredient.levelValue[index].val - s.Ingredient.levelValue[index-1].val) / (s.Ingredient.levelValue[index].lev - s.Ingredient.levelValue[index-1].lev)
			default: // level between
				valuePerDelay = (s.Ingredient.levelValue[index+1].val - s.Ingredient.levelValue[index].val) / (s.Ingredient.levelValue[index+1].lev - s.Ingredient.levelValue[index].lev)
			}
			return valuePerDelay, index
		}
	}
	return 0, 0
}

func (s *Stock) Value() float32 { return s.value }

func (s *Stock) Set(v float32) { s.value = v }

func (s *Stock) Has(v float32) bool {
	if s.Ingredient.Min == 0 {
		return true
	}
	return s.value-v >= float32(s.Ingredient.Min)
}

func (s *Stock) Wrap(d engine.Doer) engine.Doer {
	return &custom{stock: s, before: d}
}

func (s *Stock) TranslateSpend(arg engine.Arg) float32 {
	return translate(int32(arg.(int16)), s.Ingredient.SpendRate)
}

// signature match engine.FuncArg.F
func (s *Stock) spendArg(ctx context.Context, arg engine.Arg) error {
	s.spendValue(s.TranslateSpend(arg.(int16)))
	return nil
}

func (s *Stock) spendValue(v float32) {
	s.value -= v
}

type custom struct {
	stock  *Stock
	before engine.Doer
	after  engine.Doer
	arg    engine.Arg
	spend  float32
}

func (c *custom) Apply(arg engine.Arg) (engine.Doer, bool, error) {
	if c.after != nil {
		err := engine.ErrArgOverwrite
		return nil, false, errors.Annotatef(err, engine.FmtErrContext, c.stock.String())
	}
	return c.apply(arg)
}

func (c *custom) Calculation() float64 {
	return c.stock.Ingredient.Cost * float64(c.arg.(int16)) * float64(c.stock.Ingredient.SpendRate)
}

func (c *custom) Validate() error {
	if err := c.after.Validate(); err != nil {
		return errors.Annotatef(err, "stock=%s", c.stock.Ingredient.Name)
	}
	if c.stock.Ingredient.SpendRate == 0 {
		return nil
	}
	if c.stock.Has(c.spend) {
		return nil
	}
	if !c.stock.ErrorSend {
		c.stock.Log.Errorf("low-%s", c.stock.Ingredient.Name)
		c.stock.ErrorSend = true
	}
	return errors.Errorf("%s low", c.stock.Ingredient.Name)
}

func (c *custom) Do(ctx context.Context) error {
	e := engine.GetGlobal(ctx)
	if tunedCtx, tuneRate, ok := takeTuneRate(ctx, c.stock.Ingredient.TuneKey); ok {
		tunedArg := engine.Arg(int16(math.Round(float64(c.arg.(int16)) * float64(tuneRate))))
		d, _, err := c.apply(tunedArg)
		// log.Printf("stock=%s before=%#v arg=%v tuneRate=%v tunedArg=%v d=%v err=%v", c.stock.String(), c.before, c.arg, tuneRate, tunedArg, d, err)
		if err != nil {
			return errors.Annotatef(err, "stock=%s tunedArg=%v", c.stock.Label, tunedArg)
		}
		return e.Exec(tunedCtx, d)
	}

	// log.Printf("stock=%s value=%f arg=%v spending=%f", c.stock.Name, c.stock.Value(), c.arg, c.spend)
	// TODO remove this redundant check when sure that Validate() is called in all proper places
	if c.stock.Ingredient.Min != 0 && !c.stock.Has(c.spend) {
		return errors.Errorf("stock=%s check fail", c.stock.Label)
	}

	if err := c.after.Validate(); err != nil {
		return errors.Annotatef(err, "stock=%s", c.stock.Label)
	}
	err := e.Exec(ctx, c.after)
	if err != nil {
		return err
	}
	c.stock.spendValue(c.spend)
	return nil
}

func (c *custom) String() string { return fmt.Sprintf("stock.%s(%d)", c.stock.Label, c.arg) }

func (c *custom) apply(arg engine.Arg) (engine.Doer, bool, error) {
	after, applied, err := engine.ArgApply(c.before, arg)
	if err != nil {
		return nil, false, errors.Annotatef(err, engine.FmtErrContext, c.stock.String())
	}
	if !applied {
		err = engine.ErrArgNotApplied
		return nil, false, errors.Annotatef(err, engine.FmtErrContext, c.stock.String())
	}
	new := &custom{
		stock:  c.stock,
		before: c.before,
		after:  after,
		arg:    arg,
		spend:  c.stock.TranslateSpend(arg),
	}
	return new, true, nil
}

func takeTuneRate(ctx context.Context, key string) (context.Context, float32, bool) {
	tk := CTXkey(key)
	v := ctx.Value(tk)
	if v == nil { // either no tuning or masked to avoid Do() recursion
		return ctx, 0, false
	}
	if tuneRate, ok := v.(float32); ok { // tuning found for the first time
		ctx = context.WithValue(ctx, tk, nil)
		return ctx, tuneRate, true
	}
	panic(fmt.Sprintf("code error takeTuneRate(key=%s) found invalid value=%#v", tk, v))
}

func translate(arg int32, rate float32) float32 {
	if arg == 0 {
		return 0
	}
	// result := float32(math.Round(float64(arg) * float64(rate)))
	result := float32(float64(arg) * float64(rate))
	if result == 0 {
		return 1
	}
	return result
}
