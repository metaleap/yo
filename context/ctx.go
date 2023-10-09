package context

import (
	"context"
	"net/http"

	. "yo/config"
	"yo/diag"
	"yo/str"
	. "yo/util"
)

type Ctx struct {
	context.Context
	ctxDone func()
	Req     *http.Request
	Resp    http.ResponseWriter
	UrlPath string
	Timings diag.Timings
}

func New(req *http.Request, resp http.ResponseWriter) *Ctx {
	ret := Ctx{
		Timings: diag.NewTimings("init ctx", !IsDevMode),
		Context: context.Background(),
		Req:     req,
		Resp:    resp,
		UrlPath: str.TrimR(str.TrimL(req.URL.Path, "/"), "/"),
	}
	if Cfg.YO_API_IMPL_TIMEOUT > 0 {
		ret.Context, ret.ctxDone = context.WithTimeout(ret.Context, Cfg.YO_API_IMPL_TIMEOUT)
	}
	return &ret
}

func (me *Ctx) Dispose() {
	if code, crashed := 500, recover(); crashed != nil {
		if err, is_app_err := crashed.(Err); is_app_err {
			code = If(str.Has(err.Error(), "AlreadyExists"), 409, If(str.Has(err.Error(), "DoesNotExist"), 404, 400))
		}
		me.HttpErr(code, str.Fmt("%v", crashed))
	}
	if me.ctxDone != nil {
		me.ctxDone()
	}
	if IsDevMode {
		total_duration, steps := me.Timings.AllDone()
		println(me.Req.RequestURI, str.DurationMs(total_duration))
		for _, step := range steps {
			println("\t" + step.Step + ":\t" + str.DurationMs(step.Duration))
		}
	}
}

func (me *Ctx) HttpErr(statusCode int, statusText string) {
	http.Error(me.Resp, statusText, statusCode)
}

func (me *Ctx) Get(name string) any {
	if s := me.Req.URL.Query().Get(name); s != "" {
		return s
	}
	return me.Context.Value(name)
}

func (me *Ctx) GetStr(name string) (ret string) {
	any := me.Get(name)
	ret, _ = any.(string)
	return
}
