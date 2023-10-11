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

type APIMethods = map[string]APIMethod

var API = APIMethods{}

type apiHandleFunc = func(*Ctx, any) any

type APIMethod interface {
	handle() apiHandleFunc
	loadPayload(data []byte) (any, error)
	reflTypes() (reflect.Type, reflect.Type)
}

func Method[TIn any, TOut any](f func(*Ctx, *TIn, *TOut) any) APIMethod {
	var tmp_in TIn
	var tmp_out TOut
	if reflect.ValueOf(tmp_in).Kind() != reflect.Struct || reflect.ValueOf(tmp_out).Kind() != reflect.Struct {
		panic(str.Fmt("in/out types must be structs, got in:%T, out:%T", tmp_in, tmp_out))
	}
	return apiMethod[TIn, TOut](func(ctx *Ctx, in any) any {
		var output TOut
		return f(ctx, in.(*TIn), &output)
	})
}

type apiMethod[TIn any, TOut any] apiHandleFunc

func (me apiMethod[TIn, TOut]) handle() apiHandleFunc { return me }
func (me apiMethod[TIn, TOut]) loadPayload(data []byte) (any, error) {
	if len(data) == 0 || bytes.Equal(data, yojson.JsonNullTok) {
		return nil, nil
	}
	var it TIn
	err := yojson.Unmarshal(data, &it)
	return &it, err
}
func (me apiMethod[TIn, TOut]) reflTypes() (reflect.Type, reflect.Type) {
	var tmp_in TIn
	var tmp_out TOut
	return reflect.ValueOf(tmp_in).Type(), reflect.ValueOf(tmp_out).Type()
}

func apiHandleRequest(ctx *Ctx) (result any, handlerCalled bool) {
	ctx.Timings.Step("handler lookup")
	api := API[ctx.Http.UrlPath]
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

	ctx.Timings.Step("HANDLE")
	return api.handle()(ctx, payload), true
}