package yo

import (
	"context"
	"net/http"
)

type Ctx struct {
	context.Context
	ctxDone func()
	Req     *http.Request
}

func ctxNew(req *http.Request) *Ctx {
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

func (me *Ctx) dispose() {
}
