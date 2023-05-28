package currency

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"

	oerr "github.com/juju/errors"
)

// Amount is integer counting lowest currency unit, e.g. $1.20 = 120
type Amount uint32

const MaxAmount = Amount(math.MaxUint32)

func (a Amount) Format100I() string { return fmt.Sprint(float32(a) / 100) }
func (a Amount) AddPersent(persent float64) Amount {
	vf := float64(a) * persent
	vi := int32(math.Round(vf))
	return Amount(vi)
}
func (a Amount) FormatCtx(ctx context.Context) string {
	// XXX FIXME
	return a.Format100I()
}

// Nominal is value of one coin or bill
type Nominal Amount

func (n Nominal) Format100I() string { return fmt.Sprint(float32(n) / 100) }

// func (n Nominal) Amount() Amount     { return n.Amount() }

var (
	ErrNominalInvalid = oerr.New("Nominal is not valid for this group")
	ErrNominalCount   = oerr.New("Not enough nominals for this amount")
)

// NominalGroup operates money comprised of multiple nominals, like coins or bills.
// coin1 : 3
// coin5 : 1
// coin10: 4
// total : 48
type NominalGroup struct {
	values map[Nominal]uint
}

func (ng *NominalGroup) Copy() *NominalGroup {
	ng2 := &NominalGroup{
		values: make(map[Nominal]uint, len(ng.values)),
	}
	for k, v := range ng.values {
		ng2.values[k] = v
	}
	return ng2
}

func (ng *NominalGroup) SetValid(valid []Nominal) {
	ng.values = make(map[Nominal]uint, len(valid))
	for _, n := range valid {
		if n != 0 {
			ng.values[n] = 0
		}
	}
}

func (ng *NominalGroup) Add(n Nominal) error {
	if _, ok := ng.values[n]; !ok {
		return oerr.Annotatef(ErrNominalInvalid, "Add(n=%s)", Amount(n).Format100I())
	}
	ng.values[n]++
	return nil
}
func (ng *NominalGroup) Sub(n Nominal) error {
	if _, ok := ng.values[n]; !ok {
		return oerr.Annotatef(ErrNominalInvalid, "Add(n=%s)", Amount(n).Format100I())
	}
	ng.values[n]--
	return nil
}
func (ng *NominalGroup) AddMany(n Nominal, count uint) error {
	if _, ok := ng.values[n]; !ok {
		return oerr.Annotatef(ErrNominalInvalid, "Add(n=%s, c=%d)", Amount(n).Format100I(), count)
	}
	ng.MustAdd(n, count)
	return nil
}

func (ng *NominalGroup) AddFrom(source *NominalGroup) {
	if ng.values == nil {
		ng.values = make(map[Nominal]uint, len(source.values))
	}
	for k, v := range source.values {
		ng.values[k] += v
	}
}

// MustAdd just adds count ignoring valid nominals.
func (ng *NominalGroup) MustAdd(n Nominal, count uint) {
	ng.values[n] += count
}

func (ng *NominalGroup) Clear() {
	for n := range ng.values {
		ng.values[n] = 0
	}
}

func (ng *NominalGroup) Get(n Nominal) (uint, error) {
	if stored, ok := ng.values[n]; !ok {
		return 0, ErrNominalInvalid
	} else {
		return stored, nil
	}
}
func (ng *NominalGroup) InTube(n Nominal) uint {
	if stored, ok := ng.values[n]; !ok {
		return 0
	} else {
		return stored
	}
}

func (ng *NominalGroup) Iter(f func(nominal Nominal, count uint) error) error {
	for nominal, count := range ng.values {
		if err := f(nominal, count); err != nil {
			return err
		}
	}
	return nil
}

func (ng *NominalGroup) ToMapUint32(m map[uint32]uint32) {
	for nominal, count := range ng.values {
		m[uint32(nominal)] = uint32(count)
	}
}

func (ng *NominalGroup) Total() Amount {
	sum := Amount(0)
	for nominal, count := range ng.values {
		sum += Amount(nominal) * Amount(count)
	}
	return sum
}

func (ng *NominalGroup) Diff(other *NominalGroup) Amount {
	result := Amount(0)
	for n, c := range ng.values {
		result += Amount(n)*Amount(c) - Amount(n)*Amount(other.values[n])
	}
	return result
}
func (ng *NominalGroup) SubOther(other *NominalGroup) {
	for nominal := range ng.values {
		ng.values[nominal] -= other.values[nominal]
	}
}

func (ng *NominalGroup) Withdraw(to *NominalGroup, a Amount, strategy ExpendStrategy) error {
	return ng.expendLoop(to, a, strategy)
}

