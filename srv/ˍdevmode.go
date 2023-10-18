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

func ViaHttp[TIn any, TOut any](ctx *Ctx, methodPath string, args *TIn, client *http.Client) *TOut {
	json_raw, err := yojson.Marshal(args)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"http://localhost:"+str.FromInt(Cfg.YO_API_HTTP_PORT)+"/"+str.TrimL(methodPath, "/"),
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
	if bytes.Equal(resp_raw, yojson.JsonNullTok) {
		return nil
	}

	var ret TOut
	if err = yojson.Unmarshal(resp_raw, &ret); err != nil {
		panic(err)
	}
	return &ret
}
