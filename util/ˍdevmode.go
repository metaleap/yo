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

func CurDirPath() string {
	ret, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return ret
}

func fsStat(path string) fs.FileInfo {
	fs_info, err := os.Stat(path)
	is_not_exist := os.IsNotExist(err)
	if err != nil && !is_not_exist {
		panic(err)
	}
	return If(is_not_exist, nil, fs_info)
}

func fsIs(path string, check func(fs.FileInfo) bool, expect bool) bool {
	fs_info := fsStat(path)
	return (fs_info != nil) && (expect == check(fs_info))
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

func IsDir(dirPath string) bool   { return fsIs(dirPath, fs.FileInfo.IsDir, true) }
func IsFile(filePath string) bool { return fsIs(filePath, fs.FileInfo.IsDir, false) }

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

func ReadFile(filePath string) []byte {
	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	return data
}

func WriteFile(filePath string, data []byte) {
	err := os.WriteFile(filePath, data, os.ModePerm)
	if err != nil {
		panic(err)
	}
}

func WalkDir(dirPath string, onDirEntry func(string, fs.DirEntry)) {
	if err := fs.WalkDir(os.DirFS(dirPath), ".", func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			panic(err)
		}
		onDirEntry(filepath.Join(dirPath, path), dirEntry)
		return nil
	}); err != nil {
		panic(err)
	}
}

func WalkCodeFiles(yoDir bool, mainDir bool, onDirEntry func(string, fs.DirEntry)) {
	cur_dir_path := CurDirPath()
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
	ts_src_raw := ReadFile(tsFilePath)
	result := esbuild.Transform(string(ts_src_raw), esbuild.TransformOptions{
		Color:             esbuild.ColorNever,
		Sourcemap:         esbuild.SourceMapNone,
		Target:            esbuild.ESNext,
		Platform:          esbuild.PlatformBrowser,
		Format:            esbuild.FormatESModule,
		Charset:           esbuild.CharsetUTF8,
		TreeShaking:       esbuild.TreeShakingFalse,
		IgnoreAnnotations: true,
		LegalComments:     esbuild.LegalCommentsNone,
		TsconfigRaw:       string(ReadFile("tsconfig.json")),
		Banner:            "// this js-from-ts by esbuild, not tsc\n",
		Sourcefile:        tsFilePath,
		Loader:            esbuild.LoaderTS,
	})
	for _, msg := range result.Warnings {
		panic("esbuild WARNs: " + str.From(msg))
	}
	for _, msg := range result.Errors {
		panic("esbuild ERRs: " + msg.Text + " @ " + str.From(msg.Location))
	}
	WriteFile(out_file_path, result.Code)
}
