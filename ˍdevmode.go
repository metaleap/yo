//go:build debug

package yo

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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
	dst_dir_path := filepath.Join(DirPathHome(), "rwa", "src-"+app_name)
	deploy_dir_path := filepath.Join(DirPathHome(), "rwa", "deploy-"+app_name)
	DelDir(dst_dir_path)
	EnsureDir(dst_dir_path)

	// 1. touch go.work
	FileWrite(filepath.Join(dst_dir_path, "go.work"), []byte(str.Trim(`
go `+str.TrimPref(runtime.Version(), "go")+`
use ./yo
use ./`+app_name+`
	`)))

	// 2. copy .go and .env files
	for src_dir_path, is_app := range map[string]bool{
		".":     true,
		"../yo": false,
	} {
		strip := If(is_app, "", "../yo/")
		WalkDir(src_dir_path, func(fsPath string, fsEntry fs.DirEntry) {
			if str.Ends(fsPath, ".go") || str.Ends(fsPath, "go.mod") {
				path_equiv := fsPath[len(strip):]
				dst_file_path := filepath.Join(dst_dir_path, If(is_app, app_name, "yo"), path_equiv)
				EnsureDir(filepath.Dir(dst_file_path))
				CopyFile(fsPath, dst_file_path)
			} else if str.Ends(fsPath, ".env") || str.Ends(fsPath, ".env.prod") {
				CopyFile(fsPath, filepath.Join(deploy_dir_path, fsEntry.Name()))
			}
		})
	}

	// 3. ensure static files
	for src_dir_path, is_app := range map[string]bool{
		"../yo/__yostatic": false,
		"__static":         true,
	} {
		strip := If(is_app, "", "../yo/")

		// copy static files other than .ts / .js or sub dirs (all non-script files sit in top level, never in sub dirs)
		WalkDir(src_dir_path, func(fsPath string, fsEntry fs.DirEntry) {
			path_equiv := fsPath[len(strip):]
			if (!fsEntry.IsDir()) && (!str.Ends(fsPath, ".js")) && !str.Ends(fsPath, ".ts") {
				dst_file_path := filepath.Join(dst_dir_path, app_name, path_equiv)
				EnsureDir(filepath.Dir(dst_file_path))
				if !str.Ends(fsPath, ".css") {
					CopyFile(fsPath, dst_file_path)
				} else {
					FileWrite(dst_file_path, cssDownsize(FileRead(fsPath)))
				}
			}
		})

		// bundle+minify .js files
		esbuild_options := esbuild.BuildOptions{
			Color:         esbuild.ColorNever,
			Sourcemap:     esbuild.SourceMapNone,
			Target:        esbuild.ESNext,
			Platform:      esbuild.PlatformBrowser,
			Charset:       esbuild.CharsetUTF8,
			LegalComments: esbuild.LegalCommentsNone,
			Format:        esbuild.FormatESModule,

			EntryPoints: If(is_app,
				[]string{"__static/" + app_name + ".js"},
				[]string{"__yostatic/yo.js", "__yostatic/yo-sdk.js"}),
			Bundle:            true,
			MinifyWhitespace:  true,
			MinifyIdentifiers: true,
			MinifySyntax:      true,
			TreeShaking:       esbuild.TreeShakingTrue,
			Outdir:            filepath.Join(dst_dir_path, app_name, If(is_app, "__static", "__yostatic")),
			Write:             true,
		}
		result := esbuild.Build(esbuild_options)
		for _, msg := range result.Warnings {
			panic("esbuild WARNs: " + str.GoLike(msg))
		}
		for _, msg := range result.Errors {
			panic("esbuild ERRs: " + msg.Text + " @ " + str.GoLike(msg.Location))
		}
	}

	// 4. go build
	os.Setenv("CGO_ENABLED", "0")
	cmd_go := exec.Command("go", "build",
		"-C", dst_dir_path,
		"-o", filepath.Join(deploy_dir_path, app_name+".exec"),
		"-buildvcs=false",
		"./"+app_name)
	cmd_out, err := cmd_go.CombinedOutput()
	if err != nil {
		panic(str.Fmt("%s>>>>%s", err, cmd_out))
	}
}

func cssDownsize(srcCss []byte) []byte {
	is_ascii_nonspace_whitespace, is_sep, is_brace_or_paren := func(c byte) bool { return (c == '\n') || (c == '\t') }, func(c byte) bool { return (c == ':') || (c == ';') || (c == ',') }, func(c byte) bool {
		return (c == '{') || (c == '}') || (c == '[') || (c == ']') || (c == '(') || (c == ')')
	}
	for again, start, end := true, []byte("/*"), []byte("*/"); again; {
		again = false
		if idx2 := bytes.Index(srcCss, end); idx2 > 1 {
			if idx1 := bytes.Index(srcCss[:idx2], start); idx1 >= 0 {
				again = true
				srcCss = append(srcCss[:idx1], srcCss[idx2+2:]...)
			}
		}
	}
	for i := 0; i < len(srcCss); i++ {
		if c := srcCss[i]; is_ascii_nonspace_whitespace(c) || ((c == ' ') &&
			(((i > 0) && (srcCss[i-1] == ' ') || is_brace_or_paren(srcCss[i-1]) || is_sep(srcCss[i-1])) ||
				((i < (len(srcCss)-1)) && (srcCss[i+1] == ' ') || is_brace_or_paren(srcCss[i+1]) || is_sep(srcCss[i+1])))) {
			srcCss = append(srcCss[:i], srcCss[i+1:]...)
			i--
		}
	}
	return srcCss
}
