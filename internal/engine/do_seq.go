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
	_b    [seqBuffer]Doer
	items []Doer
	name  string
}

func NewSeq(name string) *Seq {
	seq := &Seq{name: name}
	seq.items = seq._b[:0]
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
			// FIXME AlexM
			helpers.SaveAndShowDoError(itemsList, err, "/home/vmc/vender-db/errors/")
			return err
		}
	}
	//	helpers.SaveAndShowDoError(itemsList, nil)
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
	copy(seq._b[:], zeroBuffer[:])
	seq.items = ds
}
