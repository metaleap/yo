package yoserve

import (
	"embed"
	"io/fs"
	"net/http"
	"os"

	. "yo/cfg"
	yoctx "yo/ctx"
	yojson "yo/json"
	yolog "yo/log"
	. "yo/util"
	"yo/util/str"
)

var staticFileDir *embed.FS
var staticFileServes = map[string]fs.FS{}

const StaticFileDirPath = "__yostatic"

var apiGenSdkMaybe func() = nil // overwritten by apisdkgen.go in debug build mode

func Init(staticFS *embed.FS) (func(), func()) {
	staticFileDir = staticFS
	API["__/refl"] = Method(apiHandleReflReq)
	for method_path := range API {
		if str.Trim(method_path) != method_path || method_path == "" || !str.IsPrtAscii(method_path) {
			panic("not a valid method path: '" + method_path + "'")
		}
	}
	return apiGenSdkMaybe, listenAndServe
}

func listenAndServe() {
	if IsDevMode {
		staticFileServes[StaticFileDirPath] = os.DirFS("../yo")
	} else {
		staticFileServes[StaticFileDirPath] = staticFileDir
	}
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

	ctx.Timings.Step("check static")
	if ctx.Http.UrlPath == str.ReSuffix(sdkGenDstTsFilePath, ".ts", ".js") {
		if IsDevMode {
			http.ServeFile(rw, req, ctx.Http.UrlPath)
		} else {
			http.FileServer(http.FS(staticFileServes[ctx.Http.UrlPath])).ServeHTTP(rw, req)
		}
		return
	}
	for static_prefix, static_serve := range staticFileServes {
		if static_prefix = static_prefix + "/"; str.Begins(ctx.Http.UrlPath, static_prefix) && ctx.Http.UrlPath != static_prefix {
			req.URL.Path = str.TrimL(ctx.Http.UrlPath, "__/")
			http.FileServer(http.FS(static_serve)).ServeHTTP(rw, req)
			return
		}
	}

	if result, handler_called := apiHandleRequest(ctx); handler_called {
		ctx.Timings.Step("jsonify result")
		resp_data, err := yojson.MarshalIndent(result, "", "  ")
		if err != nil {
			ctx.HttpErr(500, err.Error())
			return
		}

		ctx.Timings.Step("write resp")
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Content-Length", str.FromInt(len(resp_data)))
		_, _ = rw.Write(resp_data)
	}
}
