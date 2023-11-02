// for now just re-exports stdlib's `encoding/json`, but this pkg allows for later adoptions of alt impls for perf... and changing back if need be
package yojson

import (
	"encoding/json"

	. "yo/util"
)

type Num = json.Number

var (
	Unmarshal       = json.Unmarshal
	JsonTokNull     = []byte("null")
	JsonTokEmptyArr = []byte("[]")
	JsonTokEmptyObj = []byte("{}")
)

func Load(json_src []byte, dst any) {
	if err := json.Unmarshal(json_src, dst); err != nil {
		if IsDevMode {
			panic(string(json_src) + "\n" + err.Error())
		}
		panic(err)
	}
}

func From(it any, indent bool) (ret []byte) {
	var err error
	if indent {
		ret, err = json.MarshalIndent(it, "", "  ")
	} else {
		ret, err = json.Marshal(it)
	}
	if err != nil {
		panic(err)
	}
	return ret
}

func Dict(fromStruct any) (ret map[string]any) {
	json_src := From(fromStruct, false)
	Load(json_src, &ret)
	return
}
