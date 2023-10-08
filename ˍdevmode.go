//go:build debug

package yo

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
)

func init() {
	cur_dir_path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	yo_dir_path, all_enums := filepath.Join(filepath.Dir(cur_dir_path), "yo"), map[string][]string{}
	for _, dir_path := range []string{cur_dir_path, yo_dir_path} {
		if err := fs.WalkDir(os.DirFS(dir_path), ".", func(path string, dirEntry fs.DirEntry, err error) error {
			if path = filepath.Join(dir_path, path); strEnds(path, ".go") {
				data, err := os.ReadFile(path)
				if err != nil {
					panic(err)
				}
				pkg_name := ""
				for _, line := range strSplit(strTrim(string(data)), "\n") {
					if strBegins(line, "package ") {
						pkg_name = line[len("package "):]
					} else if strBegins(line, "\t") && strEnds(line, "\"") && strHas(line, " = \"") {
						if name_and_type, value, ok := strCut(line[1:len(line)-1], " = \""); ok {
							if name, type_name, ok := strCut(name_and_type, " "); ok {
								if name, type_name = strTrim(name), strTrim(type_name); name != type_name && strBegins(name, type_name) {
									enumerant_name := name[len(type_name):]
									if enumerant_name != value && name != value {
										panic(value + "!=" + enumerant_name + " && " + value + "!=" + name)
									}
									all_enums[pkg_name+"."+type_name] = append(all_enums[pkg_name+"."+type_name], value)
								}
							}
						}
					}
				}
			}
			return nil
		}); err != nil {
			panic(err)
		}
	}
	apiReflEnum = func(it *apiReflect, rt reflect.Type, typeIdent string) string {
		found, exists := all_enums[typeIdent]
		if it.Enums[typeIdent] = found; !exists {
			panic("no enumerants for " + typeIdent)
		}
		return typeIdent
	}
}
