package yoctx

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"time"

	yodiag "yo/diag"
	. "yo/util"
	"yo/util/str"
)

var (
	DB     *sql.DB
	OnDone []func(ctx *Ctx, fail any)
)

type Ctx struct {
	context.Context
	ctxDone func()
	ctxVals map[string]any
	Http    struct {
		Req         *http.Request
		Resp        http.ResponseWriter
		UrlPath     string
		reqCookies  str.Dict
		respCookies map[string]*http.Cookie
	}
	Db struct {
		Tx *sql.Tx
	}
	Timings yodiag.Timings
}

func newCtx(timeout time.Duration) *Ctx {
	ctx := Ctx{Timings: yodiag.NewTimings("init ctx", !IsDevMode), Context: context.Background(), ctxVals: map[string]any{}}
	if timeout > 0 {
		ctx.Context, ctx.ctxDone = context.WithTimeout(ctx.Context, timeout)
	}
	return &ctx
}

func NewForHttp(req *http.Request, resp http.ResponseWriter, timeout time.Duration) *Ctx {
	ctx := newCtx(timeout)
	ctx.Http.Req, ctx.Http.Resp, ctx.Http.UrlPath, ctx.Http.respCookies, ctx.Http.reqCookies =
		req, resp, str.TrimR(str.TrimL(req.URL.Path, "/"), "/"), map[string]*http.Cookie{}, str.Dict{}
	return ctx
}

func NewNonHttp(timeout time.Duration) *Ctx {
	ctx := newCtx(timeout)
	if DB != nil {
		ctx.DbTx()
	}
	return ctx
}

func (me *Ctx) Dispose() {
	const catch = true // false // gotta toggle occasionally during local debug
	var fail any
	if catch {
		fail = recover()
	}
	if err, _ := fail.(error); err == context.DeadlineExceeded {
		fail = Err("OperationTimedOut")
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
				code = err.HttpStatusCode()
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
	if IsDevMode {
		total_duration, steps := me.Timings.AllDone()
		if me.Http.Req != nil {
			println(me.Http.Req.RequestURI, str.DurationMs(total_duration))
			for _, step := range steps {
				println("\t" + step.Step + ":\t" + str.DurationMs(step.Time))
			}
		}
	}
}

// context.Context impl/override
func (me *Ctx) Value(key any) any {
	if k, _ := key.(string); k != "" {
		return me.Get(k)
	}
	return me.Context.Value(key)
}

func (me *Ctx) Get(name string) any {
	if value, got := me.ctxVals[name]; got {
		return value
	}
	if me.Http.Req != nil {
		if s := me.Http.Req.URL.Query().Get(name); s != "" {
			return s
		}
	}
	return me.Context.Value(name)
}

func (me *Ctx) GetStr(name string) (ret string) {
	ret, _ = me.Get(name).(string)
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
