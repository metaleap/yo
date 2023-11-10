//go:build debug

package util

import (
	"io/fs"
	"os"
	"path/filepath"
	"yo/util/str"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

const IsDevMode = true

func TsFile2JsFileViaEsbuild(tsFilePath string) {
	out_file_path := FsPathSwapExt(tsFilePath, ".ts", ".js")
	ts_src_raw := FsRead(tsFilePath)
	result := esbuild.Transform(string(ts_src_raw), esbuild.TransformOptions{
		Color:         esbuild.ColorNever,
		Sourcemap:     esbuild.SourceMapNone,
		Target:        esbuild.ESNext,
		Platform:      esbuild.PlatformBrowser,
		Charset:       esbuild.CharsetUTF8,
		LegalComments: esbuild.LegalCommentsNone,
		Format:        esbuild.FormatESModule,

		TreeShaking: esbuild.TreeShakingFalse,
		TsconfigRaw: string(FsRead("tsconfig.json")),
		Banner:      "// this js-from-ts by esbuild, not tsc\n",
		Sourcefile:  tsFilePath,
		Loader:      esbuild.LoaderTS,
	})
	for _, msg := range result.Warnings {
		panic("esbuild WARNs: " + str.GoLike(msg))
	}
	for _, msg := range result.Errors {
		panic("esbuild ERRs: " + msg.Text + " @ " + str.GoLike(msg.Location))
	}
	FsWrite(out_file_path, result.Code)
}

func FsDirPathHome() string {
	ret, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return ret
}

func FsDirPathCur() string {
	ret, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return ret
}

func FsPathAbs(fsPath string) string {
	ret, err := filepath.Abs(fsPath)
	if err != nil {
		panic(err)
	}
	return ret
}

func FsPathSwapExt(filePath string, oldExtInclDot string, newExtInclDot string) string {
	if str.Ends(filePath, oldExtInclDot) {
		filePath = filePath[:len(filePath)-len(oldExtInclDot)] + newExtInclDot
	}
	return filePath
}

func FsIsNewerThan(file1Path string, file2Path string) bool {
	fs_info1, fs_info2 := fsStat(file1Path), fsStat(file2Path)
	return (fs_info1 == nil) || (fs_info1.IsDir()) || (fs_info2 == nil) || (fs_info2.IsDir()) ||
		fs_info1.ModTime().After(fs_info2.ModTime())
}

func FsDirEnsure(dirPath string) (did bool) {
	if FsIsDir(dirPath) { // wouldn't think you'd need this, with the below, but do
		return false
	}
	err := os.MkdirAll(dirPath, os.ModePerm)
	if (err != nil) && !os.IsExist(err) {
		panic(err)
	}
	return (err == nil)
}

func FsLinkEnsure(linkLocationPath string, pointsToPath string, ensureDirInstead bool) (did bool) {
	if ensureDirInstead {
		did = FsDirEnsure(pointsToPath)
	} else if !FsIsFile(linkLocationPath) {
		did = FsDirEnsure(filepath.Dir(linkLocationPath))
		points_to_path, link_location_path := FsPathAbs(pointsToPath), FsPathAbs(linkLocationPath)
		if err := os.Symlink(points_to_path, link_location_path); (err != nil) && !os.IsExist(err) {
			panic(err)
		} else {
			did = (err == nil)
		}
	}
	return
}

func FsDirWalk(dirPath string, onDirEntry func(fsPath string, fsEntry fs.DirEntry)) {
	if err := fs.WalkDir(os.DirFS(dirPath), ".", func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			panic(err)
		}
		fs_path := filepath.Join(dirPath, path)
		if fs_path != dirPath { // dont want that DirEntry with Name()=="." in *our* walks
			onDirEntry(fs_path, dirEntry)
		}
		return nil
	}); err != nil {
		panic(err)
	}
}

func FsWalkCodeDirs(yoDir bool, mainDir bool, onDirEntry func(string, fs.DirEntry)) {
	cur_dir_path := FsDirPathCur()
	dir_paths := If(!yoDir, []string{}, []string{filepath.Join(filepath.Dir(cur_dir_path), "yo")})
	if mainDir {
		dir_paths = append(dir_paths, cur_dir_path)
	}
	for _, dir_path := range dir_paths {
		FsDirWalk(dir_path, onDirEntry)
	}
}
