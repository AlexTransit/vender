package inventory

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/juju/errors"
)

// const tuneKeyFormat = "run/inventory-%s-tune"

type Stock struct {
	Name        string  `hcl:",label"`
	Code        int     `hcl:"code"`
	Check       bool    `hcl:"check,optional"`
	Min         float32 `hcl:"min,optional"`
	SpendRate   float32 `hcl:"spend_rate,optional"`
	RegisterAdd string  `hcl:"register_add,optional"`
	Level       string  `hcl:"level,optional"`
	TuneKey     string
	value       float32
	levelValue  []struct { // used fixed comma x.xx
		lev int
		val int
	}
}

func (s *Stock) String() string {
	return fmt.Sprintf("inventory.%s #%d check=%t spend_rate=%f min=%f",
		s.Name, s.Code, s.Check, s.SpendRate, s.Min)
}

// type Stock struct { //nolint:maligned
//
//		Code uint32
//		Name string
//		// enabled   uint32 // atomic
//		enabled   bool
//		check     bool
//		TeleLow   bool
//		spendRate float32
//		min       float32
//		value     float32
//		tuneKey   string
//		level     []struct { // used fixed comma x.xx
//			lev int
//			val int
//		}
//	}

// func NewStock(c inventory_config.Stock, e *engine.Engine) (*Stock, error) {
// 	if c.Name == "" {
// 		return nil, errors.Errorf("stock=(empty) is invalid")
// 	}
// 	if c.SpendRate == 0 {
// 		c.SpendRate = 1
// 	}
// 	tk := fmt.Sprintf(tuneKeyFormat, c.Name)
// 	if c.TuneKey != "" {
// 		tk = fmt.Sprintf(tuneKeyFormat, c.TuneKey)
// 	}
// 	s := &Stock{
// 		Name:      c.Name,
// 		Code:      uint32(c.Code),
// 		check:     c.Check,
// 		enabled:   true,
// 		spendRate: c.SpendRate,
// 		min:       c.Min,
// 		tuneKey:   tk,
// 	}
// 	s.fillLevels(&c)
// 	doSpend1 := engine.Func0{
// 		Name: fmt.Sprintf("stock.%s.spend1", s.Name),
// 		F:    s.spend1,
// 	}
// 	doSpendArg := engine.FuncArg{
// 		Name: fmt.Sprintf("stock.%s.spend(?)", s.Name),
// 		F:    s.spendArg,
// 	}
// 	addName := fmt.Sprintf("add.%s(?)", s.Name)
// 	if c.RegisterAdd != "" {
// 		doAdd, err := e.ParseText(addName, c.RegisterAdd)
// 		if err != nil {
// 			return nil, errors.Annotatef(err, "stock=%s register_add", s.Name)
// 		}
// 		_, ok, err := engine.ArgApply(doAdd, 0)
// 		switch {
// 		case err == nil && !ok:
// 			return nil, errors.Errorf("stock=%s register_add=%s no free argument", s.Name, c.RegisterAdd)

// 		case (err == nil && ok) || engine.IsNotResolved(err): // success path
// 			e.Register(addName, s.Wrap(doAdd))

// 		case err != nil:
// 			return nil, errors.Annotatef(err, "stock=%s register_add=%s", s.Name, c.RegisterAdd)
// 		}
// 	}
// 	e.Register(doSpend1.Name, doSpend1)
// 	e.Register(doSpendArg.Name, doSpendArg)
// 	return s, nil
// }

func (s *Stock) GetSpendRate() float32 { return s.SpendRate }

func (s *Stock) SpendValue(value byte) {
	if !s.Check {
		return
	}
	s.value -= float32(value) / s.SpendRate
}

func (s *Stock) ShowLevel() string {
	currenValue := int(s.value) * 100
	valuePerDelay, i := s.valuePerDelay(currenValue, false)
	if valuePerDelay == 0 {
		return "0"
	}
	ost := currenValue - s.levelValue[i].val
	valOst := float64(s.levelValue[i].lev)/100 + math.Round(float64(ost/valuePerDelay))/100
	return fmt.Sprintf("%.2f", valOst)
}

func (s *Stock) SetLevel(level int) {
	valuePerDelay, i := s.valuePerDelay(level, true)
	ost := level - s.levelValue[i].lev
	l1 := s.levelValue[i].val
	l2 := ost * valuePerDelay
	s.value = float32((l1 + l2) / 100)
}

// returns the number per 0.01 division and the index of the smaller value
func (s *Stock) valuePerDelay(value int, valueIsLevel bool) (valuePerDelay int, index int) {
	countLevels := len(s.levelValue) - 1
	for index = countLevels; index >= 0; index-- {
		var v int
		if valueIsLevel {
			v = s.levelValue[index].lev
		} else {
			v = s.levelValue[index].val
		}
		if v < value {
			switch {
			case countLevels == index && index == 0: // levels not sets
				return 0, 0
			case countLevels == index && index > 0: // level > max rate
				valuePerDelay = (s.levelValue[index].val - s.levelValue[index-1].val) / (s.levelValue[index].lev - s.levelValue[index-1].lev)
			default: // level between
				valuePerDelay = (s.levelValue[index+1].val - s.levelValue[index].val) / (s.levelValue[index+1].lev - s.levelValue[index].lev)
			}
			return valuePerDelay, index
		}
	}
	return 0, 0
}

