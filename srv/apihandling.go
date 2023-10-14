package yosrv

import (
	"bytes"
	"io"
	"reflect"

	. "yo/ctx"
	yojson "yo/json"
	. "yo/util"
	"yo/util/str"
)

var api = ApiMethods{}

type ApiMethods map[string]ApiMethod

func Apis(all ApiMethods) {
	for method_path, method := range all {
		if api[method_path] != nil {
			panic("already a method registered for path '" + method_path + "'")
		}
		method.init(method_path)
		api[method_path] = method
	}
}

type apiHandleFunc func(*Ctx, any) any

type ApiMethod interface {
	init(string)
	handler() apiHandleFunc
	knownErrs() []Err
	loadPayload(data []byte) (any, error)
	reflTypes() (reflect.Type, reflect.Type)
	ApiPkgInfo
}

type ApiCtx[TIn any, TOut any] struct {
	Ctx  *Ctx
	Args *TIn
	Ret  *TOut
}

type ApiPkgInfo interface {
	PkgName() string
}

func Api[TIn any, TOut any](f func(*ApiCtx[TIn, TOut]), pkgInfo ApiPkgInfo, knownErrs ...Err) ApiMethod {
	var tmp_in TIn
	var tmp_out TOut
	if reflect.ValueOf(tmp_in).Kind() != reflect.Struct || reflect.ValueOf(tmp_out).Kind() != reflect.Struct {
		panic(str.Fmt("in/out types must be structs, got in:%T, out:%T", tmp_in, tmp_out))
	}
	var ret *apiMethod[TIn, TOut]
	ret = &apiMethod[TIn, TOut]{errs: knownErrs, PkgInfo: pkgInfo, handleFunc: func(ctx *Ctx, in any) any {
		ctx.Http.ApiErrs = ret.errs // *must* be that field, *not* the `knownErrs` local!
		var output TOut
		api_ctx := &ApiCtx[TIn, TOut]{Ctx: ctx, Args: in.(*TIn), Ret: &output}
		f(api_ctx)
		return api_ctx.Ret
	}}
	return ret
}

type apiMethod[TIn any, TOut any] struct {
	methodPath string
	handleFunc apiHandleFunc
	errs       []Err
	PkgInfo    ApiPkgInfo
}

func (me *apiMethod[TIn, TOut]) knownErrs() []Err       { return me.errs }
func (me *apiMethod[TIn, TOut]) handler() apiHandleFunc { return me.handleFunc }
func (me *apiMethod[TIn, TOut]) PkgName() string {
	if me.PkgInfo != nil {
		return me.PkgInfo.PkgName()
	}
	return ""
}
func (*apiMethod[TIn, TOut]) loadPayload(data []byte) (_ any, err error) {
	var it TIn
	if len(data) > 0 && !bytes.Equal(data, yojson.JsonNullTok) {
		err = yojson.Unmarshal(data, &it)
	}
	return &it, err
}
func (*apiMethod[TIn, TOut]) reflTypes() (reflect.Type, reflect.Type) {
	var tmp_in TIn
	var tmp_out TOut
	return reflect.ValueOf(tmp_in).Type(), reflect.ValueOf(tmp_out).Type()
}
func (me *apiMethod[TIn, TOut]) init(methodPath string) {
	me.methodPath = methodPath
	err_name_prefix := Err(str.Up0(methodPath))
	for i, err := range me.errs {
		me.errs[i] = err_name_prefix + err
	}
}

func apiHandleRequest(ctx *Ctx) (result any, handlerCalled bool) {
	ctx.Timings.Step("handler lookup")
	api := api[ctx.Http.UrlPath]
	if api == nil {
		ctx.HttpErr(404, "Not Found")
		return nil, false
	}

	ctx.Timings.Step("read req")
	payload_data, err := io.ReadAll(ctx.Http.Req.Body)
	if err != nil {
		ctx.HttpErr(500, err.Error())
		return nil, false
	}

	ctx.Timings.Step("parse req")
	payload, err := api.loadPayload(payload_data)
	if err != nil {
		ctx.HttpErr(400, err.Error()+If(IsDevMode, "\n"+string(payload_data), ""))
		return nil, false
	}

	// generic input sanitizations
	ctx.Timings.Step("sani payload")
	ReflWalk(reflect.ValueOf(payload), nil, true, func(path []any, it reflect.Value) {
		if it.Kind() == reflect.String {
			ReflSet(it, str.Trim(ReflGet[string](it)))
		}
	})

	ctx.Timings.Step("HANDLE")
	return api.handler()(ctx, payload), true
}
