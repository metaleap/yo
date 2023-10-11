package yojson

import (
	"encoding/json"
)

var (
	MarshalIndent = json.MarshalIndent
	Unmarshal     = json.Unmarshal
	JsonNullTok   = []byte("null")
)
