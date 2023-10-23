package yoctx

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"os"
	"time"

	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

const (
	ErrMustBeAdmin        = Err("MustBeAdmin")
	ErrTimedOut           = Err("TimedOut")
	ErrDbUpdExpectedIdGt0 = Err("DbUpdExpectedIdGt0")
	CtxKeyForcedTestUser  = "yoUserTest" // handled only with IsDevMode==true
)

var (
	DB     *sql.DB
	OnDone []func(ctx *Ctx, fail any)
)

type apiMethod interface {
	KnownErrs() []Err
	PkgName() string
}

type Ctx struct {
	context.Context
	ctxDone func()
	ctxVals map[string]any
	Http    struct {
		Req         *http.Request
		Resp        http.ResponseWriter
		UrlPath     string
		ApiMethod   apiMethod
		reqCookies  str.Dict
		respCookies map[string]*http.Cookie
		respWriting bool
	}
	Db struct {
		PrintRawSqlInDevMode bool // never printed in non-dev-mode anyway
		Tx                   *sql.Tx
	}
	Timings                 Timings
	TimingsNoPrintInDevMode bool // never printed in non-dev-mode anyway
}

func newCtx(timeout time.Duration, timingsName string) *Ctx {
	ctx := Ctx{Timings: NewTimings(timingsName, "init ctx"), Context: context.Background(), ctxVals: map[string]any{}}
	if timeout > 0 {
		ctx.Context, ctx.ctxDone = context.WithTimeout(ctx.Context, timeout)
	}
	return &ctx
}

func NewCtxForHttp(req *http.Request, resp http.ResponseWriter, timeout time.Duration) *Ctx {
	ctx := newCtx(timeout, req.RequestURI)
	ctx.Http.Req, ctx.Http.Resp, ctx.Http.UrlPath, ctx.Http.respCookies, ctx.Http.reqCookies =
		req, resp, str.TrimR(str.TrimL(req.URL.Path, "/"), "/"), map[string]*http.Cookie{}, str.Dict{}
	return ctx
}

var NewCtxNonHttp = newCtx

func (me *Ctx) OnDone(subTimings Timings) {
	var fail any
	if (!IsDevMode) || catchPanics {
		if fail = recover(); IsDevMode && (fail != nil) {
			println(str.Fmt(">>>>>>>>>%v<<<<<<<<<", fail))
		}
	}
	if err, _ := fail.(error); err == context.DeadlineExceeded {
		fail = ErrTimedOut
	}
	if me.Db.Tx != nil {
		if fail == nil {
			fail = me.Db.Tx.Commit()
		}
		if fail != nil {
			_ = me.Db.Tx.Rollback()
		}
	}
	if me.Http.Req != nil && me.Http.Resp != nil {
		me.httpEnsureCookiesSent()
		if code := 500; fail != nil {
			if err, is_app_err := fail.(Err); is_app_err {
				if IsDevMode && me.Http.ApiMethod != nil {
					if known_errs := me.Http.ApiMethod.KnownErrs(); (len(known_errs) > 0) && (err != ErrMustBeAdmin) && !sl.Has(err, known_errs) {
						os.Stderr.WriteString("\n\nunexpected/undocumented Err thrown: " + string(err) + " not found in " + str.From(known_errs) + ", add it to " + str.From(me.Http.ApiMethod) + "\n\n")
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
	if IsDevMode && !me.TimingsNoPrintInDevMode {
		do_print_timings := func(it Timings) {
			total_duration, steps := it.AllDone()
			println("\n" + it.String() + "\n  . . . " + str.DurationMs(total_duration) + ", like so:")
			for _, step := range steps {
				println(step.Step + ":\t" + str.DurationMs(step.Time))
			}
		}
		if do_print_timings(me.Timings); subTimings != nil {
			do_print_timings(subTimings)
		}
	}
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
	if me.Http.Req != nil {
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
	if me.Db.Tx == nil {
		return
	}
	var err error
	if me.Db.Tx, err = DB.BeginTx(me, nil); err != nil {
		panic(err)
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
