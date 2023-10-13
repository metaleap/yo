package yoserve

import (
	"io/fs"
	"net/http"
	"os"
	"reflect"

	. "yo/cfg"
	yoctx "yo/ctx"
	yojson "yo/json"
	yolog "yo/log"
	. "yo/util"
	"yo/util/str"
)

var (
	StaticFileServes        = map[string]fs.FS{}
	apiGenSdkMaybe   func() = nil // overwritten by apisdkgen.go in debug build mode
	PreServe         []func(*yoctx.Ctx)
)

const StaticFilesDirName = "__yostatic"

// called from yo.Init, not user code
func Init(dbStructs []reflect.Type) (func(), func()) {
	apiReflAllDbStructs = dbStructs
	API["__/refl"] = Method(apiHandleReflReq)
	for method_path := range API {
		if str.Trim(method_path) != method_path || method_path == "" || !str.IsPrtAscii(method_path) {
			panic("not a valid method path: '" + method_path + "'")
		}
	}
	return apiGenSdkMaybe, listenAndServe
}

func listenAndServe() {
	StaticFileServes["__yo"] = os.DirFS("../yo/" + StaticFilesDirName)
	yolog.Println("live @ port %d", Cfg.YO_API_HTTP_PORT)
	panic(http.ListenAndServe(":"+str.FromInt(Cfg.YO_API_HTTP_PORT), http.HandlerFunc(handleHTTPRequest)))
}

func handleHTTPRequest(rw http.ResponseWriter, req *http.Request) {
	ctx := yoctx.NewForHttp(req, rw, Cfg.YO_API_IMPL_TIMEOUT)
	defer ctx.Dispose()

	ctx.Timings.Step("check yoFail")
	if s := ctx.GetStr("yoFail"); s != "" {
		code, _ := str.ToInt(s)
		ctx.HttpErr(If(code == 0, 500, code), "forced error via query-string param 'yoFail'")
		return
	}

	for _, pre_serve := range PreServe {
		pre_serve(ctx)
	}

	{
		ctx.Timings.Step("check static")
		for static_prefix, static_serve := range StaticFileServes {
			if static_prefix = static_prefix + "/"; str.Begins(ctx.Http.UrlPath, static_prefix) && ctx.Http.UrlPath != static_prefix {
				req.URL.Path = ctx.Http.UrlPath[len(static_prefix):]
				ctx.HttpOnPreWriteResponse()
				http.FileServer(http.FS(static_serve)).ServeHTTP(rw, req)
				return
			}
		}
	}

	// no static content was requested or served, so it's an api call
	if result, handler_called := apiHandleRequest(ctx); handler_called {
		ctx.Timings.Step("jsonify result")
		resp_data, err := yojson.MarshalIndent(result, "", "  ")
		if err != nil {
			ctx.HttpErr(500, err.Error())
			return
		}

		ctx.Timings.Step("write resp")
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("X-Content-Type-Options", "nosniff")
		rw.Header().Set("Content-Length", str.FromInt(len(resp_data)))
		ctx.HttpOnPreWriteResponse()
		_, _ = rw.Write(resp_data)
	}
}
