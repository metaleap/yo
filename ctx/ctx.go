package yoctx

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	yodiag "yo/diag"
	. "yo/util"
	"yo/util/str"
)

type Ctx struct {
	context.Context
	ctxDone func()
	Http    struct {
		Req     *http.Request
		Resp    http.ResponseWriter
		UrlPath string
	}
	Db struct {
		Tx *sql.Tx
	}
	Timings yodiag.Timings
}

func newCtx(timeout time.Duration) *Ctx {
	ctx := Ctx{
		Timings: yodiag.NewTimings("init ctx", !IsDevMode),
		Context: context.Background(),
	}
	if timeout > 0 {
		ctx.Context, ctx.ctxDone = context.WithTimeout(ctx.Context, timeout)
	}
	return &ctx
}

func NewForHttp(req *http.Request, resp http.ResponseWriter, timeout time.Duration) *Ctx {
	ctx := newCtx(timeout)
	ctx.Http.Req, ctx.Http.Resp, ctx.Http.UrlPath = req, resp, str.TrimR(str.TrimL(req.URL.Path, "/"), "/")
	return ctx
}

func NewForDbTx(timeout time.Duration) *Ctx {
	ctx := newCtx(timeout)
	return ctx
}

func (me *Ctx) Dispose() {
	if me.Http.Req != nil && me.Http.Resp != nil {
		if code, fail := 500, recover(); fail != nil {
			if err, is_app_err := fail.(Err); is_app_err {
				code = err.HttpStatusCode()
			}
			me.HttpErr(code, str.Fmt("%v", fail))
		}
	}
	if me.ctxDone != nil {
		me.ctxDone()
	}
	if IsDevMode {
		total_duration, steps := me.Timings.AllDone()
		if me.Http.Req != nil {
			println(me.Http.Req.RequestURI, str.DurationMs(total_duration))
			for _, step := range steps {
				println("\t" + step.Step + ":\t" + str.DurationMs(step.Duration))
			}
		}
	}
}

func (me *Ctx) HttpErr(statusCode int, statusText string) {
	http.Error(me.Http.Resp, statusText, statusCode)
}

func (me *Ctx) Get(name string) any {
	if s := me.Http.Req.URL.Query().Get(name); s != "" {
		return s
	}
	return me.Context.Value(name)
}

func (me *Ctx) GetStr(name string) (ret string) {
	any := me.Get(name)
	ret, _ = any.(string)
	return
}
