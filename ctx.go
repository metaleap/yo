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
	}
	return &ret
}

func (me *Ctx) dispose() {
}
