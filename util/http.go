package util

import (
	"embed"
	"io/fs"
)

var StaticFileDir *embed.FS
var StaticFileServes = map[string]fs.FS{}

const StaticFileDirPath = "__yostatic"
