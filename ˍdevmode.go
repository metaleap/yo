//go:build debug

package yo

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	yosrv "yo/srv"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

var AppSideBuildTimeContainerFileNames []string

func init() {
	buildDeployablyNow = doBuildAppDeployablyAndMaybePush
	ts2jsAppSideStaticDir = func() {
		FsDirWalk(yosrv.StaticFilesDirNameApp, func(fsPath string, fsEntry fs.DirEntry) {
			if fsEntry.IsDir() {
				return
			}
			switch {
			case str.Ends(fsPath, ".js"):
				ts_file_path := FsPathSwapExt(fsPath, ".js", ".ts")
				if !FsIsFile(ts_file_path) {
					println("rm " + fsPath)
					FsDelFile(fsPath)
				}
			case str.Ends(fsPath, ".ts") && !str.Ends(fsPath, ".d.ts"):
				js_file_path := FsPathSwapExt(fsPath, ".ts", ".js")
				if FsIsNewerThan(fsPath, js_file_path) {
					println("ts2js " + fsPath)
					TsFile2JsFileViaEsbuild(fsPath)
				}
			}
		})
	}
}

func doBuildAppDeployablyAndMaybePush() {
	app_name := filepath.Base(FsDirPathCur())
	dst_dir_path := filepath.Join(FsDirPathHome(), "rwa", "src-"+app_name)
	deploy_dir_path := filepath.Join(FsDirPathHome(), "rwa", "deploy-"+app_name)
	FsDelDir(dst_dir_path)
	FsDirEnsure(dst_dir_path)
	FsDirEnsure(deploy_dir_path)

	// 1. clear deploy_dir_path except for .git
	dotgit_dir_path := filepath.Join(deploy_dir_path, ".git")
	FsDirWalk(deploy_dir_path, func(fsPath string, fsEntry fs.DirEntry) {
		if str.Begins(fsPath, dotgit_dir_path) {
			return
		}
		if !FsIsDir(fsPath) {
			FsDelFile(fsPath)
		} else {
			panic("TODO since apparently this unexpected-by-design need for dirs here arose: replace this panic with just `DelDir(fsPath)`")
		}
	})

	if true { // 2. touch own Dockerfile, preferred choice
		FsWrite(filepath.Join(deploy_dir_path, "Dockerfile"), []byte(str.Trim(`
FROM scratch
COPY .env /.env
COPY .env.prod /.env.prod
COPY `+app_name+`.exec /`+app_name+`.exec
`+str.Join(sl.As(AppSideBuildTimeContainerFileNames, func(s string) string { return "COPY " + s + " /" + s }), "\n")+`
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/`+app_name+`.exec"]
	`)))
	} else { // 2. touch railway.toml, the alternative way (implies Railway.app default Dockerfile)
		FsWrite(filepath.Join(deploy_dir_path, "railway.toml"), []byte(str.Trim(`
# note, unused if Dockerfile present too
[build]
builder = "nixpacks"
buildCommand = "chmod +x ./`+app_name+`.exec"
watchPatterns = ["*"]
[deploy]
startCommand = "./`+app_name+`.exec"
restartPolicyType = "ALWAYS"
restartPolicyMaxRetries = 2
# healthcheckPath = "/"
# healthcheckTimeout = 543
	`)))
	}

	// 3. touch go.work
	FsWrite(filepath.Join(dst_dir_path, "go.work"), []byte(str.Trim(`
go `+str.TrimPref(runtime.Version(), "go")+`
use ./yo
use ./`+app_name+`
	`)))

	// 4. copy .go and .env files
	for src_dir_path, is_app := range map[string]bool{
		".":     true,
		"../yo": false,
	} {
		strip := If(is_app, "", "../yo/")
		FsDirWalk(src_dir_path, func(fsPath string, fsEntry fs.DirEntry) {
			if str.Ends(fsPath, ".go") || str.Ends(fsPath, "go.mod") {
				path_equiv := fsPath[len(strip):]
				dst_file_path := filepath.Join(dst_dir_path, If(is_app, app_name, "yo"), path_equiv)
				FsDirEnsure(filepath.Dir(dst_file_path))
				FsCopy(fsPath, dst_file_path)
			} else if is_app && (str.Ends(fsPath, ".env") || str.Ends(fsPath, ".env.prod")) {
				FsCopy(fsPath, filepath.Join(deploy_dir_path, fsEntry.Name()))
			} else if is_app {
				for _, file_name := range AppSideBuildTimeContainerFileNames {
					if str.Ends(fsPath, filepath.Join(src_dir_path, file_name)) {
						FsCopy(fsPath, filepath.Join(deploy_dir_path, fsEntry.Name()))
					}
				}
			}
		})
	}

	// 5. ensure static files
	for src_dir_path, is_app := range map[string]bool{
		"../yo/__yostatic": false,
		"__static":         true,
	} {
		strip := If(is_app, "", "../yo/")
		// copy static files other than .ts / .js or sub dirs (all non-script files sit in top level, never in sub dirs)
		FsDirWalk(src_dir_path, func(fsPath string, fsEntry fs.DirEntry) {
			path_equiv := fsPath[len(strip):]
			if (!fsEntry.IsDir()) && (!str.Ends(fsPath, ".js")) && !str.Ends(fsPath, ".ts") {
				dst_file_path := filepath.Join(dst_dir_path, app_name, path_equiv)
				FsDirEnsure(filepath.Dir(dst_file_path))
				if !str.Ends(fsPath, ".css") {
					FsCopy(fsPath, dst_file_path)
				} else {
					FsWrite(dst_file_path, cssDownsize(FsRead(fsPath)))
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

	// 6. go build
	println("BUILD...")
	cmd_go := exec.Command("go", "build",
		"-C", dst_dir_path,
		"-o", filepath.Join(deploy_dir_path, app_name+".exec"),
		"-buildvcs=false",
		"-a",
		"-installsuffix", "deploy_"+app_name,
		"-ldflags", "-w -s -extldflags \"-static\"",
		"-tags", "timetzdata",
		"./"+app_name)
	cmd_go.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	cmd_out, err := cmd_go.CombinedOutput()
	if err != nil {
		panic(str.Fmt("%s>>>>%s", err, cmd_out))
	}

	if os.Getenv("YO_PUSH") != "" {
		// 7. git push
		println("PUSH...")
		msg_commit := time.Now().Format(time.DateTime)
		cmd_git1, cmd_git2, cmd_git3 := exec.Command("git", "add", "-A"), exec.Command("git", "commit", "-m", msg_commit), exec.Command("git", "push", "--force")
		cmd_git1.Dir, cmd_git2.Dir, cmd_git3.Dir = deploy_dir_path, deploy_dir_path, deploy_dir_path
		for _, cmd_git := range []*exec.Cmd{cmd_git1, cmd_git2, cmd_git3} {
			cmd_out, err := cmd_git.CombinedOutput()
			if err != nil {
				println(str.Fmt("%s>>>>%s", err, cmd_out))
			}
		}
	}
}

func cssDownsize(srcCss []byte) []byte {
	is_ascii_nonspace_whitespace, is_sep, is_brace_or_paren := func(c byte) bool { return (c == '\n') || (c == '\t') }, func(c byte) bool { return (c == ':') || (c == ';') || (c == ',') }, func(c byte) bool {
		return (c == '{') || (c == '}') || (c == '[') || (c == ']') || (c == '(') // || (c == ')') // keep the closing-paren out, generates bugged css with in situations like `var(--foo) calc(...)`
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
