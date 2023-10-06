package yo

import (
	"net/http"
	"strings"
)

type APIMethod func(*Ctx, any) (any, error)

var API = map[string]APIMethod{}

func apisInit() {

}

func ListenAndServe() {
	http.ListenAndServe(":5555", http.HandlerFunc(handleHTTPRequest))
}

func handleHTTPRequest(rw http.ResponseWriter, req *http.Request) {
	ctx := ctxNew(req)
	defer ctx.dispose()
	path := strings.TrimPrefix(req.URL.Path, "/")
	api := API[path]
	if api == nil {
		http.Error(rw, "Not Found", 404)
		return
	}

}
