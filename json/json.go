// for now just re-exports stdlib's `encoding/json`, but this pkg allows for later adoptions of alt impls for perf... and changing back if need be
package yojson

import (
	"encoding/json"
)

type Num = json.Number

var (
	MarshalIndent   = json.MarshalIndent
	Marshal         = json.Marshal
	Unmarshal       = json.Unmarshal
	JsonTokNull     = []byte("null")
	JsonTokEmptyArr = []byte("[]")
	JsonTokEmptyObj = []byte("{}")
)
