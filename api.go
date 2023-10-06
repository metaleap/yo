package yo

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"reflect"
)

type APIMethods = map[string]APIMethod

var API = APIMethods{}

type apiHandleFunc = func(*Ctx, any) (any, error)

type APIMethod interface {
	handle() apiHandleFunc
	loadPayload(data []byte) (any, error)
	reflTypes() (reflect.Type, reflect.Type)
}

func InOut[TIn any, TOut any](f func(*Ctx, *TIn, *TOut) error) APIMethod {
	var tmp_in TIn
	var tmp_out TOut
	if reflect.ValueOf(tmp_in).Kind() != reflect.Struct || reflect.ValueOf(tmp_out).Kind() != reflect.Struct {
		panic(strFmt("in/out types must be structs, got in:%T, out:%T", tmp_in, tmp_out))
	}
	return apiMethod[TIn, TOut](func(ctx *Ctx, in any) (any, error) {
		var output TOut
		input, _ := in.(*TIn)
		err := f(ctx, input, &output)
		return &output, err
	})
}

type apiMethod[TIn any, TOut any] apiHandleFunc

func (me apiMethod[TIn, TOut]) handle() apiHandleFunc { return me }
func (me apiMethod[TIn, TOut]) loadPayload(data []byte) (any, error) {
	if len(data) == 0 || bytes.Equal(data, jsonNullTok) {
		return nil, nil
	}
	var it TIn
	err := json.Unmarshal(data, &it)
	return &it, err
}
func (me apiMethod[TIn, TOut]) reflTypes() (reflect.Type, reflect.Type) {
	var tmp_in TIn
	var tmp_out TOut
	return reflect.ValueOf(tmp_in).Type(), reflect.ValueOf(tmp_out).Type()
}

func apiInit() {
	API["__/refl"] = InOut[Void, apiReflect](apiHandleRefl)
}

func ListenAndServe() {
	log.Fatal(http.ListenAndServe(":"+iToA(cfg.YO_API_HTTP_PORT), http.HandlerFunc(handleHTTPRequest)))
}

func handleHTTPRequest(rw http.ResponseWriter, req *http.Request) {
	if !IsDebugMode { // want stack traces in local dev
		defer func() {
			if crashed := recover(); crashed != nil {
				http.Error(rw, strFmt("%v", crashed), 500)
			}
		}()
	}
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

	resp_data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Content-Length", iToA(len(resp_data)))
	_, _ = rw.Write(resp_data)
}
