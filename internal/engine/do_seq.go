package engine

import (
	"context"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/juju/errors"
)

const seqBuffer uint = 8

// Sequence executor. Specialized version of Tree for performance.
// Error in one action aborts whole group.
// Build graph with NewSeq().Append()
type Seq struct {
	name  string
	_b    [seqBuffer]Doer
	items []Doer
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

func (seq *Seq) Validate() error {
	errs := make([]error, 0, len(seq.items))

	for _, d := range seq.items {
		// log.Printf("Seq.Validate d=%#v", d)
		if err := d.Validate(); err != nil {
			err = errors.Annotatef(err, "seq=%s node=%s validate", seq.String(), d.String())
			errs = append(errs, err)
		}
	}

	return helpers.FoldErrors(errs)
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
			helpers.SaveAndShowDoError(itemsList, err)
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
