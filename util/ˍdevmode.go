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

func DirPathHome() string {
	ret, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return ret
}

func DirPathCur() string {
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

func FilePathSwapExt(filePath string, oldExtInclDot string, newExtInclDot string) string {
	if str.Ends(filePath, oldExtInclDot) {
		filePath = filePath[:len(filePath)-len(oldExtInclDot)] + newExtInclDot
	}
	return filePath
}

func IsNewer(file1Path string, file2Path string) bool {
	fs_info_1, fs_info_2 := fsStat(file1Path), fsStat(file2Path)
	return (fs_info_1 == nil) || (fs_info_1.IsDir()) || (fs_info_2 == nil) || (fs_info_2.IsDir()) ||
		fs_info_1.ModTime().After(fs_info_2.ModTime())
}

func EnsureDir(dirPath string) (did bool) {
	if IsDir(dirPath) { // wouldn't think you'd need this, with the below, but do
		return false
	}
	err := os.MkdirAll(dirPath, os.ModePerm)
	if (err != nil) && !os.IsExist(err) {
		panic(err)
	}
	return (err == nil)
}

func EnsureLink(linkLocationPath string, pointsToPath string, pointsToIsDir bool) (did bool) {
	if pointsToIsDir {
		did = EnsureDir(pointsToPath)
	} else if !IsFile(linkLocationPath) { // dito as the comment above in EnsureDir  =)
		did = EnsureDir(filepath.Dir(linkLocationPath))
		points_to_path, link_location_path := FsPathAbs(pointsToPath), FsPathAbs(linkLocationPath)
		if err := os.Symlink(points_to_path, link_location_path); (err != nil) && !os.IsExist(err) {
			panic(err)
		} else {
			did = (err == nil)
		}
	}
	return
}

func WalkDir(dirPath string, onDirEntry func(fsPath string, fsEntry fs.DirEntry)) {
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

func WalkCodeFiles(yoDir bool, mainDir bool, onDirEntry func(string, fs.DirEntry)) {
	cur_dir_path := DirPathCur()
	dir_paths := If(!yoDir, []string{}, []string{filepath.Join(filepath.Dir(cur_dir_path), "yo")})
	if mainDir {
		dir_paths = append(dir_paths, cur_dir_path)
	}
	for _, dir_path := range dir_paths {
		WalkDir(dir_path, onDirEntry)
	}
}

func TsFile2JsFileViaEsbuild(tsFilePath string) {
	out_file_path := FilePathSwapExt(tsFilePath, ".ts", ".js")
	ts_src_raw := FileRead(tsFilePath)
	result := esbuild.Transform(string(ts_src_raw), esbuild.TransformOptions{
		Color:         esbuild.ColorNever,
		Sourcemap:     esbuild.SourceMapNone,
		Target:        esbuild.ESNext,
		Platform:      esbuild.PlatformBrowser,
		Charset:       esbuild.CharsetUTF8,
		LegalComments: esbuild.LegalCommentsNone,
		Format:        esbuild.FormatESModule,

		TreeShaking: esbuild.TreeShakingFalse,
		TsconfigRaw: string(FileRead("tsconfig.json")),
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
	FileWrite(out_file_path, result.Code)
}
