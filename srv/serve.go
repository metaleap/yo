package yosrv

import (
	"crypto/subtle"
	"io/fs"
	"net/http"
	"reflect"

	. "yo/cfg"
	yoctx "yo/ctx"
	yojson "yo/json"
	yolog "yo/log"
	. "yo/util"
	"yo/util/str"
)

const yoAdminUrlPrefix = "__/yo/"
const QueryArgForceFail = "yoFail"
const StaticFilesDirNameYo = "__yostatic"

var StaticFilesDirNameApp string
var StaticFileDirYo fs.FS
var StaticFileDirApp fs.FS

type PreServe struct {
	Name string
	Do   func(*yoctx.Ctx)
}

var (
	// requests to key+'/' will be served from the corresponding FS
	apiStdRespHeaders = str.Dict{
		"Content-Type":           "application/json",
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "no-store",
	}

	// funcs are run (in no particular order) just prior to loading request payload, handling request, and serving response
	PreServes = []PreServe{
		{"authAdmin", If(IsDevMode, nil, authAdmin)},
	}

	codegenMaybe func() = nil // overwritten by apisdkgen.go in debug build mode
)

func InitAndMaybeCodegen(dbStructs []reflect.Type) func() {
	apiReflAllDbStructs = dbStructs
	Apis(ApiMethods{
		yoAdminUrlPrefix + "refl": Api(apiHandleReflReq, nil),
	})
	for method_path := range api {
		if (str.Trim(method_path) != method_path) || (method_path == "") || !str.IsPrtAscii(method_path) {
			panic("not a valid method path: '" + method_path + "'")
		}
	}
	if codegenMaybe != nil {
		codegenMaybe()
	}
	return listenAndServe
}

func listenAndServe() {
	yolog.Println("live @ port %d", Cfg.YO_API_HTTP_PORT)
	panic(http.ListenAndServe(":"+str.FromInt(Cfg.YO_API_HTTP_PORT), http.HandlerFunc(handleHTTPRequest)))
}

func handleHTTPRequest(rw http.ResponseWriter, req *http.Request) {
	ctx := yoctx.NewForHttp(req, rw, Cfg.YO_API_IMPL_TIMEOUT)
	defer ctx.OnDone(nil)

	ctx.Timings.Step("check " + QueryArgForceFail)
	if s := ctx.GetStr(QueryArgForceFail); s != "" {
		code, _ := str.ToInt(s)
		ctx.HttpErr(If(code == 0, 500, code), "forced error via query-string param '"+QueryArgForceFail+"'")
		return
	}

	for _, pre_serve := range PreServes {
		if pre_serve.Do != nil {
			ctx.Timings.Step("pre:" + pre_serve.Name)
			pre_serve.Do(ctx)
		}
	}

	ctx.Timings.Step("static check")
	var static_fs fs.FS
	if static_prefix := StaticFilesDirNameYo + "/"; str.Begins(ctx.Http.UrlPath, static_prefix) && (ctx.Http.UrlPath != static_prefix) {
		static_fs = StaticFileDirYo
	} else if (StaticFileDirApp != nil) && (StaticFilesDirNameApp != "") && str.Begins(ctx.Http.UrlPath, StaticFilesDirNameApp+"/") {
		static_fs = StaticFileDirApp
	}
	if static_fs != nil {
		ctx.Timings.Step("static serve")
		ctx.HttpOnPreWriteResponse()
		http.FileServer(http.FS(static_fs)).ServeHTTP(rw, req)
		return
	}

	// no static content was requested or served, so it's an api call
	if result, handler_called := apiHandleRequest(ctx); handler_called { // if not, `apiHandleRequest` did `http.Error()` and nothing more to do here
		ctx.Timings.Step("jsonify resp")
		resp_data, err := yojson.MarshalIndent(result, "", "  ")
		if err != nil {
			ctx.HttpErr(500, err.Error())
			return
		}

		ctx.Timings.Step("write resp")
		for k, v := range apiStdRespHeaders {
			rw.Header().Set(k, v)
		}
		rw.Header().Set("Content-Length", str.FromInt(len(resp_data)))
		ctx.HttpOnPreWriteResponse()
		_, _ = rw.Write(resp_data)
	}
}

func authAdmin(ctx *yoctx.Ctx) {
	if !(str.Begins(ctx.Http.UrlPath, yoAdminUrlPrefix) || str.Begins(ctx.Http.UrlPath, StaticFilesDirNameYo+"/yo.")) {
		return
	}
	user, pwd, ok := ctx.Http.Req.BasicAuth()
	if ok {
		ok = (1 == subtle.ConstantTimeCompare([]byte(Cfg.YO_API_ADMIN_USER), []byte(user))) &&
			(1 == subtle.ConstantTimeCompare([]byte(Cfg.YO_API_ADMIN_PWD), []byte(pwd)))
	}
	if !ok {
		ctx.Http.Resp.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		panic(yoctx.ErrMustBeAdmin)
	}
}
