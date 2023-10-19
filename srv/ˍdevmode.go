package yosrv

import (
	"bytes"
	"io"
	"net/http"

	. "yo/cfg"
	. "yo/ctx"
	yojson "yo/json"
	"yo/util/str"
)

type viaHttpClient[TIn any, TOut any] interface {
	ViaHttp(apiMethod ApiMethod, ctx *Ctx, args *TIn, client *http.Client) *TOut
}

func (me *apiMethod[TIn, TOut]) ViaHttp(apiMethod ApiMethod, ctx *Ctx, args *TIn, client *http.Client) *TOut {
	for method_path, api_method := range api {
		if api_method == apiMethod {
			return viaHttp[TIn, TOut](method_path, ctx, args, client)
		}
	}
	panic("no methodPath for that method")
}

func ViaHttp[TIn any, TOut any](apiMethod ApiMethod, ctx *Ctx, args *TIn, client *http.Client) *TOut {
	return apiMethod.(viaHttpClient[TIn, TOut]).ViaHttp(apiMethod, ctx, args, client)
}

func viaHttp[TIn any, TOut any](methodPath string, ctx *Ctx, args *TIn, client *http.Client) *TOut {
	json_raw, err := yojson.Marshal(args)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"http://localhost:"+str.FromInt(Cfg.YO_API_HTTP_PORT)+"/"+str.TrimL(AppApiUrlPrefix+methodPath, "/"),
		bytes.NewReader(json_raw))
	if err != nil {
		panic(err)
	}

	resp, err := client.Do(req)
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		panic(err)
	}

	resp_raw, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	if bytes.Equal(resp_raw, yojson.JsonTokNull) {
		return nil
	}

	var ret TOut
	if err = yojson.Unmarshal(resp_raw, &ret); err != nil {
		panic(err)
	}
	return &ret
}
