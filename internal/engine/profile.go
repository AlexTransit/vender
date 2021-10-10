package engine

import (
	"regexp"
	"sync/atomic"
	"time"
)

type ProfileFunc func(Doer, time.Duration)

// SetProfile re=nil or fun=nil to disable profiling.
func (e *Engine) SetProfile(re *regexp.Regexp, min time.Duration, fun ProfileFunc) {
	fast := uint32(0)
	if re != nil || fun != nil {
		fast = 1
	}
	defer atomic.StoreUint32(&e.profile.fastpath, fast)
	e.profile.Lock()
	defer e.profile.Unlock()
	e.profile.re = re
	e.profile.fun = fun
	e.profile.min = min
}

func (e *Engine) matchProfile(s string) (ProfileFunc, time.Duration) {
	if atomic.LoadUint32(&e.profile.fastpath) != 1 {
		return nil, 0
	}
	e.profile.Lock()
	defer e.profile.Unlock()
	if e.profile.re != nil && e.profile.re.MatchString(s) {
		return e.profile.fun, e.profile.min
	}
	return nil, 0
}
