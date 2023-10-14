package yoserve

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
	for k, v := range all {
		api[k] = v
	}
}

type apiHandleFunc = func(*Ctx, any) any

type ApiMethod interface {
	handle() apiHandleFunc
	loadPayload(data []byte) (any, error)
	reflTypes() (reflect.Type, reflect.Type)
}

type ApiCtx[TIn any, TOut any] struct {
	Ctx  *Ctx
	Args *TIn
	Ret  *TOut
}

func Api[TIn any, TOut any](f func(*ApiCtx[TIn, TOut])) ApiMethod {
	var tmp_in TIn
	var tmp_out TOut
	if reflect.ValueOf(tmp_in).Kind() != reflect.Struct || reflect.ValueOf(tmp_out).Kind() != reflect.Struct {
		panic(str.Fmt("in/out types must be structs, got in:%T, out:%T", tmp_in, tmp_out))
	}
	return apiMethod[TIn, TOut](func(ctx *Ctx, in any) any {
		var output TOut
		var input *TIn
		if in != nil { // could shorten to `input, _ := in.(*TIn)` but want to panic below in case of new bugs
			input = in.(*TIn)
		}
		api_ctx := &ApiCtx[TIn, TOut]{Ctx: ctx, Args: input, Ret: &output}
		f(api_ctx)
		return api_ctx.Ret
	})
}

type apiMethod[TIn any, TOut any] apiHandleFunc

func (me apiMethod[TIn, TOut]) handle() apiHandleFunc { return me }
func (me apiMethod[TIn, TOut]) loadPayload(data []byte) (_ any, err error) {
	var it TIn
	if len(data) > 0 && !bytes.Equal(data, yojson.JsonNullTok) {
		err = yojson.Unmarshal(data, &it)
	}
	return &it, err
}
func (me apiMethod[TIn, TOut]) reflTypes() (reflect.Type, reflect.Type) {
	var tmp_in TIn
	var tmp_out TOut
	return reflect.ValueOf(tmp_in).Type(), reflect.ValueOf(tmp_out).Type()
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
	return api.handle()(ctx, payload), true
}
