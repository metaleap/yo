package yojson

import (
	"encoding/json"
)

type Num = json.Number

var (
	MarshalIndent = json.MarshalIndent
	Unmarshal     = json.Unmarshal
	JsonNullTok   = []byte("null")
)
