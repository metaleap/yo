//go:build debug

package yo

import (
	"io/fs"
	"path/filepath"

	yosrv "yo/srv"
	. "yo/util"
	"yo/util/str"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

func init() {
	buildFun = doBuildAppDeployably
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

func doBuildAppDeployably() {
	app_name := filepath.Base(DirPathCur())
	dst_dir_path := filepath.Join(DirPathHome(), "rwa", "src", app_name)
	DelDir(dst_dir_path)
	EnsureDir(dst_dir_path)

	// copy static files other than .ts / .js or sub dirs (all non-script files sit in top level, never in sub dirs)
	for src_dir_path, is_app := range map[string]bool{
		"../yo/__yostatic": false,
		"__static":         true,
	} {
		strip := If(is_app, "", "../yo/")
		WalkDir(src_dir_path, func(fsPath string, fsEntry fs.DirEntry) {
			path_equiv := fsPath[len(strip):]
			if (!fsEntry.IsDir()) && (!str.Ends(fsPath, ".js")) && !str.Ends(fsPath, ".ts") {
				dst_file_path := filepath.Join(dst_dir_path, path_equiv)
				EnsureDir(filepath.Dir(dst_file_path))
				CopyFile(fsPath, dst_file_path)
			}
		})
		esbuild_options := esbuild.BuildOptions{
			Color:             esbuild.ColorNever,
			Sourcemap:         esbuild.SourceMapNone,
			Target:            esbuild.ESNext,
			Platform:          esbuild.PlatformBrowser,
			Charset:           esbuild.CharsetUTF8,
			IgnoreAnnotations: true,
			LegalComments:     esbuild.LegalCommentsNone,

			EntryPoints: If(is_app,
				[]string{"__static/" + app_name + ".js"},
				[]string{"__yostatic/yo.js", "__yostatic/yo-sdk.js"}),
			Bundle:            true,
			MinifyWhitespace:  true,
			MinifyIdentifiers: true,
			MinifySyntax:      true,
			TreeShaking:       esbuild.TreeShakingTrue,
			Outdir:            filepath.Join(dst_dir_path, If(is_app, "__static", "__yostatic")),
			Write:             true,
		}
		println(is_app, str.FmtV(esbuild_options))
		if true {
			continue
		}
		result := esbuild.Build(esbuild_options)
		for _, msg := range result.Warnings {
			panic("esbuild WARNs: " + str.GoLike(msg))
		}
		for _, msg := range result.Errors {
			panic("esbuild ERRs: " + msg.Text + " @ " + str.GoLike(msg.Location))
		}
	}
}
