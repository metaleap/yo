package util

import (
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
