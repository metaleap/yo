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
			is_dir := fsEntry.IsDir()
			switch {
			case (!is_dir) && str.Ends(fsPath, ".js"):
				ts_file_path := FilePathSwapExt(fsPath, ".js", ".ts")
				if !IsFile(ts_file_path) {
					DelFile(fsPath)
				}
			case (!is_dir) && str.Ends(fsPath, ".ts") && !str.Ends(fsPath, ".d.ts"):
				// js_file_path := path[:len(path)-len(".ts")] + ".js"
				// is_newer := (!IsFile(js_file_path))
			}
		})
	}
}
