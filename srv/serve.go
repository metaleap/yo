package yosrv

import (
	"bytes"
	"crypto/subtle"
	"io"
	"io/fs"
	"net/http"
	"os"
	"reflect"
	"time"

	. "yo/cfg"
	yoctx "yo/ctx"
	yojson "yo/json"
	yolog "yo/log"
	. "yo/util"
	"yo/util/kv"
	"yo/util/sl"
	"yo/util/str"
)

const QueryArgForceFail = "yoFail"
const QueryArgForceUser = "yoUser"
const StaticFilesDirName_Yo = "__yostatic"
const StaticFilesDirName_App = "__static"

var StaticFileDir_Yo fs.FS
var StaticFileDir_App fs.FS
var StaticFileDirs = map[string]fs.FS{}

type Middleware struct {
	Name string
	Do   func(*yoctx.Ctx)
}

var (
	detectEnumsAndMaybeCodegen func() = nil // overwritten by codegen_apistuff.go in debug build mode

	// requests to key+'/' will be served from the corresponding FS
	apiStdRespHeaders = str.Dict{
		"Content-Type":           apisContentType_Json,
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "no-store",
	}

	// PreServes funcs are run at the start of the request handling, prior to any other request processing
	PreServes = []Middleware{}

	// PreHandling funcs are run just prior to loading api request payload, handling that request, and serving the handler's response
	PreApiHandling = []Middleware{}

	// PostApiHandling funcs are run in order only after a non-erroring request had its response fully sent.
	// They should kick off any IO-involving operations in a separate goroutine (but those shouldn't take the `Ctx`).
	PostApiHandling = []Middleware{}

	StaticFileFilters = map[string]func(*yoctx.Ctx, []byte) (fileExt string, fileSrc []byte){}

	// set by app for home and vanity-URL (non-file, non-API) requests
	AppSideStaticRePathFor func(string) string

	OnBeforeServingStaticFile = func(*yoctx.Ctx) {}
)

func InitAndMaybeCodegen(dbStructs []reflect.Type) func() {
	apiReflAllDbStructs = dbStructs
	for dir_name, dir_path := range Cfg.STATIC_FILE_STORAGE_DIRS {
		StaticFileDirs[dir_name] = os.DirFS(dir_path)
	}
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
	if !IsDevMode {
		PreServes = append(PreServes, Middleware{"authAdmin", If(IsDevMode, nil, authAdmin)})
	}
	yolog.Println("live @ port %d", Cfg.YO_API_HTTP_PORT)
	yoctx.IsUp = true
	panic(http.ListenAndServe(":"+str.FromInt(Cfg.YO_API_HTTP_PORT), http.HandlerFunc(handleHttpRequest)))
}

func handleHttpRequest(rw http.ResponseWriter, req *http.Request) {
	ctx := yoctx.NewCtxForHttp(req, rw, Cfg.YO_API_IMPL_TIMEOUT, false)
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
	if handleHttpStaticFileRequestMaybe(ctx) {
		return
	}

	for _, pre_serve := range PreApiHandling {
		if pre_serve.Do != nil {
			ctx.Timings.Step("pre:" + pre_serve.Name)
			pre_serve.Do(ctx)
		}
	}

	if result, handler_called := apiHandleRequest(ctx); handler_called { // if not, `apiHandleRequest` did `http.Error()` and nothing more to do here
		ctx.Timings.Step("jsonify resp")
		resp_data := yojson.From(result, true)

		ctx.Timings.Step("write resp")
		for k, v := range apiStdRespHeaders {
			rw.Header().Set(k, v)
		}
		rw.Header().Set("Content-Length", str.FromInt(len(resp_data)))
		ctx.HttpOnPreWriteResponse()
		_, _ = rw.Write(resp_data)

		if len(PostApiHandling) != 0 {
			for _, middleware := range PostApiHandling {
				middleware.Do(ctx)
			}
		}
	}
}

func handleHttpStaticFileRequestMaybe(ctx *yoctx.Ctx) bool {
	if (AppSideStaticRePathFor != nil) && (!str.Begins(ctx.Http.UrlPath, "__")) &&
		!sl.Any(kv.Keys(StaticFileDirs), func(it string) bool { return str.Begins(ctx.Http.UrlPath, it+"/") }) {
		if re_path := AppSideStaticRePathFor(ctx.Http.UrlPath); (re_path != "") && (re_path != ctx.Http.UrlPath) {
			ctx.Http.UrlPath = re_path
			ctx.Http.Req.RequestURI = "/" + re_path // loses query-args, which aren't expected for purely static content anyway
			ctx.Http.Req.URL.Path = "/" + re_path
		}
	}

	var fs_static fs.FS
	var fs_handler http.Handler
	var fs_strip_name string
	for dir_name, fs := range StaticFileDirs {
		if static_prefix := dir_name + "/"; str.Begins(ctx.Http.UrlPath, static_prefix) && (ctx.Http.UrlPath != static_prefix) {
			fs_strip_name = dir_name
			fs_static, fs_handler = fs, http.StripPrefix("/"+fs_strip_name, http.FileServer(http.FS(fs)))
			break
		}
	}
	if fs_handler == nil {
		if static_prefix := StaticFilesDirName_Yo + "/"; str.Begins(ctx.Http.UrlPath, static_prefix) && (ctx.Http.UrlPath != static_prefix) {
			fs_static, fs_handler = StaticFileDir_Yo, http.FileServer(http.FS(StaticFileDir_Yo))
		} else if (StaticFileDir_App != nil) && (StaticFilesDirName_App != "") && str.Begins(ctx.Http.UrlPath, StaticFilesDirName_App+"/") {
			fs_static, fs_handler = StaticFileDir_App, http.FileServer(http.FS(StaticFileDir_App))
		}
	}
	if fs_handler != nil {
		ctx.Timings.Step("static serve")
		OnBeforeServingStaticFile(ctx)
		if ctx.Http.Req.URL.RawQuery != "" {
			for query_arg_name, static_file_filter := range StaticFileFilters {
				if ctx.GetStr(query_arg_name) != "" {
					file, err := fs_static.Open(str.TrimPref(ctx.Http.UrlPath, fs_strip_name+"/"))
					if file != nil {
						defer file.Close()
					}
					if err != nil {
						panic(err)
					}
					data, err := io.ReadAll(file)
					if err != nil {
						panic(err)
					}
					file_ext, data := static_file_filter(ctx, data)
					ctx.HttpOnPreWriteResponse()
					http.ServeContent(ctx.Http.Resp, ctx.Http.Req, ctx.Http.UrlPath+file_ext, time.Now(), bytes.NewReader(data))
					return true
				}
			}
		}

		ctx.HttpOnPreWriteResponse()
		httpFileServer(fs_handler).ServeHTTP(ctx.Http.Resp, ctx.Http.Req)
		return true
	}
	return false
}

func authAdmin(ctx *yoctx.Ctx) {
	if (!(str.Begins(ctx.Http.UrlPath, yoAdminApisUrlPrefix) || str.Begins(ctx.Http.UrlPath, StaticFilesDirName_Yo+"/yo."))) || (ctx.Http.UrlPath == (yoAdminApisUrlPrefix + "refl")) {
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

func httpFileServer(fileServingHandler http.Handler) http.Handler { // stackoverflow.com/a/33881296
	if !IsDevMode {
		return fileServingHandler
	}
	// local dev: dont keep cache-encouraging req headers; also send cache-discouraging resp headers
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
