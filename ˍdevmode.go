//go:build debug

package yo

import (
	"io/fs"

	yosrv "yo/srv"
	. "yo/util"
	"yo/util/str"
)

func init() {
	ts2jsAppSideStaticDir = func() {
		WalkDir(yosrv.StaticFilesDirNameApp, func(fsPath string, fsEntry fs.DirEntry) {
			if fsEntry.IsDir() {
				return
			}
			switch {
			case str.Ends(fsPath, ".js"):
				ts_file_path := FilePathSwapExt(fsPath, ".js", ".ts")
				if !IsFile(ts_file_path) {
					println("rm " + fsPath)
					DelFile(fsPath)
				}
			case str.Ends(fsPath, ".ts") && !str.Ends(fsPath, ".d.ts"):
				js_file_path := FilePathSwapExt(fsPath, ".ts", ".js")
				if IsNewer(fsPath, js_file_path) {
					println("ts2js " + fsPath)
					TsFile2JsFileViaEsbuild(fsPath)
				}
			}
		})
	}
}
