package server

import (
	"embed"
	"io/fs"
	"net/http"
	"os"

	"yo/context"

	. "yo/config"
	"yo/json"
	"yo/str"
	. "yo/util"
)

var staticFileDir *embed.FS
var staticFileServes = map[string]fs.FS{}

const StaticFileDirPath = "__yostatic"

var apiGenSdkMaybe func() = nil // overwritten by apisdkgen.go in debug build mode

func Init(staticFS *embed.FS) (func(), func()) {
	staticFileDir = staticFS
	API["__/refl"] = Method[Void, apiRefl](apiHandleReflReq)
	return apiGenSdkMaybe, listenAndServe
}

func listenAndServe() {
	if IsDevMode {
		staticFileServes[StaticFileDirPath] = os.DirFS("../yo")
	} else {
		staticFileServes[StaticFileDirPath] = staticFileDir
	}
	panic(http.ListenAndServe(":"+str.FromInt(Cfg.YO_API_HTTP_PORT), http.HandlerFunc(handleHTTPRequest)))
}

func handleHTTPRequest(rw http.ResponseWriter, req *http.Request) {
	ctx := context.New(req, rw)
	defer ctx.Dispose()

	ctx.Timings.Step("check yoFail")
	if s := ctx.GetStr("yoFail"); s != "" {
		code, _ := str.ToInt(s)
		ctx.HttpErr(If(code == 0, 500, code), "forced error via query-string param 'yoFail'")
		return
	}

	ctx.Timings.Step("check static")
	if ctx.UrlPath == str.ReSuffix(sdkGenDstTsFilePath, ".ts", ".js") {
		if IsDevMode {
			http.ServeFile(rw, req, ctx.UrlPath)
		} else {
			http.FileServer(http.FS(staticFileServes[ctx.UrlPath])).ServeHTTP(rw, req)
		}
		return
	}
	for static_prefix, static_serve := range staticFileServes {
		if static_prefix = static_prefix + "/"; str.Begins(ctx.UrlPath, static_prefix) && ctx.UrlPath != static_prefix {
			req.URL.Path = str.TrimL(ctx.UrlPath, "__/")
			http.FileServer(http.FS(static_serve)).ServeHTTP(rw, req)
			return
		}
	}

	if result, handler_called := apiHandleRequest(ctx); handler_called {
		ctx.Timings.Step("marshal resp")
		resp_data, err := json.MarshalIndent(result, "", "  ")
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
