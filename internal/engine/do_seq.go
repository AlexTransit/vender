package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/helpers"
)

const seqBuffer uint = 8

// Sequence executor. Specialized version of Tree for performance.
// Error in one action aborts whole group.
// Build graph with NewSeq().Append()
type Seq struct {
	name string
	// _b    [seqBuffer]Doer
	items        []Doer
	ErrorActions map[int32]Doer
}

func (seq *Seq) AddErrorAction(code int32, d Doer) {
	if seq.ErrorActions == nil {
		seq.ErrorActions = make(map[int32]Doer)
	}
	seq.ErrorActions[code] = d
}

func NewSeq(name string, items ...int) *Seq {
	seq := &Seq{name: name}
	if len(items) > 0 {
		seq.items = make([]Doer, 0, items[0])
	} else {
		seq.items = make([]Doer, 0, seqBuffer)
	}
	return seq
}

func (seq *Seq) Append(d Doer) *Seq {
	seq.items = append(seq.items, d)
	return seq
}

func (seq *Seq) Validate() (err error) {
	for _, d := range seq.items {
		if e := d.Validate(); e != nil {
			err = errors.Join(err, fmt.Errorf("seq=%s node=%s validate (%v)", seq.String(), d.String(), e))
		}
	}
	return err
}

func (seq *Seq) Calculation() (summ float64) {
	for _, d := range seq.items {
		if v := d.Calculation(); v != 0 {
			summ += v
		}
	}
	return summ
}

func (seq *Seq) Do(ctx context.Context) error {
	e := GetGlobal(ctx)
	var itemsList []string
	itemsList = append(itemsList, time.Now().Format("2006-01-02_15-04-05.00000"))
	itemsList = append(itemsList, seq.name)
	for _, d := range seq.items {
		itemsList = append(itemsList, time.Now().Format("-> 15:04:05.00000 ")+d.String())
		err := e.Exec(ctx, d)
		itemsList = append(itemsList, time.Now().Format("<- 15:04:05.00000 ")+d.String())
		if err != nil {
			var appErr *helpers.AppError
			if !errors.As(err, &appErr) {
				return err
			}
			ok, ErrorD := e.CheckAction(fmt.Sprintf("%s-Err:%d", d.String(), appErr.Code()))
			if !ok {
				return err
			}
			err1 := e.Exec(ctx, ErrorD)
			if err1 == nil {
				e.Log.Infof("error action fix problem. (%v)", err)
				continue
			}
			// FIXME AlexM
			helpers.SaveAndShowDoError(itemsList, err, "/home/vmc/vender-db/errors/")
			return err
		}
	}
	return nil
}

func (seq *Seq) String() string {
	return seq.name
}

func (seq *Seq) cloneEmpty() *Seq {
	new := NewSeq(seq.name)
	if n := len(seq.items); n > cap(new.items) {
		new.items = make([]Doer, 0, len(seq.items))
	}
	return new
}

func (seq *Seq) setItems(ds []Doer) {
	var zeroBuffer [seqBuffer]Doer
	copy(seq.items[:], zeroBuffer[:])
	seq.items = ds
}
