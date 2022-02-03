// Package log2 solves these issues:
// - log level filtering, e.g. show debug messages in internal tests only
// - safe concurrent change of log level
//
// Primary goal was to run parallel tests and log into t.Logf() safely,
// and TBH, would have been enough to pass around explicit stdlib *log.Logger.
// Well, log levels is just a cherry on top.
package log2

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"sync/atomic"
	"testing"

	"github.com/juju/errors"
)

const ContextKey = "run/log"

const (
	// type specified here helped against accidentally passing flags as level
	Lmicroseconds     int = log.Lmicroseconds
	Lshortfile        int = log.Lshortfile
	LStdFlags         int = log.Ltime | Lshortfile
	LInteractiveFlags int = log.Ltime | Lshortfile | Lmicroseconds
	LServiceFlags     int = Lshortfile
	LTestFlags        int = Lshortfile | Lmicroseconds
)

func ContextValueLogger(ctx context.Context) *Log {
	const key = ContextKey
	v := ctx.Value(key)
	if v == nil {
		// return nil
		panic(fmt.Errorf("context['%v'] is nil", key))
	}
	if log, ok := v.(*Log); ok {
		return log
	}
	panic(fmt.Errorf("context['%v'] expected type *Log", key))
}

type Level int32

const (
	LError = iota
	LInfo
	LDebug
	LAll = math.MaxInt32
)

type Log struct {
	l       *log.Logger
	level   Level
	onError atomic.Value // <ErrorHandler>
	fatalf  FmtFunc
}

func NewStderr(level Level) *Log { return NewWriter(os.Stderr, level) }
func NewWriter(w io.Writer, level Level) *Log {
	if w == ioutil.Discard {
		return nil
	}
	return &Log{
		l:     log.New(w, "", LStdFlags),
		level: level,
	}
}

type ErrorFunc func(error)
type FmtFunc func(format string, args ...interface{})
type FmtFuncWriter struct{ FmtFunc }

func NewFunc(f FmtFunc, level Level) *Log { return NewWriter(FmtFuncWriter{f}, level) }
func (ffw FmtFuncWriter) Write(b []byte) (int, error) {
	ffw.FmtFunc(string(b))
	return len(b), nil
}

func NewTest(t testing.TB, level Level) *Log {
	lg := NewFunc(t.Logf, level)
	lg.fatalf = t.Fatalf
	return lg
}

func (lg *Log) Clone(level Level) *Log {
	if lg == nil {
		return nil
	}
	new := NewWriter(lg.l.Writer(), level)
	new.fatalf = lg.fatalf
	new.storeErrorFunc(lg.loadErrorFunc())
	new.SetFlags(lg.l.Flags())
	return new
}

func (lg *Log) SetErrorFunc(f ErrorFunc) {
	if lg == nil {
		return
	}
	lg.storeErrorFunc(f)
}

func (lg *Log) SetLevel(l Level) {
	if lg == nil {
		return
	}
	atomic.StoreInt32((*int32)(&lg.level), int32(l))
}

func (lg *Log) SetFlags(f int) {
	if lg == nil {
		return
	}
	lg.l.SetFlags(f)
}

func (lg *Log) Stdlib() *log.Logger {
	if lg == nil {
		return nil
	}
	return lg.l
}

func (lg *Log) SetOutput(w io.Writer) {
	if lg == nil {
		return
	}
	lg.l.SetOutput(w)
}

func (lg *Log) SetPrefix(prefix string) {
	if lg == nil {
		return
	}
	lg.l.SetPrefix(prefix)
}

func (lg *Log) Enabled(level Level) bool {
	if lg == nil {
		return false
	}
	return atomic.LoadInt32((*int32)(&lg.level)) >= int32(level)
}

func (lg *Log) Log(level Level, s string) {
	if lg.Enabled(level) {
		_ = lg.l.Output(3, s)
	}
}
func (lg *Log) Logf(level Level, format string, args ...interface{}) {
	if lg.Enabled(level) {
		_ = lg.l.Output(3, fmt.Sprintf(format, args...))
	}
}

// compatibility with eclipse.paho.mqtt
func (lg *Log) Printf(format string, args ...interface{}) { lg.Logf(LInfo, format, args...) }
func (lg *Log) Println(args ...interface{})               { lg.Log(LInfo, fmt.Sprint(args...)) }

func (lg *Log) Info(args ...interface{}) {
	lg.Log(LInfo, fmt.Sprint(args...))
}
func (lg *Log) Infof(format string, args ...interface{}) {
	lg.Logf(LInfo, format, args...)
}
func (lg *Log) Debug(args ...interface{}) {
	lg.Log(LDebug, "debug: "+fmt.Sprint(args...))
}
func (lg *Log) Debugf(format string, args ...interface{}) {
	lg.Logf(LDebug, "debug: "+format, args...)
}

func (lg *Log) Error(args ...interface{}) {
	lg.Log(LError, "error: "+fmt.Sprint(args...))
	if lg == nil {
		return
	}
	if errfun := lg.loadErrorFunc(); errfun != nil {
		var e error
		if len(args) >= 1 {
			e, _ = args[0].(error)
		}
		if e != nil {
			args = args[1:]
			if len(args) > 0 { // Log.Error(err, arg1) please don't do this
				rest := fmt.Sprint(args...)
				e = errors.Annotate(e, rest)
			}
			errfun(e)
		}
	}
}
func (lg *Log) Errorf(format string, args ...interface{}) {
	lg.Logf(LError, "error: "+format, args...)
	if lg == nil {
		return
	}
	if errfun := lg.loadErrorFunc(); errfun != nil {
		e := fmt.Errorf(fmt.Sprintf(format, args...))
		errfun(e)
	}
}

func (lg *Log) Fatalf(format string, args ...interface{}) {
	if lg.fatalf != nil {
		lg.fatalf(format, args...)
	} else {
		lg.Logf(LError, "fatal: "+format, args...)
		os.Exit(1)
	}
}
func (lg *Log) Fatal(args ...interface{}) {
	s := fmt.Sprint(args...)
	if lg.fatalf != nil {
		lg.fatalf(s)
	} else {
		lg.Logf(LError, "fatal: "+s)
		os.Exit(1)
	}
}

// workaround for atomic.Value with nil
type wrapErrorFunc struct{ ErrorFunc }

func (lg *Log) loadErrorFunc() ErrorFunc {
	if x := lg.onError.Load(); x != nil {
		return x.(wrapErrorFunc).ErrorFunc
	} else {
		return nil
	}
}

func (lg *Log) storeErrorFunc(new ErrorFunc) {
	lg.onError.Store(wrapErrorFunc{new})
}
