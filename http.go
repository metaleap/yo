package yo

import (
	"embed"
	"io"
	"net/http"
	"os"

	"yo/context"
	"yo/diag"

	. "yo/config"
	"yo/json"
	"yo/str"
	. "yo/util"
)

//go:embed __yostatic
var staticFileDir embed.FS

func init() {
	StaticFileDir = staticFileDir
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
	var timings diag.Timings
	defer func() {
		if code, crashed := 500, recover(); crashed != nil {
			if err, is_app_err := crashed.(Err); is_app_err {
				code = If(str.Has(err.Error(), "AlreadyExists"), 409, If(str.Has(err.Error(), "DoesNotExist"), 404, 400))
			}
			http.Error(rw, str.Fmt("%v", crashed), code)
		}
		if IsDevMode {
			total_duration, steps := timings.AllDone()
			println(req.RequestURI, str.DurationMs(total_duration))
			for _, step := range steps {
				println("\t" + step.Step + ":\t" + str.DurationMs(step.Duration))
			}
		}
		ctx.Dispose()
	}()

	timings = diag.NewTimings("init ctx", !IsDevMode)
	ctx = context.New(req)

	timings.Step("check yoFail")
	if s := ctx.GetStr("yoFail"); s != "" {
		code, _ := str.ToInt(s)
		http.Error(rw, "forced error via query-string param 'yoFail'", If(code == 0, 500, code))
		return
	}

	url_path := str.TrimR(str.TrimL(req.URL.Path, "/"), "/")

	timings.Step("check static")
	if url_path == str.ReSuffix(ApiSdkGenDstTsFilePath, ".ts", ".js") {
		if IsDevMode {
			http.ServeFile(rw, req, url_path)
		} else {
			http.FileServer(http.FS(StaticFileServes[url_path])).ServeHTTP(rw, req)
		}
		return
	}
	for static_prefix, static_serve := range StaticFileServes {
		if static_prefix = static_prefix + "/"; str.Begins(url_path, static_prefix) && url_path != static_prefix {
			req.URL.Path = str.TrimL(url_path, "__/")
			http.FileServer(http.FS(static_serve)).ServeHTTP(rw, req)
			return
		}
	}

	timings.Step("handler lookup")
	api := API[url_path]
	if api == nil {
		http.Error(rw, "Not Found", 404)
		return
	}

	timings.Step("read req")
	payload_data, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}

	timings.Step("unmarshal req")
	payload, err := api.loadPayload(payload_data)
	if err != nil {
		http.Error(rw, err.Error()+If(IsDevMode, "\n"+string(payload_data), ""), 400)
		return
	}

	timings.Step("handle req")
	result := api.handle()(ctx, payload)

	timings.Step("marshal resp")
	resp_data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}

	timings.Step("write resp")
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Content-Length", str.FromInt(len(resp_data)))
	_, _ = rw.Write(resp_data)
}
