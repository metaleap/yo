//go:build debug

package yoserve

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"sync"

	yolog "yo/log"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

const sdkGenDstTsFilePath = StaticFileDirPath + "/yo-sdk.ts"

var foundModifiedTsFiles bool

func init() {
	apiGenSdkMaybe = apiGenSdk

	WalkCodeFiles(true, true, func(path string, dirEntry fs.DirEntry) {
		if str.Ends(path, ".ts") && (!str.Ends(path, ".d.ts")) && !foundModifiedTsFiles {
			fileinfo_ts, err := dirEntry.Info()
			if err != nil || fileinfo_ts == nil {
				panic(err)
			}
			fileinfo_js, err := os.Stat(path[:len(path)-len(".ts")] + ".js")
			foundModifiedTsFiles = ((fileinfo_js == nil) || (err != nil) ||
				(fileinfo_ts.ModTime().After(fileinfo_js.ModTime())))
		}

		if str.Ends(path, ".go") { // looking for enums' enumerants
			data, err := os.ReadFile(path)
			if err != nil {
				panic(err)
			}
			pkg_name := ""
			for _, line := range str.Split(str.Trim(string(data)), "\n") {
				if str.Begins(line, "package ") {
					pkg_name = line[len("package "):]
				} else if str.Begins(line, "\t") && str.Ends(line, "\"") && str.Has(line, " = \"") {
					if name_and_type, value, ok := str.Cut(line[1:len(line)-1], " = \""); ok && value != "" {
						if name, type_name, ok := str.Cut(name_and_type, " "); ok {
							if name, type_name = str.Trim(name), str.Trim(type_name); type_name != "" && type_name != "string" && name != type_name && str.Begins(name, type_name) {
								enumerant_name := name[len(type_name):]
								if enumerant_name != value && name != value {
									panic(value + "!=" + enumerant_name + " && " + value + "!=" + name)
								}
								apiReflAllEnums[pkg_name+"."+type_name] = append(apiReflAllEnums[pkg_name+"."+type_name], value)
							}
						}
					}
				}
			}
		}
	})
}

func apiGenSdk() {
	buf, api := str.Buf{}, apiRefl{}
	yolog.Println("  reflect...")
	apiHandleReflReq(nil, nil, &api)
	yolog.Println("  generate...")
	b, err := staticFileDir.ReadFile(StaticFileDirPath + "/sdkgen.ts")
	if err != nil {
		panic(err)
	}
	_, _ = buf.Write(b)
	for _, enum_name := range sl.Sorted(Keys(api.Enums)) {
		apiGenSdkType(&buf, &api, enum_name, nil, api.Enums[enum_name])
	}
	for _, struct_name := range sl.Sorted(Keys(api.Types)) {
		apiGenSdkType(&buf, &api, struct_name, api.Types[struct_name], nil)
	}
	for _, method := range api.Methods {
		apiGenSdkMethod(&buf, &api, &method)
	}
	src_is_changed, src_to_write := true, []byte(buf.String())
	data, _ := os.ReadFile(sdkGenDstTsFilePath)
	src_is_changed = (len(data) == 0) || (!bytes.Equal(data, src_to_write))
	if src_is_changed {
		foundModifiedTsFiles = true
		yolog.Println("  writing files...")
		if err := os.WriteFile("tsconfig.json", []byte(`{"extends": "../yo/tsconfig.json"}`), os.ModePerm); err != nil {
			panic(err)
		}
		if err := os.WriteFile(sdkGenDstTsFilePath, src_to_write, os.ModePerm); err != nil {
			panic(err)
		}
	}
	if foundModifiedTsFiles {
		yolog.Println("  2x tsc...")
		var work sync.WaitGroup
		work.Add(2)
		for _, dir_path := range []string{"", "../yo"} {
			go func(dirPath string) {
				defer work.Done()
				tsc := exec.Command("tsc")
				tsc.Dir = dirPath
				if output, err := tsc.CombinedOutput(); err != nil {
					panic(err.Error() + "\n" + string(output))
				}
			}(dir_path)
		}
		work.Wait()
	}
}

func apiGenSdkMethod(buf *str.Buf, api *apiRefl, method *apiReflMethod) {
	_, _ = buf.WriteString(str.Fmt(`
export function yoReq_%s(payload: %s, onSuccess: (_: %s) => void, onFailed?: (err: any, resp?: Response, query?: {[_:string]:string}) => void): void {
	yoReq(%s, payload, onSuccess, onFailed)
}`, ToIdent(method.Path), apiGenSdkTypeName(api, method.In), apiGenSdkTypeName(api, method.Out), str.Q(method.Path)))
}

func apiGenSdkType(buf *str.Buf, api *apiRefl, typeName string, structFields map[string]string, enumMembers []string) {
	switch typeName {
	case "time.Time":
		_, _ = buf.WriteString(str.Fmt("\nexport type %s = %s", apiGenSdkTypeName(api, typeName), apiGenSdkTypeName(api, ".string")))
		return
	}
	if structFields != nil {
		_, _ = buf.WriteString(str.Fmt("\nexport type %s = {", apiGenSdkTypeName(api, typeName)))
		for _, field_name := range sl.Sorted(Keys(structFields)) {
			field_type := structFields[field_name]
			_, _ = buf.WriteString(str.Fmt("\n\t%s: %s", ToIdent(field_name), apiGenSdkTypeName(api, field_type)))
		}
		_, _ = buf.WriteString("\n}\n")
	} else {
		_, _ = buf.WriteString(str.Fmt("\nexport type %s = %s\n", apiGenSdkTypeName(api, typeName),
			If(len(enumMembers) == 0, "string", "\""+str.Join(enumMembers, "\" | \"")+"\"")))
	}
}

func apiGenSdkTypeName(api *apiRefl, typeName string) string {
	if str.Begins(typeName, ".") {
		switch t := typeName[1:]; t {
		case "string":
			return "string"
		case "bool":
			return "boolean"
		case "int8", "int16", "int32", "int64":
			return "Yo_i" + t[len("int"):]
		case "uint8", "uint16", "uint32", "uint64":
			return "Yo_u" + t[len("uint"):]
		case "float32", "float64":
			return "Yo_f" + t[len("float"):]
		default:
			panic("no type-name gen for '" + typeName + "'")
		}
	}
	if str.Begins(typeName, "[") && str.Ends(typeName, "]") {
		return apiGenSdkTypeName(api, typeName[1:len(typeName)-1]) + "[]"
	}
	if str.Begins(typeName, "{") && str.Ends(typeName, "}") {
		key_part, val_part, ok := str.Cut(typeName[1:len(typeName)-1], ":")
		if !ok {
			panic(typeName)
		}
		if _, is_enum := api.Enums[key_part]; is_enum {
			return str.Fmt("{ [key in %s]?: %s }", apiGenSdkTypeName(api, key_part), apiGenSdkTypeName(api, val_part))
		}
		return str.Fmt("{ [_:%s]: %s }", apiGenSdkTypeName(api, key_part), apiGenSdkTypeName(api, val_part))
	}
	return "Yo_" + ToIdent(typeName[str.Idx(typeName, '.')+1:])
}
