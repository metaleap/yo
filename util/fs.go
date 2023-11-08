package util

import (
	"io/fs"
	"os"
)

func FileRead(filePath string) []byte {
	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	return data
}

func FileWrite(filePath string, data []byte) {
	err := os.WriteFile(filePath, data, os.ModePerm)
	if err != nil {
		panic(err)
	}
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

func IsDir(dirPath string) bool   { return fsIs(dirPath, fs.FileInfo.IsDir, true) }
func IsFile(filePath string) bool { return fsIs(filePath, fs.FileInfo.IsDir, false) }
