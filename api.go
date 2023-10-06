package yo

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"reflect"
	"strconv"
)

type Void struct{}

type APIMethods = map[string]APIMethod

var API = APIMethods{}

type apiHandleFunc = func(*Ctx, any) (any, error)

type APIMethod interface {
	handle() apiHandleFunc
	loadPayload(data []byte) (any, error)
}

func InOut[TIn any, TOut any](f func(*Ctx, *TIn, *TOut) error) APIMethod {
	var tmp_in TIn
	var tmp_out TOut
	if reflect.ValueOf(tmp_in).Kind() != reflect.Struct || reflect.ValueOf(tmp_out).Kind() != reflect.Struct {
		panic(strFmt("in/out types must be structs, got in:%T, out:%T", tmp_in, tmp_out))
	}
	return apiMethod[TIn](func(ctx *Ctx, in any) (any, error) {
		var out TOut
		err := f(ctx, in.(*TIn), &out)
		return &out, err
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

func apiInit() {
	API["__/refl"] = InOut[Void, apiReflect](apiHandleRefl)
}

type apiReflect struct {
	Types   map[string]map[string]string
	Methods []apiReflectMethod
}

type apiReflectMethod struct {
	Path string
	In   string
	Out  string
}

func apiHandleRefl(it *Ctx, in *Void, out *apiReflect) error {
	for methodPath := range API {
		out.Methods = append(out.Methods, apiReflectMethod{Path: methodPath})
	}
	return nil
}

func apiRefl(ctx *apiReflect) {

}
