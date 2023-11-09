package yoctx

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync"
	"time"

	. "yo/util"
	"yo/util/dict"
	"yo/util/sl"
	"yo/util/str"
)

const (
	ErrMustBeAdmin        = Err("MustBeAdmin")
	ErrTimedOut           = Err("TimedOut")
	ErrDbUpdExpectedIdGt0 = Err("DbUpdExpectedIdGt0")
	CtxKeyForcedTestUser  = "yoUserTest" // handled only with IsDevMode==true
	CtxKeyDbNoLogging     = "yoCtxDbNoLogging"
)

var (
	IsUp            bool
	DB              *sql.DB
	OnDone          []func(ctx *Ctx, fail any)
	NotifyErrCaught = func(nowInvalidCtx *Ctx, ctxVals dict.Any, fail any, stackTrace string) {}
)

type apiMethod interface {
	KnownErrs() []Err
	PkgName() string
}

type Ctx struct {
	context.Context
	ctxDone func()
	ctxVals dict.Any
	caches  struct {
		maps map[string]map[any]any
		muts map[string]*sync.RWMutex
		mut  *sync.Mutex
	}
	Http *ctxHttp
	Job  *ctxJob
	Db   struct {
		PrintRawSqlInDevMode bool // never printed in non-dev-mode anyway
		Tx                   *sql.Tx
	}
	Timings                 Timings
	TimingsNoPrintInDevMode bool // never printed in non-dev-mode anyway
	DevModeNoCatch          bool
	ErrNoNotify             bool
	ErrNoNotifyOf           []Err
}

type ctxHttp struct {
	Req         *http.Request
	Resp        http.ResponseWriter
	UrlPath     string
	ApiMethod   apiMethod
	reqCookies  str.Dict
	respCookies map[string]*http.Cookie
	respWriting bool
}

type ctxJob struct {
	RunId   int64
	Details any
	TaskId  int64
}

func newCtx(timeout time.Duration, cancelable bool, timingsName string) *Ctx {
	if IsDevMode && (timeout <= 0) && !cancelable {
		panic("unsupported Ctx")
	}
	me := Ctx{Context: context.Background(), ctxVals: dict.Any{},
		Timings: NewTimings(timingsName, "init ctx"), TimingsNoPrintInDevMode: (timingsName == "")}
	me.caches.mut, me.caches.maps, me.caches.muts = new(sync.Mutex), map[string]map[any]any{}, map[string]*sync.RWMutex{}
	if timeout > 0 {
		me.Context, me.ctxDone = context.WithTimeout(me.Context, timeout)
	}
	if cancelable {
		me.Context, me.ctxDone = context.WithCancel(me.Context)
	}
	return &me
}

func NewCtxForHttp(req *http.Request, resp http.ResponseWriter, timeout time.Duration, cancelable bool) *Ctx {
	ctx := newCtx(timeout, cancelable, req.RequestURI)
	ctx.Http = &ctxHttp{Req: req, Resp: resp, UrlPath: str.TrimSuff(str.TrimPref(req.URL.Path, "/"), "/"), respCookies: map[string]*http.Cookie{}, reqCookies: str.Dict{}}
	return ctx
}

var NewCtxNonHttp = newCtx

func (me *Ctx) Cancel() {
	if me.ctxDone != nil {
		me.ctxDone()
	}
}

func (me *Ctx) WithJob(jobRunId int64, jobRunDetails any, jobTaskId int64) *Ctx {
	me.Job = &ctxJob{RunId: jobRunId, Details: jobRunDetails, TaskId: jobTaskId}
	return me
}

func (me *Ctx) CopyButWith(timeout time.Duration, cancelable bool) *Ctx {
	ret := *me
	ret.Db.Tx, ret.Context, ret.ctxDone = nil, context.Background(), nil
	if timeout > 0 {
		ret.Context, ret.ctxDone = context.WithTimeout(ret.Context, timeout)
	} else if dt_deadline, has := me.Context.Deadline(); has && (timeout < 0) {
		ret.Context, ret.ctxDone = context.WithDeadline(ret.Context, dt_deadline)
	}
	if cancelable {
		ret.Context, ret.ctxDone = context.WithCancel(ret.Context)
	} else if _, has_deadline := ret.Context.Deadline(); !has_deadline {
		panic("unsupported Ctx")
	}
	return &ret
}

