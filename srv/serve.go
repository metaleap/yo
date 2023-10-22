package yosrv

import (
	"crypto/subtle"
	"io/fs"
	"net/http"
	"reflect"
	"time"

	. "yo/cfg"
	yoctx "yo/ctx"
	yojson "yo/json"
	yolog "yo/log"
	. "yo/util"
	"yo/util/str"
)

const QueryArgForceFail = "yoFail"
const QueryArgForceUser = "yoUser"
const StaticFilesDirNameYo = "__yostatic"
const StaticFilesDirNameApp = "__static"

var StaticFileDirYo fs.FS
var StaticFileDirApp fs.FS

type Middleware struct {
	Name string
	Do   func(*yoctx.Ctx)
}

var (
	detectEnumsAndMaybeCodegen func() = nil // overwritten by codegen_apistuff.go in debug build mode

	// requests to key+'/' will be served from the corresponding FS
	apiStdRespHeaders = str.Dict{
		"Content-Type":           apisContentType,
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "no-store",
	}

	// PreServes funcs are run at the start of the request handling, prior to any other request processing
	PreServes = []Middleware{{"authAdmin", If(IsDevMode, nil, authAdmin)}}

	// PreHandling funcs are run just prior to loading api request payload, handling that request, and serving the handler's response
	PreApiHandling = []Middleware{}

	// set by app for home and vanity-URL (non-file, non-API) requests
	AppSideStaticRePathFor func(string) string
)

func InitAndMaybeCodegen(dbStructs []reflect.Type) func() {
	apiReflAllDbStructs = dbStructs
	for method_path := range api {
		if (str.Trim(method_path) != method_path) || (method_path == "") || !str.IsPrtAscii(method_path) {
			panic("not a valid method path: '" + method_path + "'")
		}
	}
	if detectEnumsAndMaybeCodegen != nil {
		detectEnumsAndMaybeCodegen()
	}
	return listenAndServe
}

func listenAndServe() {
	yolog.Println("live @ port %d", Cfg.YO_API_HTTP_PORT)
	panic(http.ListenAndServe(":"+str.FromInt(Cfg.YO_API_HTTP_PORT), http.HandlerFunc(handleHTTPRequest)))
}

func handleHTTPRequest(rw http.ResponseWriter, req *http.Request) {
	ctx := yoctx.NewCtxForHttp(req, rw, Cfg.YO_API_IMPL_TIMEOUT)
	defer ctx.OnDone(nil)

	if IsDevMode {
		if s := ctx.GetStr(QueryArgForceFail); s != "" {
			code, _ := str.ToInt(s)
			ctx.HttpErr(If(code == 0, 500, code), "forced error via query-string param '"+QueryArgForceFail+"'")
			return
		} else if s = ctx.GetStr(QueryArgForceUser); s != "" {
			ctx.Set(yoctx.CtxKeyForcedTestUser, s)
		}
	}

	for _, pre_serve := range PreServes {
		if pre_serve.Do != nil {
			ctx.Timings.Step("pre:" + pre_serve.Name)
			pre_serve.Do(ctx)
		}
	}

	ctx.Timings.Step("static check")
	if (AppSideStaticRePathFor != nil) && !str.Begins(ctx.Http.UrlPath, "_") {
		if re_path := AppSideStaticRePathFor(ctx.Http.UrlPath); (re_path != "") && (re_path != ctx.Http.UrlPath) {
			ctx.Http.UrlPath = re_path
			req.RequestURI = "/" + re_path // loses query-args, which aren't expected for purely static content anyway
			req.URL.Path = "/" + re_path
		}
	}
	var static_fs fs.FS
	if static_prefix := StaticFilesDirNameYo + "/"; str.Begins(ctx.Http.UrlPath, static_prefix) && (ctx.Http.UrlPath != static_prefix) {
		static_fs = StaticFileDirYo
	} else if (StaticFileDirApp != nil) && (StaticFilesDirNameApp != "") && str.Begins(ctx.Http.UrlPath, StaticFilesDirNameApp+"/") {
		static_fs = StaticFileDirApp
	}
	if static_fs != nil {
		ctx.Timings.Step("static serve")
		ctx.HttpOnPreWriteResponse()
		httpFileServer(http.FileServer(http.FS(static_fs))).ServeHTTP(rw, req)
		return
	}

	for _, pre_serve := range PreApiHandling {
		if pre_serve.Do != nil {
			ctx.Timings.Step("pre:" + pre_serve.Name)
			pre_serve.Do(ctx)
		}
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
	if !(str.Begins(ctx.Http.UrlPath, yoAdminApisUrlPrefix) || str.Begins(ctx.Http.UrlPath, StaticFilesDirNameYo+"/yo.")) {
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

var httpDevModeUncachedFileServingResponseHeaders = map[string]string{"Expires": time.Unix(0, 0).Format(time.RFC1123), "Cache-Control": "no-cache, private, max-age=0", "Pragma": "no-cache", "X-Accel-Expires": "0"}
var httpDevModeUncachedFileServingRequestHeaders = []string{"ETag", "If-Modified-Since", "If-Match", "If-None-Match", "If-Range", "If-Unmodified-Since"}

// stackoverflow.com/a/33881296
func httpFileServer(fileServingHandler http.Handler) http.Handler {
	if !IsDevMode {
		return fileServingHandler
	}
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		for _, etag_header_name := range httpDevModeUncachedFileServingRequestHeaders {
			if req.Header.Get(etag_header_name) != "" {
				req.Header.Del(etag_header_name)
			}
		}
		for k, v := range httpDevModeUncachedFileServingResponseHeaders {
			rw.Header().Set(k, v)
		}
		fileServingHandler.ServeHTTP(rw, req)
	})
}
