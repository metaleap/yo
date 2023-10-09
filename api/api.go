package api

import (
	"bytes"
	"io"
	"reflect"

	. "yo/context"
	"yo/json"
	"yo/str"
	. "yo/util"
)

type Methods = map[string]APIMethod

var API = Methods{}

type handleFunc = func(*Ctx, any) any

type APIMethod interface {
	handle() handleFunc
	loadPayload(data []byte) (any, error)
	reflTypes() (reflect.Type, reflect.Type)
}

func InOut[TIn any, TOut any](f func(*Ctx, *TIn, *TOut)) APIMethod {
	var tmp_in TIn
	var tmp_out TOut
	if reflect.ValueOf(tmp_in).Kind() != reflect.Struct || reflect.ValueOf(tmp_out).Kind() != reflect.Struct {
		panic(str.Fmt("in/out types must be structs, got in:%T, out:%T", tmp_in, tmp_out))
	}
	return apiMethod[TIn, TOut](func(ctx *Ctx, in any) any {
		var output TOut
		input, _ := in.(*TIn)
		f(ctx, input, &output)
		return &output
	})
}

type apiMethod[TIn any, TOut any] handleFunc

func (me apiMethod[TIn, TOut]) handle() handleFunc { return me }
func (me apiMethod[TIn, TOut]) loadPayload(data []byte) (any, error) {
	if len(data) == 0 || bytes.Equal(data, json.JsonNullTok) {
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

func Init() (func(), func(*Ctx) (any, bool)) {
	API["__/refl"] = InOut[Void, refl](handleReflReq)
	return If(IsDevMode, genSdk, nil), handle
}

func handle(ctx *Ctx) (any, bool) {
	ctx.Timings.Step("handler lookup")
	api := API[ctx.UrlPath]
	if api == nil {
		ctx.HttpErr(404, "Not Found")
		return nil, false
	}

	ctx.Timings.Step("read req")
	payload_data, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		ctx.HttpErr(500, err.Error())
		return nil, false
	}

	ctx.Timings.Step("unmarshal req")
	payload, err := api.loadPayload(payload_data)
	if err != nil {
		ctx.HttpErr(400, err.Error()+If(IsDevMode, "\n"+string(payload_data), ""))
		return nil, false
	}

	ctx.Timings.Step("handle req")
	return api.handle()(ctx, payload), true
}
