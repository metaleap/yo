//go:build debug

package util

import (
	"io/fs"
	"os"
	"path/filepath"
)

const IsDevMode = true

func WalkCodeFiles(yoDir bool, mainDir bool, onDirEntry func(string, fs.DirEntry)) {
	cur_dir_path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	dir_paths := If(yoDir, []string{filepath.Join(filepath.Dir(cur_dir_path), "yo")}, []string{})
	if mainDir {
		dir_paths = append(dir_paths, cur_dir_path)
	}
	for _, dir_path := range dir_paths {
		if err := fs.WalkDir(os.DirFS(dir_path), ".", func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				panic(err)
			}
			onDirEntry(filepath.Join(dir_path, path), dirEntry)
			return nil
		}); err != nil {
			panic(err)
		}
	}
}
