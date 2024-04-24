package engine

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/log2"
	"github.com/juju/errors"
)

type ErrNotResolved struct{ msg string }

func NewErrNotResolved(action string) ErrNotResolved {
	return ErrNotResolved{msg: fmt.Sprintf("action=%s not resolved", action)}
}
func (e ErrNotResolved) Error() string { return e.msg }

type Engine struct {
	Log     *log2.Log
	lk      sync.RWMutex
	actions map[string]Doer
	profile struct {
		// optimistic field access guard; fastpath=0 -> profiling disabled, don't touch mutex
		fastpath uint32

		sync.Mutex // fields below access guard

		re  *regexp.Regexp
		min time.Duration
		fun ProfileFunc
	}
}

func NewEngine(log *log2.Log) *Engine {
	e := &Engine{
		Log:     log,
		actions: make(map[string]Doer, 128),
	}
	e.actions["ignore(?)"] = FuncArg{
		Name: "ignore(?)",
		F:    func(context.Context, Arg) error { return nil },
	}
	e.actions["sleep(100ms)"] = Sleep{Duration: 100 * time.Millisecond}
	return e
}

func (e *Engine) Register(action string, d Doer) {
	e.lk.Lock()
	e.actions[action] = d
	e.lk.Unlock()
}

func (e *Engine) RegisterNewFunc(name string, fun func(context.Context) error) {
	e.Register(name, Func{
		Name: name,
		F:    fun,
	})
}

func (e *Engine) RegisterNewFuncAgr(name string, fun func(ctx context.Context, arg Arg) error) {
	e.Register(name, FuncArg{
		Name: name,
		F:    fun,
	})
}

func (e *Engine) RegisterNewSeq(name string, ds ...Doer) {
	tx := NewSeq(name)
	for _, d := range ds {
		tx.Append(d)
	}
	e.Register(name, tx)
}

func (e *Engine) RegisterParse(name, scenario string) error {
	d, err := e.ParseText(name, scenario)
	if err != nil {
		err = errors.Annotatef(err, "engine.RegisterParse() name=%s scenario=%s", name, scenario)
		return err
	}
	e.Register(name, d)
	return nil
}

var reActionArg = regexp.MustCompile(`^(.+)\((\d+|\?)\)$`)

func (e *Engine) resolve(action string) (Doer, error) {
	// e.Log.Debugf("engine.resolve action=%s", action)
	e.lk.RLock()
	defer e.lk.RUnlock()
	return e.locked_resolve(action)
}

type token struct {
	tag  string
	norm string
	arg  string
	ok   bool
}

func parseArg(s string) token {
	match := reActionArg.FindStringSubmatch(s)
	if match == nil {
		return token{tag: s}
	}
	return token{
		tag:  match[1],
		norm: match[1] + "(?)",
		arg:  match[2],
		ok:   true,
	}
}

func (e *Engine) locked_resolve(action string) (Doer, error) {
	d, ok := e.actions[action]
	if ok {
		// e.Log.Debugf("engine.resolve action=%s resolved d=%v", action, d)
		return d, nil
	}

	tok := parseArg(action)
	if !tok.ok {
		// e.Log.Debugf("engine.resolve action=%s match=nil", action)
		return nil, NewErrNotResolved(action)
	}

	d, ok = e.actions[tok.norm]
	// e.Log.Debugf("engine.resolve action=%s match=%v normalized=%s ok=%t", action, match, tok.norm, ok)
	if !ok {
		e.Log.Debugf("resolve action=%s normalized=%s not found", action, tok.norm)
		err := NewErrNotResolved(tok.norm)
		err.msg = fmt.Sprintf(FmtErrContext, action) + err.msg
		return nil, err
	}
	if tok.arg != "?" {
		argn, err := strconv.Atoi(tok.arg)
		if err != nil {
			e.Log.Debugf("resolve action=%s err=%s", action, err)
			return nil, errors.Annotatef(err, FmtErrContext, action)
		}
		var applied bool
		d, applied, err = ArgApply(d, Arg(argn))
		if err != nil {
			e.Log.Debugf("resolve action=%s err=%s", action, err)
			return nil, errors.Annotatef(err, FmtErrContext, action)
		}
		if !applied {
			e.Log.Debugf("resolve action=%s arg=%v not applied", action, tok.arg)
			err = ErrArgNotApplied
			return nil, errors.Annotatef(err, FmtErrContext, action)
		}
	}
	return d, nil
}