// func (s *Stock) fillLevels(c *inventory_config.Stock) {
// 	rm := `([0-9]*[.,]?[0-9]+)\(([0-9]*[.,]?[0-9]+)\)`
// 	parts := regexp.MustCompile(rm).FindAllStringSubmatch(c.Level, 50)
// 	s.levelValue = make([]struct {
// 		lev int
// 		val int
// 	}, len(parts)+1)

// 	if len(parts) == 0 {
// 		return
// 	}

//		for i, v := range parts {
//			s.levelValue[i+1].lev = stringToFixInt(v[1])
//			s.levelValue[i+1].val = stringToFixInt(v[2])
//		}
//		sort.Slice(s.levelValue, func(i, j int) bool {
//			return s.levelValue[i].lev < s.levelValue[j].lev
//		})
//	}
const RegexLevels = `([0-9]*[.,]?[0-9]+)\(([0-9]*[.,]?[0-9]+)\)`

func (s *Stock) fillLevels() {
	parts := regexp.MustCompile(RegexLevels).FindAllStringSubmatch(s.Level, 50)
	s.levelValue = make([]struct {
		lev int
		val int
	}, len(parts)+1)

	if len(parts) == 0 {
		return
	}

	for i, v := range parts {
		s.levelValue[i+1].lev = stringToFixInt(v[1])
		s.levelValue[i+1].val = stringToFixInt(v[2])
	}
	sort.Slice(s.levelValue, func(i, j int) bool {
		return s.levelValue[i].lev < s.levelValue[j].lev
	})
}

func stringToFixInt(s string) int {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return int(v * 100)
	}
	return 0
}

// func (s *Stock) Enabled() bool { return s.enabled }

func (s *Stock) Value() float32 { return s.value }

// func (s *Stock) Set(new float32)    { s.value.Store(new) }
func (s *Stock) Set(v float32)      { s.value = v }
func (s *Stock) Has(v float32) bool { return s.value-v >= s.Min }

// func (s *Stock) String() string {
// 	return fmt.Sprintf("source(name=%s value=%f)", s.Name, s.Value())
// }

func (s *Stock) Wrap(d engine.Doer) engine.Doer {
	return &custom{stock: s, before: d}
}

func (s *Stock) TranslateSpend(arg engine.Arg) float32 {
	return translate(int32(arg.(int16)), s.SpendRate)
}

// signature match engine.Func0.F
func (s *Stock) spend1() error {
	s.spendValue(s.TranslateSpend(1))
	return nil
}

// signature match engine.FuncArg.F
func (s *Stock) spendArg(ctx context.Context, arg engine.Arg) error {
	s.spendValue(s.TranslateSpend(arg.(int16)))
	return nil
}

func (s *Stock) spendValue(v float32) {
	if s.Check {
		s.value -= v
	}
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

func (c *custom) Validate() error {
	if err := c.after.Validate(); err != nil {
		return errors.Annotatef(err, "stock=%s", c.stock.Name)
	}
	// if !c.stock.Enabled() {
	// 	return nil
	// }
	if !c.stock.Check {
		return nil
	}
	if c.stock.Has(c.spend) {
		return nil
	}
	// if !c.stock.TeleLow {
	// 	types.TeleError(c.stock.Name + " - low")
	// 	c.stock.TeleLow = true
	// }
	return errors.Errorf("%s low", c.stock.Name)
}

func (c *custom) Do(ctx context.Context) error {
	e := engine.GetGlobal(ctx)
	if tunedCtx, tuneRate, ok := takeTuneRate(ctx, c.stock.TuneKey); ok {
		tunedArg := engine.Arg(int16(math.Round(float64(c.arg.(int16)) * float64(tuneRate))))
		d, _, err := c.apply(tunedArg)
		// log.Printf("stock=%s before=%#v arg=%v tuneRate=%v tunedArg=%v d=%v err=%v", c.stock.String(), c.before, c.arg, tuneRate, tunedArg, d, err)
		if err != nil {
			return errors.Annotatef(err, "stock=%s tunedArg=%v", c.stock.Name, tunedArg)
		}
		return e.Exec(tunedCtx, d)
	}

	// log.Printf("stock=%s value=%f arg=%v spending=%f", c.stock.Name, c.stock.Value(), c.arg, c.spend)
	// TODO remove this redundant check when sure that Validate() is called in all proper places
	if c.stock.Check && !c.stock.Has(c.spend) {
		return errors.Errorf("stock=%s check fail", c.stock.Name)
	}

	if err := c.after.Validate(); err != nil {
		return errors.Annotatef(err, "stock=%s", c.stock.Name)
	}
	err := e.Exec(ctx, c.after)
	if err != nil {
		return err
	}
	c.stock.spendValue(c.spend)
	return nil
}

func (c *custom) String() string { return fmt.Sprintf("stock.%s(%d)", c.stock.Name, c.arg) }

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
	v := ctx.Value(key)
	if v == nil { // either no tuning or masked to avoid Do() recursion
		return ctx, 0, false
	}
	if tuneRate, ok := v.(float32); ok { // tuning found for the first time
		ctx = context.WithValue(ctx, key, nil)
		return ctx, tuneRate, true
	}
	panic(fmt.Sprintf("code error takeTuneRate(key=%s) found invalid value=%#v", key, v))
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

// func (s *Stock) overwrite(v *[]inventory.Stock) {
// 	for _, v := range v {
// 	}
// }
