package context

import (
	"context"
	"net/http"
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
	return &ret
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

func (me *Ctx) Dispose() {
}
