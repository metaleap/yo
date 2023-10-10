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
	if code, fail := 500, recover(); fail != nil {
		if err, is_app_err := fail.(Err); is_app_err {
			code = err.HttpStatusCode()
		}
		me.HttpErr(code, str.Fmt("%v", fail))
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
