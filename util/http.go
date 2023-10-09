package util

import (
	"io/fs"
)

var StaticFileDir fs.FS
var StaticFileServes = map[string]fs.FS{}

const StaticFileDirPath = "__yostatic"
