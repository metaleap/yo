package yosrv

import (
	"bytes"
	"io"
	"reflect"

	. "yo/ctx"
	q "yo/db/query"
	yojson "yo/json"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

var (
	api          = ApiMethods{}
	KnownErrSets = map[string][]Err{
		"": {ErrTimedOut},
	}
)

type ApiMethods map[string]ApiMethod

func Apis(all ApiMethods) {
	for method_path, method := range all {
		if api[method_path] != nil {
			panic("already a method registered for path '" + method_path + "'")
		}
		api[method_path] = method
		method.init(method_path)
	}
}

type apiHandleFunc func(*Ctx, any) any

type ApiMethod interface {
	init(string)
	handler() apiHandleFunc
	loadPayload(data []byte) (any, error)
	reflTypes() (reflect.Type, reflect.Type)
	KnownErrs() []Err
	ApiPkgInfo
	WithKnownErrs(...Err) ApiMethod
	FailIf(rule q.Query, err Err) ApiMethod
}

type ApiCtx[TIn any, TOut any] struct {
	Ctx  *Ctx
	Args *TIn
	Ret  *TOut
}

type ApiPkgInfo interface {
	PkgName() string
}

func Api[TIn any, TOut any](f func(*ApiCtx[TIn, TOut]), pkgInfo ApiPkgInfo) ApiMethod {
	var tmp_in TIn
	var tmp_out TOut
	if reflect.ValueOf(tmp_in).Kind() != reflect.Struct || reflect.ValueOf(tmp_out).Kind() != reflect.Struct {
		panic(str.Fmt("in/out types must be structs, got in:%T, out:%T", tmp_in, tmp_out))
	}
	var ret *apiMethod[TIn, TOut]
	method := apiMethod[TIn, TOut]{PkgInfo: pkgInfo, failIfs: map[Err]q.Query{}, handleFunc: func(ctx *Ctx, in any) any {
		ctx.Http.ApiErrs = ret
		var output TOut
		api_ctx := &ApiCtx[TIn, TOut]{Ctx: ctx, Args: in.(*TIn), Ret: &output}
		f(api_ctx)
		return api_ctx.Ret
	}}
	ret = &method
	return ret
}

type apiMethod[TIn any, TOut any] struct {
	handleFunc apiHandleFunc
	errsOwn    []Err
	errsDeps   []string // methodPath refs to other methods
	failIfs    map[Err]q.Query
	PkgInfo    ApiPkgInfo
}

func (me *apiMethod[TIn, TOut]) handler() apiHandleFunc { return me.handleFunc }
func (me *apiMethod[TIn, TOut]) PkgName() string {
	if me.PkgInfo != nil {
		return me.PkgInfo.PkgName()
	}
	return ""
}
func (me *apiMethod[TIn, TOut]) KnownErrs() (ret []Err) {
	ret = append(me.errsOwn, KnownErrSets[""]...)
	for _, err_dep := range me.errsDeps {
		if method := api[err_dep]; method != nil {
			ret = append(ret, api[err_dep].KnownErrs()...)
		} else {
			ret = append(ret, sl.To(KnownErrSets[err_dep], func(it Err) Err { return Err(err_dep+"_") + it })...)
		}
	}
	return
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
	method_name := ToIdent(methodPath)
	err_name_prefix := Err(str.Up0(method_name)) + "_"
	for i, err := range me.errsOwn {
		me.errsOwn[i] = err_name_prefix + err
	}
}
func (me *apiMethod[TIn, TOut]) WithKnownErrs(knownErrs ...Err) ApiMethod {
	errs_own := sl.Where(knownErrs, func(it Err) bool { return it[0] != ':' })
	errs_deps := sl.To(sl.Where(knownErrs, func(it Err) bool { return it[0] == ':' }), func(it Err) string { return string(it)[1:] })
	me.errsOwn, me.errsDeps = append(me.errsOwn, errs_own...), append(me.errsDeps, errs_deps...)
	return me
}
func (me *apiMethod[TIn, TOut]) FailIf(rule q.Query, err Err) ApiMethod {
	if me.failIfs[err] != nil {
		panic("buggy FailIf call: already have a condition for err '" + string(err) + "'")
	}
	me.failIfs[err] = rule
	return me
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
