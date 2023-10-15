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
	}
}

type apiHandleFunc func(*Ctx, any) any

type ApiMethod interface {
	ApiPkgInfo
	handler() apiHandleFunc
	loadPayload(data []byte) (any, error)
	validatePayload(any) (q.Query, Err)
	reflTypes() (reflect.Type, reflect.Type)
	failsIf() []Fails
	methodNameUp0() string
	KnownErrs() []Err
	CouldFailWith(...Err) ApiMethod
}

type ApiCtx[TIn any, TOut any] struct {
	Ctx  *Ctx
	Args *TIn
	Ret  *TOut
}

type ApiPkgInfo interface {
	PkgName() string
}

func Api[TIn any, TOut any](f func(*ApiCtx[TIn, TOut]), pkgInfo ApiPkgInfo, failIfs ...Fails) ApiMethod {
	var tmp_in TIn
	var tmp_out TOut
	if reflect.ValueOf(tmp_in).Kind() != reflect.Struct || reflect.ValueOf(tmp_out).Kind() != reflect.Struct {
		panic(str.Fmt("in/out types must be structs, got in:%T, out:%T", tmp_in, tmp_out))
	}
	var ret *apiMethod[TIn, TOut]
	method := apiMethod[TIn, TOut]{PkgInfo: pkgInfo, handleFunc: func(ctx *Ctx, in any) any {
		ctx.Http.ApiErrs = ret
		var output TOut
		api_ctx := &ApiCtx[TIn, TOut]{Ctx: ctx, Args: in.(*TIn), Ret: &output}
		f(api_ctx)
		return api_ctx.Ret
	}}
	ret = &method
	for _, fail := range failIfs {
		ret.errsOwn = sl.With(ret.errsOwn, fail.Err)
		if sl.IdxWhere(ret.failIfs, func(it Fails) bool { return (it.Err == fail.Err) }) > 0 {
			panic("duplicate Err '" + string(fail.Err) + "' in `failIfs`")
		}
		ret.failIfs = append(ret.failIfs, fail)
	}
	return ret
}

type Fails struct {
	Err Err
	If  q.Query
}

func FailsOf(methodPaths ...string) (ret []Fails) {
	for _, method_path := range methodPaths {
		ret = append(ret, api[method_path].failsIf()...)
	}
	return
}

type apiMethod[TIn any, TOut any] struct {
	handleFunc apiHandleFunc
	errsOwn    []Err
	errsDeps   []string // methodPath refs to other methods
	failIfs    []Fails
	PkgInfo    ApiPkgInfo
}

func (me *apiMethod[TIn, TOut]) failsIf() []Fails       { return me.failIfs }
func (me *apiMethod[TIn, TOut]) handler() apiHandleFunc { return me.handleFunc }
func (me *apiMethod[TIn, TOut]) PkgName() string {
	if me.PkgInfo != nil {
		return me.PkgInfo.PkgName()
	}
	return ""
}
func (me *apiMethod[TIn, TOut]) methodPath() (ret string) {
	for path, method := range api {
		if method == me {
			ret = path
		}
	}
	if ret == "" {
		panic("unregistered ApiMethod")
	}
	return
}
func (me *apiMethod[TIn, TOut]) methodNameUp0() (ret string) {
	return str.Up0(ToIdent(me.methodPath()))
}
func (me *apiMethod[TIn, TOut]) KnownErrs() (ret []Err) {
	method_name := me.methodNameUp0()
	err_name_prefix := Err(str.Up0(method_name)) + "_"

	ret = append(sl.To(me.errsOwn, func(it Err) Err { return err_name_prefix + it }), KnownErrSets[""]...)
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
func (me *apiMethod[TIn, TOut]) validatePayload(it any) (q.Query, Err) {
	do_check := func(method ApiMethod, check *Fails) (q.Query, Err) {
		method_name := method.methodNameUp0()
		err_name_prefix := str.Up0(method_name) + "_"
		if failed_condition := check.If.Not().Eval(it, nil); failed_condition != nil {
			return failed_condition, Err(err_name_prefix) + check.Err
		}
		return nil, ""
	}
	for _, dep := range me.errsDeps {
		if method := api[dep]; method != nil {
			for _, check := range method.failsIf() {
				if failed_condition, err := do_check(api[dep], &check); failed_condition != nil {
					return failed_condition, err
				}
			}
		}
	}
	for i := range me.failIfs {
		if failed_condition, err := do_check(me, &me.failIfs[i]); failed_condition != nil {
			return failed_condition, err
		}
	}
	return nil, ""
}
func (*apiMethod[TIn, TOut]) reflTypes() (reflect.Type, reflect.Type) {
	var tmp_in TIn
	var tmp_out TOut
	return reflect.ValueOf(tmp_in).Type(), reflect.ValueOf(tmp_out).Type()
}
func (me *apiMethod[TIn, TOut]) CouldFailWith(knownErrs ...Err) ApiMethod {
	errs_own := sl.Where(knownErrs, func(it Err) bool { return it[0] != ':' })
	errs_deps := sl.To(sl.Where(knownErrs, func(it Err) bool { return it[0] == ':' }), func(it Err) string { return string(it)[1:] })
	me.errsOwn, me.errsDeps = sl.With(me.errsOwn, errs_own...), sl.With(me.errsDeps, errs_deps...)
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

	ctx.Timings.Step("sani payload")
	ReflWalk(reflect.ValueOf(payload), nil, true, func(path []any, it reflect.Value) {
		if it.Kind() == reflect.String {
			ReflSet(it, str.Trim(ReflGet[string](it)))
		}
	})

	ctx.Timings.Step("validate req")
	_, err_validation := api.validatePayload(payload)
	if err_validation != "" {
		ctx.HttpErr(400, err_validation.Error())
		return nil, false
	}

	ctx.Timings.Step("HANDLE")
	return api.handler()(ctx, payload), true
}
