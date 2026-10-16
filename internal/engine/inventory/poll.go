package inventory

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"

	"github.com/AlexTransit/vender/internal/engine"
)

// poolCodeStep - коды ошибок бункеров пула смещаются на номер бункера * 100.
const poolCodeStep = 100

// Pool - все бункеры одного ингредиента. Команда add.<ingredient>(?) регистрируется на пул.
// Stocks отсортированы по Code; позиция в срезе - номер бункера в пуле (с 0).
type Pool struct {
	Ingredient *Ingredient
	Stocks     []*Stock
	Doers      []engine.Doer // Doers[i] - register_add склада Stocks[i]; nil, если сценария нет

	mu       sync.Mutex
	excluded []bool
}

type step struct {
	idx  int
	doer engine.Doer
}

type portion struct {
	idx int
	arg int16
}

// newPools группирует склады по ингредиенту. stocks уже отсортирован по Code.
func newPools(stocks []Stock, doers []engine.Doer) []*Pool {
	var pools []*Pool
	byName := make(map[string]*Pool)
	for i := range stocks {
		s := &stocks[i]
		if s.Ingredient == nil {
			continue
		}
		name := s.Ingredient.Name
		p, ok := byName[name]
		if !ok {
			p = &Pool{Ingredient: s.Ingredient}
			byName[name] = p
			pools = append(pools, p)
		}
		p.Stocks = append(p.Stocks, s)
		p.Doers = append(p.Doers, doers[i])
		p.excluded = append(p.excluded, false)
	}
	return pools
}

func (p *Pool) hasDoers() bool {
	for _, d := range p.Doers {
		if d != nil {
			return true
		}
	}
	return false
}

// Doer возвращает команду add.<ingredient>(?) по пулу.
func (p *Pool) Doer() engine.Doer { return &poolDoer{pool: p} }

// available - сколько единиц аргумента можно взять из бункера.
// Правило то же, что в Stock.Has: min == 0 - без ограничений, иначе не ниже min.
func available(s *Stock) int16 {
	ing := s.Ingredient
	if ing.Min == 0 || ing.SpendRate <= 0 {
		return math.MaxInt16
	}
	free := s.Value() - float32(ing.Min)
	if free <= 0 {
		return 0
	}
	n := math.Floor(float64(free) / float64(ing.SpendRate))
	if n > math.MaxInt16 {
		return math.MaxInt16
	}
	return int16(n)
}

