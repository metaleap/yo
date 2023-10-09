package api

import (
	"bytes"
	"reflect"

	. "yo/context"
	"yo/json"
	"yo/str"
	. "yo/util"
)

type APIMethods = map[string]APIMethod

var API = APIMethods{}

type apiHandleFunc = func(*Ctx, any) any

type APIMethod interface {
	handle() apiHandleFunc
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

type apiMethod[TIn any, TOut any] apiHandleFunc

func (me apiMethod[TIn, TOut]) handle() apiHandleFunc { return me }
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

func apiInit() {
	API["__/refl"] = InOut[Void, apiReflect](apiHandleRefl)
}