func (me *Ctx) OnDone(alsoDo func()) (fail any) {
	if me == nil {
		return
	}
	if (!IsDevMode) || CatchPanics { // comptime branch
		if IsUp && ((!IsDevMode) || !me.DevModeNoCatch) { // runtime branch, keep sep from above comptime one
			if fail = recover(); IsDevMode && (fail != nil) {
				println(str.Fmt(">>>>>>>>>%v<<<<<<<<<", fail))
			}
		}
	}
	if err, _ := fail.(error); err == context.DeadlineExceeded {
		fail = ErrTimedOut
	}
	if alsoDo != nil {
		alsoDo()
	}
	if me.Db.Tx != nil {
		if fail == nil {
			if fail = me.Db.Tx.Commit(); (IsDevMode || !IsUp) && (fail != nil) {
				println(str.Fmt(">>TXC>>%v<<TXC<<", fail))
			}
		}
		if fail != nil {
			_ = me.Db.Tx.Rollback()
		}
	}
	if me.Http != nil {
		me.httpEnsureCookiesSent()
		if code := 500; fail != nil {
			if err, is_app_err := fail.(Err); is_app_err {
				if IsDevMode && me.Http.ApiMethod != nil {
					if known_errs := me.Http.ApiMethod.KnownErrs(); (len(known_errs) > 0) && (err != ErrMustBeAdmin) && !sl.Has(known_errs, err) {
						os.Stderr.WriteString("\n\nunexpected/undocumented Err thrown: " + string(err) + " not found in " + str.GoLike(known_errs) + ", add it to " + str.GoLike(me.Http.ApiMethod) + "\n\n")
						os.Stderr.Sync()
						os.Exit(1)
					}
				}
				code = err.HttpStatusCodeOr(code)
			}
			me.HttpErr(code, str.Fmt("%v", fail))
		}
	}
	for _, on_done := range OnDone {
		on_done(me, fail)
	}
	if me.ctxDone != nil {
		me.ctxDone()
	}

	if err_no_notify := me.ErrNoNotify; (!err_no_notify) && (fail != nil) {
		if err, _ := fail.(Err); (err != "") && (len(me.ErrNoNotifyOf) > 0) {
			err_no_notify = sl.Has(me.ErrNoNotifyOf, err)
		}
		if !err_no_notify {
			callers := make([]uintptr, 128)
			num_callers := runtime.Callers(0, callers)
			callers = callers[:num_callers]
			stack_trace, frames := "", runtime.CallersFrames(callers)
			for more := len(callers) > 0; more; {
				var frame runtime.Frame
				frame, more = frames.Next()
				stack_trace += str.Fmt("%s:%d %s\n", frame.File, frame.Line, frame.Function)
			}
			go NotifyErrCaught(me, me.ctxVals, fail, stack_trace)
		}
	}
	if (IsDevMode || !IsUp) && !me.TimingsNoPrintInDevMode {
		total_duration, steps := me.Timings.AllDone()
		println("\n" + me.Timings.String() + "\n  . . . " + str.DurationMs(total_duration) + ", like so:")
		for _, step := range steps {
			println(step.Step + ":\t" + str.DurationMs(step.Time))
		}
	}
	return
}

// context.Context impl/override
func (me *Ctx) Value(key any) any {
	if k, _ := key.(string); k != "" {
		return me.Get(k, nil)
	}
	return me.Context.Value(key)
}

func (me *Ctx) Get(name string, defaultValue any) any {
	if value, got := me.ctxVals[name]; got {
		return value
	}
	if me.Http != nil {
		if s := me.Http.Req.URL.Query().Get(name); s != "" {
			return s
		}
	}
	if value := me.Context.Value(name); value != nil {
		return value
	}
	return defaultValue
}

func (me *Ctx) GetStr(name string) (ret string) {
	ret, _ = me.Get(name, "").(string)
	return
}

func (me *Ctx) Set(name string, value any) {
	me.ctxVals[name] = value
}

func (me *Ctx) DbTx() {
	if me.Db.Tx != nil {
		return
	}
	var err error
	if me.Db.Tx, err = DB.BeginTx(me, nil); err != nil {
		panic(err)
	}
}

func (me *Ctx) DbNoLoggingInDevMode() {
	if IsDevMode {
		me.Set(CtxKeyDbNoLogging, true)
		me.Context = context.WithValue(me.Context, CtxKeyDbNoLogging, true)
	}
}

func (me *Ctx) HttpErr(statusCode int, statusText string) {
	http.Error(me.Http.Resp, statusText, statusCode)
}

func (me *Ctx) HttpOnPreWriteResponse() {
	if me.Http.respWriting {
		panic("new bug: more than one call to HttpOnPreWriteResponse")
	}
	me.Http.respWriting = true
	me.httpEnsureCookiesSent()
}

func (me *Ctx) httpEnsureCookiesSent() {
	for _, cookie := range me.Http.respCookies {
		http.SetCookie(me.Http.Resp, cookie)
	}
	clear(me.Http.respCookies)
}

func (me *Ctx) HttpGetCookie(cookieName string) (ret string) {
	if found, got := me.Http.reqCookies[cookieName]; got {
		return found
	}
	me.Http.reqCookies[cookieName] = ""
	if cookie, err := me.Http.Req.Cookie(cookieName); err != nil && err != http.ErrNoCookie {
		panic(err)
	} else if cookie != nil {
		if ret, err = url.QueryUnescape(cookie.Value); err != nil {
			panic(err)
		}
		me.Http.reqCookies[cookieName] = ret
	}
	return ret
}

func (me *Ctx) HttpSetCookie(cookieName string, cookieValue string, numDays int) {
	me.Http.respCookies[cookieName] = &http.Cookie{
		Name:     cookieName,
		Value:    url.QueryEscape(cookieValue),
		MaxAge:   If(cookieValue == "", -1, int(time.Hour*24)*numDays),
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
		HttpOnly: true,
	}
}
