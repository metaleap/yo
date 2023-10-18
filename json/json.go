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
