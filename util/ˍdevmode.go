//go:build debug

package util

import (
	"io/fs"
	"os"
	"path/filepath"
)

const IsDevMode = true

func CurDirPath() string {
	ret, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return ret
}

func DelFile(filePath string) {
	_ = os.Remove(filePath)
}

func DelDir(dirPath string) {
	_ = os.RemoveAll(dirPath)
}

func fsIs(path string, check func(fs.FileInfo) bool, expect bool) bool {
	info, err := os.Stat(path)
	is_not_exist := os.IsNotExist(err)
	if err != nil && !is_not_exist {
		panic(err)
	}
	if is_not_exist || (info == nil) {
		return false
	}
	return (expect == check(info))
}

func IsDir(dirPath string) bool   { return fsIs(dirPath, fs.FileInfo.IsDir, true) }
func IsFile(filePath string) bool { return fsIs(filePath, fs.FileInfo.IsDir, false) }

func EnsureDir(dirPath string) {
	if err := os.MkdirAll(dirPath, os.ModePerm); (err != nil) && !os.IsExist(err) {
		panic(err)
	}
}

func EnsureLink(linkLocationPath string, pointsToPath string, pointsToIsDir bool) {
	if pointsToIsDir {
		EnsureDir(pointsToPath)
	} else {
		EnsureDir(filepath.Dir(linkLocationPath))
		link_location_path, err := filepath.Abs(linkLocationPath)
		if err != nil {
			panic(err)
		} else if err = os.Symlink(pointsToPath, link_location_path); (err != nil) && !os.IsExist(err) {
			panic(err)
		}
	}
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
