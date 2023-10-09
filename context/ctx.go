package context

import (
	"context"
	"net/http"

	. "yo/config"
)

type Ctx struct {
	context.Context
	ctxDone func()
	Req     *http.Request
}

func New(req *http.Request) *Ctx {
	ret := Ctx{
		Context: context.Background(),
		Req:     req,
	}
	if Cfg.YO_API_IMPL_TIMEOUT > 0 {
		ret.Context, ret.ctxDone = context.WithTimeout(ret.Context, Cfg.YO_API_IMPL_TIMEOUT)
	}
	return &ret
}

func (me *Ctx) Dispose() {
	me.ctxDone()
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
