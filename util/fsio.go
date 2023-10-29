package util

import (
	"os"
)

func DelFile(filePath string) {
	_ = os.Remove(filePath)
}

func DelDir(dirPath string) {
	_ = os.RemoveAll(dirPath)
}