func (e *Engine) Resolve(action string) Doer {
	d, err := e.resolve(action)
	if err != nil {
		e.Log.Errorf("engine.Resolve action=%s err=%v", action, err)
		return Fail{E: err}
	}
	return d
}

func (e *Engine) List() []string {
	e.lk.RLock()
	r := make([]string, 0, len(e.actions))
	for k := range e.actions {
		r = append(r, k)
	}
	e.lk.RUnlock()
	return r
}

var reSleep = regexp.MustCompile(`sleep\((\d+m?s)\)`)

func (e *Engine) ResolveOrLazy(action string) (Doer, error) {
	e.lk.RLock()
	defer e.lk.RUnlock()
	d, ok := e.actions[action]
	if ok {
		// e.Log.Debugf("engine.ResolveOrLazy %s -> ok %#v", action, d)
		return d, nil
	}

	if m := reSleep.FindStringSubmatch(action); len(m) == 2 {
		duration, err := time.ParseDuration(m[1])
		if err != nil {
			return nil, errors.Trace(err)
		}
		return Sleep{duration}, nil
	}

	// e.Log.Debugf("engine.ResolveOrLazy %s -> lazy %#v", action, d)
	return &Lazy{Name: action, r: e.resolve}, nil
}

var reNotSpace = regexp.MustCompile(`\S+`)

func (e *Engine) ParseText(tag, text string) (Doer, error) {
	// TODO cache with github.com/hashicorp/golang-lru

	errs := make([]error, 0)
	words := reNotSpace.FindAllString(text, -1)

	tx := NewSeq(tag)
	for _, word := range words {
		d, err := e.ResolveOrLazy(word)
		if err != nil {
			return nil, errors.Annotatef(err, "scenario=%s unparsed=%s", text, word)
		}
		tx.Append(d)
	}
	return tx, helpers.FoldErrors(errs)
}

func (e *Engine) Exec(ctx context.Context, d Doer) error     { return e.exec(ctx, d, false, true) }
func (e *Engine) ExecPart(ctx context.Context, d Doer) error { return e.exec(ctx, d, false, false) }
func (e *Engine) ValidateExec(ctx context.Context, d Doer) error {
	return e.exec(ctx, d, true, true)
}

func (e *Engine) ExecList(ctx context.Context, tag string, list []string) []error {
	// e.Log.Debugf("engine.ExecList tag=%s list=%v", tag, list)

	errs := make([]error, 0, len(list))
	for i, text := range list {
		itemTag := fmt.Sprintf("%s:%d", tag, i)
		d, err := e.ParseText(itemTag, text)
		if err == nil {
			err = e.exec(ctx, d, true, true)
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (e *Engine) exec(ctx context.Context, d Doer, validate, enableProfile bool) (err error) {
	if d == nil {
		err = fmt.Errorf("doer nil")
		return
	}
	if validate {
		err = d.Validate()
	}
	if err == nil {
		if enableProfile {
			tag := d.String() // FIXME faster .Tag() or cache result
			if profFun, profMin := e.matchProfile(tag); profFun != nil {
				tbegin := time.Now()
				defer func() {
					if duration := time.Since(tbegin); duration >= profMin {
						profFun(d, duration)
					}
				}()
			}
		}
		err = d.Do(ctx)
	}
	return err
}

// IsNotResolved Test `error` or `Doer` against ErrNotResolved
func IsNotResolved(x interface{}) bool {
	if x == nil {
		return false
	}
	e, _ := x.(error)
	if e == nil {
		if f, ok := x.(Fail); ok {
			e = f.E
		}
	}
	if e == nil {
		return false
	}
	e = errors.Cause(e)
	_, ok := e.(ErrNotResolved)
	return ok
}
