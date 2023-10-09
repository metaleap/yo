package yo

import (
	"embed"
	"net/http"
	"os"

	"yo/context"

	"yo/api"
	. "yo/config"
	"yo/json"
	"yo/str"
	. "yo/util"
)

//go:embed __yostatic
var staticFileDir embed.FS
var apiHandle func(*Ctx) (any, bool)

func init() {
	StaticFileDir = &staticFileDir
}

func ListenAndServe() {
	if IsDevMode {
		StaticFileServes[StaticFileDirPath] = os.DirFS("../yo")
	} else {
		StaticFileServes[StaticFileDirPath] = StaticFileDir
	}
	panic(http.ListenAndServe(":"+str.FromInt(Cfg.YO_API_HTTP_PORT), http.HandlerFunc(handleHTTPRequest)))
}

func handleHTTPRequest(rw http.ResponseWriter, req *http.Request) {
	var ctx *Ctx
	defer ctx.Dispose()

	ctx = context.New(req, rw)

	ctx.Timings.Step("check yoFail")
	if s := ctx.GetStr("yoFail"); s != "" {
		code, _ := str.ToInt(s)
		ctx.HttpErr(If(code == 0, 500, code), "forced error via query-string param 'yoFail'")
		return
	}

	ctx.Timings.Step("check static")
	if ctx.UrlPath == str.ReSuffix(api.SdkGenDstTsFilePath, ".ts", ".js") {
		if IsDevMode {
			http.ServeFile(rw, req, ctx.UrlPath)
		} else {
			http.FileServer(http.FS(StaticFileServes[ctx.UrlPath])).ServeHTTP(rw, req)
		}
		return
	}
	for static_prefix, static_serve := range StaticFileServes {
		if static_prefix = static_prefix + "/"; str.Begins(ctx.UrlPath, static_prefix) && ctx.UrlPath != static_prefix {
			req.URL.Path = str.TrimL(ctx.UrlPath, "__/")
			http.FileServer(http.FS(static_serve)).ServeHTTP(rw, req)
			return
		}
	}

	if result, ok := apiHandle(ctx); ok {
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
