package yosrv

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	. "yo/cfg"
	. "yo/ctx"
	yojson "yo/json"
	"yo/util/str"
)

type DummyLocalDomainJar []*http.Cookie

func (me *DummyLocalDomainJar) Cookies(*url.URL) []*http.Cookie { return ([]*http.Cookie)(*me) }
func (me *DummyLocalDomainJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	if len(cookies) > 0 {
		*me = cookies
	} else {
		panic("HUH?")
	}
}

func NewClient() *http.Client {
	return &http.Client{Timeout: 11 * time.Second, Jar: &DummyLocalDomainJar{}}
}

type viaHttpClient[TIn any, TOut any] interface {
	ViaHttp(apiMethod ApiMethod, ctx *Ctx, args *TIn, client *http.Client) *TOut
}

func (me *apiMethod[TIn, TOut]) ViaHttp(apiMethod ApiMethod, ctx *Ctx, args *TIn, client *http.Client) *TOut {
	for method_path, api_method := range api {
		if api_method == apiMethod {
			return viaHttp[TIn, TOut](method_path, ctx, args, client, apiMethod.isMultipartForm())
		}
	}
	panic("no methodPath for that method")
}

func ViaHttp[TIn any, TOut any](apiMethod ApiMethod, ctx *Ctx, args *TIn, client *http.Client) *TOut {
	return apiMethod.(viaHttpClient[TIn, TOut]).ViaHttp(apiMethod, ctx, args, client)
}

func viaHttp[TIn any, TOut any](methodPath string, ctx *Ctx, args *TIn, client *http.Client, isMultipartForm bool) *TOut {
	payload_bytes := yojson.From(args, false)
	req_content_type := apisContentType_Json
	if isMultipartForm {
		var buf bytes.Buffer
		mpw := multipart.NewWriter(&buf)
		mpw.FormDataContentType()
		if err := mpw.WriteField("_", string(payload_bytes)); err != nil {
			panic(err)
		}
		mpw.Close()
		req_content_type, payload_bytes = mpw.FormDataContentType(), buf.Bytes()
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"http://localhost:"+str.FromInt(Cfg.YO_API_HTTP_PORT)+"/"+str.TrimPref(methodPath, "/")+
			"?"+QueryArgNoCtxPrt+"=1",
		bytes.NewReader(payload_bytes))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", req_content_type)

	resp, err := client.Do(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		panic(err)
	}

	resp_raw, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	if content_type := resp.Header.Get("Content-Type"); content_type != apisContentType_Json {
		panic(string(resp_raw))
	}

	if bytes.Equal(resp_raw, yojson.TokNull) {
		return nil
	}
	var ret TOut
	yojson.Load(resp_raw, &ret)
	return &ret
}
