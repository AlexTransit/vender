package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
)

const FmtErrContext = "`%s`" // errors.Annotatef(err, FmtErrContext, doer.String())

type Doer interface {
	Validate() error
	Calculation() float64
	Do(context.Context) error
	String() string // for logs
}

type Nothing struct{ Name string }

func (n Nothing) Do(ctx context.Context) error { return nil }
func (n Nothing) Validate() error              { return nil }
func (n Nothing) Calculation() float64         { return 0 }
func (n Nothing) String() string               { return n.Name }

type Func struct {
	Name string
	F    func(context.Context) error
	V    ValidateFunc
	C    CalculationFunc
}

func (f Func) Validate() error              { return useValidator(f.V) }
func (f Func) Calculation() float64         { return useCalculation(f.C) }
func (f Func) Do(ctx context.Context) error { return f.F(ctx) }
func (f Func) String() string               { return f.Name }

type Func0 struct {
	Name string
	F    func() error
	V    ValidateFunc
	C    CalculationFunc
}

func (f Func0) Validate() error              { return useValidator(f.V) }
func (f Func0) Calculation() float64         { return useCalculation(f.C) }
func (f Func0) Do(ctx context.Context) error { return f.F() }
func (f Func0) String() string               { return f.Name }

type Sleep struct{ time.Duration }

func (s Sleep) Validate() error              { return nil }
func (s Sleep) Calculation() float64         { return 0 }
func (s Sleep) Do(ctx context.Context) error { time.Sleep(s.Duration); return nil }
func (s Sleep) String() string               { return fmt.Sprintf("Sleep(%v)", s.Duration) }

type RepeatN struct {
	D Doer
	N uint
}

func (r RepeatN) Validate() error      { return r.D.Validate() }
func (r RepeatN) Calculation() float64 { return r.D.Calculation() }
func (r RepeatN) Do(ctx context.Context) error {
	// FIXME solve import cycle, use GetGlobal(ctx).Log
	log := log2.ContextValueLogger(ctx)
	var err error
	for i := uint(1); i <= r.N && err == nil; i++ {
		log.Debugf("engine loop %d/%d", i, r.N)
		err = GetGlobal(ctx).ExecPart(ctx, r.D)
	}
	return err
}

func (r RepeatN) String() string {
	return fmt.Sprintf("RepeatN(N=%d D=%s)", r.N, r.D.String())
}

type (
	ValidateFunc    func() error
	CalculationFunc func() float64
)

func useValidator(v ValidateFunc) error {
	if v == nil {
		return nil
	}
	return v()
}

func useCalculation(c CalculationFunc) float64 {
	if c == nil {
		return 0
	}
	return c()
}

type Fail struct{ E error }

func (f Fail) Validate() error              { return f.E }
func (f Fail) Calculation() float64         { return 0 }
func (f Fail) Do(ctx context.Context) error { return f.E }
func (f Fail) String() string               { return f.E.Error() }

type RestartError struct {
	Doer
	Reset Doer
	Check func(error) bool
}

func (re *RestartError) Validate() error      { return re.Doer.Validate() }
func (re *RestartError) Calculation() float64 { return re.Doer.Calculation() }
func (re *RestartError) Do(ctx context.Context) error {
	first := GetGlobal(ctx).ExecPart(ctx, re.Doer)
	if first != nil {
		if re.Check(first) {
			resetErr := GetGlobal(ctx).ExecPart(ctx, re.Reset)
			if resetErr != nil {
				return errors.Wrap(first, resetErr)
			}
			return GetGlobal(ctx).ExecPart(ctx, re.Doer)
		}
	}
	return first
}
func (re *RestartError) String() string { return re.Doer.String() }