// split распределяет arg по бункерам: сначала с большим остатком, потом добирает со следующих.
// Исключённые и без сценария бункеры пропускаются.
func (p *Pool) split(arg int16) ([]portion, error) {
	if arg == 0 { // пропускаем, если пользователь уменьшил количество до нуля.
		return nil, nil
	}

	if arg < 0 {
		return nil, fmt.Errorf("pool=%s bad arg=%d", p.Ingredient.Name, arg)
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	order := make([]int, 0, len(p.Stocks))
	for i := range p.Stocks {
		if p.Doers[i] != nil && !p.excluded[i] {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool {
		sa, sb := p.Stocks[order[a]], p.Stocks[order[b]]
		va, vb := sa.Value(), sb.Value()
		if va != vb {
			return va > vb
		}
		return sa.Code < sb.Code
	})

	var out []portion
	rest := arg
	for _, i := range order {
		if rest == 0 {
			break
		}
		n := available(p.Stocks[i])
		if n == 0 {
			continue
		}
		if n > rest {
			n = rest
		}
		out = append(out, portion{idx: i, arg: n})
		rest -= n
	}
	if rest > 0 {
		return nil, fmt.Errorf("%s low", p.Ingredient.Name)
	}
	return out, nil
}

// plan делит отгрузку и проверяет каждый шаг до начала исполнения.
// Шаг строится через Stock.Wrap, проверки остатка и сценария делает custom.Validate.
func (p *Pool) plan(arg int16) ([]step, error) {
	parts, err := p.split(arg)
	if err != nil {
		return nil, err
	}
	steps := make([]step, 0, len(parts))
	for _, pt := range parts {
		doer, _, err := p.Stocks[pt.idx].Wrap(p.Doers[pt.idx]).Apply(engine.Arg(pt.arg))
		if err == nil {
			err = doer.Validate()
		}
		if err != nil {
			return nil, p.bunkerErr(pt.idx, err)
		}
		steps = append(steps, step{idx: pt.idx, doer: doer})
	}
	return steps, nil
}

// exclude исключает бункер с позицией idx в пуле из отгрузки.
func (p *Pool) exclude(idx int) error {
	if idx < 0 || idx >= len(p.Stocks) {
		return fmt.Errorf("pool=%s bad stock index=%d", p.Ingredient.Name, idx)
	}
	p.mu.Lock()
	p.excluded[idx] = true
	p.mu.Unlock()
	if s := p.Stocks[idx]; s.Log != nil {
		s.Log.Errorf("stock=%s excluded", s.Label)
	}
	return nil
}

// coder - ошибка с числовым кодом сценария бункера.
type coder interface{ Code() int }

func errCode(err error) (int, bool) {
	var c coder
	if errors.As(err, &c) {
		return c.Code(), true
	}
	return 0, false
}

// poolError - ошибка бункера с кодом пула: номер бункера * 100 + код ошибки бункера.
type poolError struct {
	code int
	err  error
}

func (e *poolError) Error() string { return fmt.Sprintf("%d: %v", e.code, e.err) }
func (e *poolError) Unwrap() error { return e.err }
func (e *poolError) Code() int     { return e.code }

// bunkerErr переводит код ошибки бункера в код пула. Ошибки без кода не меняются.
func (p *Pool) bunkerErr(idx int, err error) error {
	base, ok := errCode(err)
	if !ok {
		return err
	}
	if base < 0 || base >= poolCodeStep {
		return fmt.Errorf("pool=%s stock index=%d code=%d out of range: %w", p.Ingredient.Name, idx, base, err)
	}
	return &poolError{code: idx*poolCodeStep + base, err: err}
}

// poolDoer - команда add.<ingredient>(?) по пулу бункеров.
type poolDoer struct {
	pool *Pool
	arg  engine.Arg
}

// FixErrorAction исключает бункер, которому принадлежит код ошибки.
func (d *poolDoer) FixErrorAction(code string) engine.Doer {
	n, err := strconv.Atoi(code)
	if err != nil {
		return nil
	}
	_ = d.pool.exclude(n / poolCodeStep)
	return nil
}

// AddErrorAction implements [engine.Doer].
func (d *poolDoer) AddErrorAction(code string, doer engine.Doer, skipMain bool) {
	panic("unimplemented")
}

func (d *poolDoer) Apply(arg engine.Arg) (engine.Doer, bool, error) {
	if _, ok := arg.(int16); !ok {
		return nil, false, fmt.Errorf("pool=%s arg type %T", d.pool.Ingredient.Name, arg)
	}
	return &poolDoer{pool: d.pool, arg: arg}, true, nil
}

// Calculation - сумма стоимости шагов плана. При ошибке плана возвращает 0.
func (d *poolDoer) Calculation() float64 {
	steps, err := d.pool.plan(d.arg.(int16))
	if err != nil {
		return 0
	}
	var sum float64
	for _, st := range steps {
		sum += st.doer.Calculation()
	}
	return sum
}

func (d *poolDoer) Validate() error {
	_, err := d.pool.plan(d.arg.(int16))
	return err
}

// Do применяет тюнинг к общему аргументу один раз и передаёт шагам контекст без тюнинга,
// иначе каждый шаг применил бы коэффициент повторно.
func (d *poolDoer) Do(ctx context.Context) error {
	e := engine.GetGlobal(ctx)
	arg := d.arg.(int16)
	runCtx := ctx
	if tunedCtx, tuneRate, ok := takeTuneRate(ctx, d.pool.Ingredient.TuneKey); ok {
		runCtx = tunedCtx
		arg = int16(math.Round(float64(arg) * float64(tuneRate)))
	}
	steps, err := d.pool.plan(arg)
	if err != nil {
		return err
	}
	for _, st := range steps {
		if err := e.Exec(runCtx, st.doer); err != nil {
			return d.pool.bunkerErr(st.idx, err)
		}
	}
	return nil
}

func (d *poolDoer) String() string {
	return fmt.Sprintf("pool.%s(%v)", d.pool.Ingredient.Name, d.arg)
}
