package yo

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
)

var API = map[string]APIMethod{}

type apiHandleFunc = func(*Ctx, any) (any, error)

type APIMethod interface {
	handle() apiHandleFunc
	loadPayload(data []byte) (any, error)
}

func Method[TIn any, TOut any](f func(*Ctx, TIn) (TOut, error)) APIMethod {
	return apiMethod[TIn](func(ctx *Ctx, in any) (any, error) {
		return f(ctx, in.(TIn))
	})
}

type apiMethod[T any] apiHandleFunc

func (me apiMethod[T]) handle() apiHandleFunc { return me }
func (me apiMethod[T]) loadPayload(data []byte) (any, error) {
	var it T
	err := json.Unmarshal(data, &it)
	return &it, err
}

func ListenAndServe() {
	log.Fatal(http.ListenAndServe(":"+iToA(cfg.YO_API_HTTP_PORT), http.HandlerFunc(handleHTTPRequest)))
}

func handleHTTPRequest(rw http.ResponseWriter, req *http.Request) {
	ctx := ctxNew(req)
	defer ctx.dispose()

	path := strTrimL(req.URL.Path, "/")
	api := API[path]
	if api == nil {
		http.Error(rw, "Not Found", 404)
		return
	}

	payload_data, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	payload, err := api.loadPayload(payload_data)
	if err != nil {
		http.Error(rw, err.Error(), 400)
		return
	}

	result, err := api.handle()(ctx, payload)
	if err != nil {
		_, is_app_err := err.(Err)
		http.Error(rw, err.Error(), If(is_app_err, 400, 500))
		return
	}

	resp_data, err := json.Marshal(result)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Content-Length", strconv.Itoa(len(resp_data)))
	_, _ = rw.Write(resp_data)
}
