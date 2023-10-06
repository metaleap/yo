package yo

import (
	"embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

//go:embed .frontend
var staticFrontendDir embed.FS
var staticFileServingHandler = http.FileServer(http.FS(&staticFrontendDir))

func ListenAndServe() {
	log.Fatal(http.ListenAndServe(":"+iToA(cfg.YO_API_HTTP_PORT), http.HandlerFunc(handleHTTPRequest)))
}

func handleHTTPRequest(rw http.ResponseWriter, req *http.Request) {
	if !IsDebugMode { // in debug-mode, DO want stack traces
		defer func() {
			if crashed := recover(); crashed != nil {
				http.Error(rw, strFmt("%v", crashed), 500)
			}
		}()
	}

	ctx := ctxNew(req)
	defer ctx.dispose()

	urlPath := strTrimL(req.URL.Path, "/")
	if static_prefix := "__/.frontend/"; strBegins(urlPath, static_prefix) && urlPath != static_prefix {
		req.URL.Path = strTrimL(urlPath, "__/")
		staticFileServingHandler.ServeHTTP(rw, req)
		return
	}

	api := API[urlPath]
	if api == nil {
		http.Error(rw, "Not Found", 404)
		return
	}

	payload_data, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	payload, err := api.loadPayload(payload_data)
	if err != nil {
		http.Error(rw, err.Error(), 400)
		return
	}

	result, err := api.handle()(ctx, payload)
	if code := 500; err != nil {
		if _, is_app_err := err.(Err); is_app_err {
			code = If(strHas(err.Error(), "AlreadyExists"), 409, If(strHas(err.Error(), "DoesNotExist"), 404, 400))
		}
		http.Error(rw, err.Error(), code)
		return
	}

	resp_data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Content-Length", iToA(len(resp_data)))
	_, _ = rw.Write(resp_data)
}
