package yosrv

import (
	"bytes"
	"io"
	"reflect"

	. "yo/cfg"
	. "yo/ctx"
	q "yo/db/query"
	yojson "yo/json"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

const (
	QueryArgValidateOnly                   = "yoValiOnly"
	QueryArgNoCtxPrt                       = "yoNoCtxPrt"
	ErrUnauthorized                    Err = "Unauthorized"
	ErrMissingOrExcessiveContentLength Err = "MissingOrExcessiveContentLength"
	yoAdminApisUrlPrefix                   = "__/yo/"
	apisContentType                        = "application/json"
)

var (
	api             = ApiMethods{}
	AppApiUrlPrefix = ""
	KnownErrSets    = map[string][]Err{
		"": {ErrTimedOut, ErrMissingOrExcessiveContentLength},
	}
	ErrsNoPrefix  = errsNoCodegen
	errsNoCodegen = []Err{ErrTimedOut, ErrMissingOrExcessiveContentLength, ErrUnauthorized, ErrDbUpdExpectedIdGt0, ErrMustBeAdmin}
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
	pkgInfo() ApiPkgInfo
	handler() apiHandleFunc
	loadPayload(data []byte) (any, error)
	validatePayload(any) (q.Query, Err)
	reflTypes() (reflect.Type, reflect.Type)
	failsIf() []Fails
	methodPath() string
	methodNameUp0() string
	isMultipartForm() bool
	IsMultipartForm() ApiMethod
	From(ApiPkgInfo) ApiMethod
	KnownErrs() []Err
	Checks(...Fails) ApiMethod
	CouldFailWith(...Err) ApiMethod
	FailIf(func(*Ctx) bool, Err) ApiMethod
}

type ApiCtx[TIn any, TOut any] struct {
	Ctx  *Ctx
	Args *TIn
	Ret  *TOut
}

func Do[TIn any, TOut any](f func(*ApiCtx[TIn, TOut]), ctx *Ctx, args *TIn) *TOut {
	var ret TOut
	api_ctx := &ApiCtx[TIn, TOut]{Ctx: ctx, Args: args, Ret: &ret}
	f(api_ctx)
	return api_ctx.Ret
}

type ApiPkgInfo interface {
	PkgName() string
}

func Api[TIn any, TOut any](f func(*ApiCtx[TIn, TOut]), failIfs ...Fails) ApiMethod {
	var ret apiMethod[TIn, TOut]
	method[TIn, TOut](f, &ret)
	ret.Checks(failIfs...)
	return &ret
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
	handleFunc    apiHandleFunc
	errsOwn       []Err
	errsDeps      []string // methodPath refs to other methods
	failIfs       []Fails
	preChecks     []Pair[Err, func(*Ctx) bool]
	multipartForm bool
	PkgInfo       ApiPkgInfo
}

func (me *apiMethod[TIn, TOut]) pkgInfo() ApiPkgInfo    { return me.PkgInfo }
func (me *apiMethod[TIn, TOut]) failsIf() []Fails       { return me.failIfs }
func (me *apiMethod[TIn, TOut]) handler() apiHandleFunc { return me.handleFunc }
func (me *apiMethod[TIn, TOut]) From(pkgInfo ApiPkgInfo) ApiMethod {
	me.PkgInfo = pkgInfo
	return me
}
func (me *apiMethod[TIn, TOut]) isMultipartForm() bool { return me.multipartForm }
func (me *apiMethod[TIn, TOut]) IsMultipartForm() ApiMethod {
	me.multipartForm = true
	return me
}
func (me *apiMethod[TIn, TOut]) Checks(failIfs ...Fails) ApiMethod {
	for _, fail := range failIfs {
		me.errsOwn = sl.With(me.errsOwn, fail.Err)
		if sl.HasWhere(me.failIfs, func(it Fails) bool { return (it.Err == fail.Err) }) {
			panic("duplicate Err '" + string(fail.Err) + "' in `failIfs`")
		}
		me.failIfs = append(me.failIfs, fail)
	}
	return me
}
func (me *apiMethod[TIn, TOut]) FailIf(inCaseOf func(*Ctx) bool, withErr Err) ApiMethod {
	me.CouldFailWith(withErr)
	me.preChecks = append(me.preChecks, Pair[Err, func(*Ctx) bool]{withErr, inCaseOf})
	return me
}
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
func (me *apiMethod[TIn, TOut]) methodNameUp0() string {
	return str.Up0(ToIdent(me.methodPath()))
}
func (me *apiMethod[TIn, TOut]) KnownErrs() (ret []Err) {
	method_name := me.methodNameUp0()
	err_name_prefix := Err(str.Up0(method_name)) + "_"

	ret = append(sl.To(me.errsOwn, func(it Err) Err { return If(sl.Has(ErrsNoPrefix, it), it, err_name_prefix+it) }),
		KnownErrSets[""]...)
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
	if len(data) > 0 && !bytes.Equal(data, yojson.JsonTokNull) {
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
	// commented-out for now because: running input checks of deps is usually senseless since api input types, or validation semantics, differ more often than not
	// for _, dep := range me.errsDeps {
	// 	if method := api[dep]; method != nil {
	// 		for _, check := range method.failsIf() {
	// 			if failed_condition, err := do_check(api[dep], &check); failed_condition != nil {
	// 				return failed_condition, err
	// 			}
	// 		}
	// 	}
	// }
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

func method[TIn any, TOut any](f func(*ApiCtx[TIn, TOut]), ret *apiMethod[TIn, TOut]) {
	var tmp_in TIn
	var tmp_out TOut
	if IsDevMode && (reflect.ValueOf(tmp_in).Kind() != reflect.Struct || reflect.ValueOf(tmp_out).Kind() != reflect.Struct) {
		panic(str.Fmt("in/out types must both be structs, got %T and %T", tmp_in, tmp_out))
	}
	*ret = apiMethod[TIn, TOut]{
		handleFunc: func(ctx *Ctx, in any) any {
			ctx.Http.ApiMethod = ret
			for _, fail_check := range ret.preChecks {
				if fail_check.It(ctx) {
					panic(fail_check.Key)
				}
			}
			var output TOut
			api_ctx := &ApiCtx[TIn, TOut]{Ctx: ctx, Args: in.(*TIn), Ret: &output}
			f(api_ctx)
			return api_ctx.Ret
		}}
}

func apiHandleRequest(ctx *Ctx) (result any, handlerCalled bool) {
	if ctx.GetStr(QueryArgNoCtxPrt) != "" {
		ctx.TimingsNoPrintInDevMode = true
	}

	ctx.Timings.Step("handler lookup")
	var api_method ApiMethod
	method_path := ctx.Http.UrlPath
	if (AppApiUrlPrefix == "") || str.Begins(method_path, yoAdminApisUrlPrefix) {
		api_method = api[ctx.Http.UrlPath]
	} else if str.Begins(method_path, AppApiUrlPrefix) {
		api_method = api[ctx.Http.UrlPath[len(AppApiUrlPrefix):]]
	}
	if api_method == nil {
		ctx.HttpErr(404, "Not Found")
		return
	}

	max_payload_size := (1024 * 1024 * int64(Cfg.YO_API_MAX_REQ_CONTENTLENGTH_MB))
	if (ctx.Http.Req.ContentLength < 0) || (ctx.Http.Req.ContentLength > max_payload_size) {
		ctx.HttpErr(406, string(ErrMissingOrExcessiveContentLength))
		return
	}

	var payload_data []byte
	var err error
	if api_method.isMultipartForm() {
		if err = ctx.Http.Req.ParseMultipartForm(ctx.Http.Req.ContentLength); err != nil {
			ctx.HttpErr(400, err.Error())
			return
		}
		defer ctx.Http.Req.MultipartForm.RemoveAll()
		payload_data = []byte(ctx.Http.Req.MultipartForm.Value["_"][0])
	} else {
		ctx.Timings.Step("read req")
		payload_data, err = io.ReadAll(ctx.Http.Req.Body)
		if err != nil {
			ctx.HttpErr(500, err.Error())
			return
		}
	}

	ctx.Timings.Step("parse req")
	payload, err := api_method.loadPayload(payload_data)
	if err != nil {
		ctx.HttpErr(400, err.Error()+If(IsDevMode, "\n"+string(payload_data), ""))
		return
	}

	ctx.Timings.Step("sani payload")
	ReflWalk(reflect.ValueOf(payload), nil, true, func(path []any, it reflect.Value) {
		if it.Kind() == reflect.String {
			ReflSet(it, str.Trim(ReflGet[string](it)))
		}
	})

	ctx.Timings.Step("validate req")
	failed_condition, err_validation := api_method.validatePayload(payload)
	if err_validation != "" {
		if IsDevMode {
			println(">>>FAILCOND>>>" + q.SqlReprForDebugging(failed_condition))
		}
		ctx.HttpErr(err_validation.HttpStatusCodeOr(400), err_validation.Error())
		return
	}

	if ctx.GetStr(QueryArgValidateOnly) != "" {
		return
	}

	ctx.Timings.Step("call handler")
	return api_method.handler()(ctx, payload), true
}