func (ng *NominalGroup) String() string {
	parts := make([]string, 0, len(ng.values)+1)
	sum := Amount(0)
	for nominal, count := range ng.values {
		if count > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", Amount(nominal).Format100I(), count))
			sum += Amount(nominal) * Amount(count)
		}
	}
	sort.Strings(parts)
	parts = append(parts, fmt.Sprintf("total:%s", sum.Format100I()))
	return strings.Join(parts, ",")
}

func (ng *NominalGroup) expendLoop(to *NominalGroup, amount Amount, strategy ExpendStrategy) error {
	strategy.Reset(ng)
	for amount > 0 {
		nominal, err := strategy.ExpendOne(ng, amount)
		if err != nil {
			return err
		}
		if nominal == 0 {
			panic("ExpendStrategy returned Nominal 0 without error")
		}
		amount -= Amount(nominal)
		if to != nil {
			to.values[nominal] += 1
		}
	}
	return nil
}

// common code from strategies
func expendOneOrdered(from *NominalGroup, order []Nominal, max Amount) (Nominal, error) {
	if len(order) < len(from.values) {
		panic("expendOneOrdered order must include all nominals")
	}
	if max == 0 {
		return 0, nil
	}
	for _, n := range order {
		if Amount(n) <= max && from.values[n] > 0 {
			from.values[n] -= 1
			return n, nil
		}
	}
	return 0, ErrNominalCount
}

type ngOrderSortElemFunc func(Nominal, uint) Nominal

func (ng *NominalGroup) order(sortElemFunc ngOrderSortElemFunc) []Nominal {
	order := make([]Nominal, 0, len(ng.values))
	for n := range ng.values {
		order = append(order, n)
	}
	sort.Slice(order, func(i, j int) bool {
		ni, nj := order[i], order[j]
		return sortElemFunc(ni, ng.values[ni]) > sortElemFunc(nj, ng.values[nj])
	})
	return order
}
func ngOrderSortElemNominal(n Nominal, c uint) Nominal { return n }
func ngOrderSortElemCount(n Nominal, c uint) Nominal   { return Nominal(c) }

// NominalGroup.Withdraw = strategy.Reset + loop strategy.ExpendOne
type ExpendStrategy interface {
	Reset(from *NominalGroup)
	ExpendOne(from *NominalGroup, max Amount) (Nominal, error)
	Validate() bool
}

type ExpendGenericOrder struct {
	order        []Nominal
	SortElemFunc ngOrderSortElemFunc
}

func (ego *ExpendGenericOrder) Reset(from *NominalGroup) {
	ego.order = from.order(ego.SortElemFunc)
}
func (ego *ExpendGenericOrder) ExpendOne(from *NominalGroup, max Amount) (Nominal, error) {
	return expendOneOrdered(from, ego.order, max)
}
func (ego *ExpendGenericOrder) Validate() bool { return true }

func NewExpendLeastCount() ExpendStrategy {
	return &ExpendGenericOrder{SortElemFunc: ngOrderSortElemNominal}
}
func NewExpendMostAvailable() ExpendStrategy {
	return &ExpendGenericOrder{SortElemFunc: ngOrderSortElemCount}
}

type ExpendStatistical struct {
	order []Nominal
	Stat  *NominalGroup
}

func (es *ExpendStatistical) Reset(from *NominalGroup) {
	es.order = es.Stat.order(ngOrderSortElemCount)
}
func (es *ExpendStatistical) ExpendOne(from *NominalGroup, max Amount) (Nominal, error) {
	return expendOneOrdered(from, es.order, max)
}
func (es *ExpendStatistical) Validate() bool {
	return es.Stat.Total() > 0
}

type ExpendCombine struct {
	rnd   *rand.Rand
	S1    ExpendStrategy
	S2    ExpendStrategy
	Ratio float32
}

func (ec *ExpendCombine) Reset(from *NominalGroup) {
	ec.rnd = rand.New(rand.NewSource(int64(from.Total())))
	ec.S1.Reset(from)
	ec.S2.Reset(from)
}
func (ec *ExpendCombine) ExpendOne(from *NominalGroup, max Amount) (Nominal, error) {
	if ec.rnd.Float32() < ec.Ratio {
		return ec.S1.ExpendOne(from, max)
	}
	return ec.S2.ExpendOne(from, max)
}
func (ec *ExpendCombine) Validate() bool {
	return ec.S1 != nil && ec.S2 != nil && ec.S1.Validate() && ec.S2.Validate()
}
