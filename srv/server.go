package yosrv

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

const QueryArgForceFail = "yoFail"

type PreServe struct {
	Name string
	Do   func(*yoctx.Ctx)
}

var (
	// requests to key+'/' will be served from the corresponding FS
	StaticFileServes = map[string]fs.FS{}

	// funcs are run (in no particular order) just prior to loading request payload, handling request, and serving response
	PreServes = []PreServe{
		{"authAdmin", func(ctx *yoctx.Ctx) {
			if str.Begins(ctx.Http.UrlPath, "__/yo/") || (ctx.Http.UrlPath == "__yo/swag.html") {
				user, pwd, ok := ctx.Http.Req.BasicAuth()
				if ok {
					ok = (user == "foo") && (pwd == "bar")
				}
				if !ok {
					ctx.Http.Resp.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
					panic(yoctx.ErrMustBeAdmin)
				}
			}
		}},
	}

	codegenMaybe func() = nil // overwritten by apisdkgen.go in debug build mode
)

const StaticFilesDirName = "__yostatic"

func InitAndMaybeCodegen(dbStructs []reflect.Type) func() {
	apiReflAllDbStructs = dbStructs
	Apis(ApiMethods{
		"__/yo/refl": Api(apiHandleReflReq, nil),
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
	StaticFileServes["__yo"] = os.DirFS("../yo/" + StaticFilesDirName)
	yolog.Println("live @ port %d", Cfg.YO_API_HTTP_PORT)
	panic(http.ListenAndServe(":"+str.FromInt(Cfg.YO_API_HTTP_PORT), http.HandlerFunc(handleHTTPRequest)))
}

func handleHTTPRequest(rw http.ResponseWriter, req *http.Request) {
	ctx := yoctx.NewForHttp(req, rw, Cfg.YO_API_IMPL_TIMEOUT)
	defer ctx.OnDone()

	ctx.Timings.Step("check " + QueryArgForceFail)
	if s := ctx.GetStr(QueryArgForceFail); s != "" {
		code, _ := str.ToInt(s)
		ctx.HttpErr(If(code == 0, 500, code), "forced error via query-string param '"+QueryArgForceFail+"'")
		return
	}

	for _, pre_serve := range PreServes {
		ctx.Timings.Step("pre:" + pre_serve.Name)
		pre_serve.Do(ctx)
	}

	ctx.Timings.Step("static check")
	for static_prefix, static_serve := range StaticFileServes {
		if static_prefix = static_prefix + "/"; str.Begins(ctx.Http.UrlPath, static_prefix) && ctx.Http.UrlPath != static_prefix {
			req.URL.Path = ctx.Http.UrlPath[len(static_prefix):]
			ctx.Timings.Step("static serve")
			ctx.HttpOnPreWriteResponse()
			http.FileServer(http.FS(static_serve)).ServeHTTP(rw, req)
			return
		}
	}

	// no static content was requested or served, so it's an api call
	if result, handled := apiHandleRequest(ctx); handled {
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
