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

var foundModifiedTsFiles bool

func init() {
	apiGenSdkMaybe = apiGenSdk

	enum_pkgs := str.Dict{}
	WalkCodeFiles(true, true, func(path string, dirEntry fs.DirEntry) {
		// if dirEntry.IsDir() {
		// 	file_set := token.FileSet{}
		// 	if pkgs, err := parser.ParseDir(&file_set, path, nil, parser.AllErrors); err == nil {
		// 		for _, pkg := range pkgs {
		// 			analyzeGoPkg(path, pkg, &file_set)
		// 		}
		// 	}
		// }

		if (!dirEntry.IsDir()) && str.Ends(path, ".ts") && (!str.Ends(path, ".d.ts")) && !foundModifiedTsFiles {
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
								if str.IsLo(enumerant_name[:1]) {
									continue
								}
								if (str.Lo(enumerant_name) != str.Lo(value)) && (str.Lo(name) != str.Lo(value)) {
									panic(value + "!=" + enumerant_name + " && " + value + "!=" + name)
								}
								apiReflAllEnums[pkg_name+"."+type_name] = append(apiReflAllEnums[pkg_name+"."+type_name], value)
								if existing := enum_pkgs[type_name]; existing != "" && existing != pkg_name {
									panic("enum name clash: '" + pkg_name + "." + type_name + "' vs '" + existing + "." + type_name + "'")
								}
								enum_pkgs[type_name] = pkg_name
								apiReflAllEnums[type_name] = append(apiReflAllEnums[type_name], value)
							}
						}
					}
				}
			}
		}
	})
}

func apiGenSdk() {
	const sdkGenDstTsFileRelPath = StaticFilesDirName + "/yo-sdk.ts"
	buf, api := str.Buf{}, apiRefl{}
	api.codeGen.typesUsed, api.codeGen.typesEmitted = map[string]bool{}, map[string]bool{}
	yolog.Println("  reflect...")
	apiHandleReflReq(&ApiCtx[Void, apiRefl]{Ret: &api})
	yolog.Println("  generate...")
	b, err := os.ReadFile("../yo/" + sdkGenDstTsFileRelPath)
	if err != nil {
		panic(err)
	}
	_, _ = buf.Write(b)
	for _, method := range api.Methods {
		apiGenSdkMethod(&buf, &api, &method)
	}
	for again := true; again; {
		again = false
		for _, enum_name := range sl.Sorted(Keys(api.Enums)) {
			if (!api.codeGen.typesUsed[enum_name]) && str.Ends(enum_name, "Field") {
				api.codeGen.typesUsed[enum_name] = true
			}
			if apiGenSdkType(&buf, &api, enum_name, nil, api.Enums[enum_name]) {
				again = true
			}
		}
		for _, struct_name := range sl.Sorted(Keys(api.Types)) {
			if apiGenSdkType(&buf, &api, struct_name, api.Types[struct_name], nil) {
				again = true
			}
		}
	}
	src_is_changed, src_to_write := true, []byte(buf.String())
	data, _ := os.ReadFile(sdkGenDstTsFileRelPath)
	src_is_changed = (len(data) == 0) || (!bytes.Equal(data, src_to_write))
	if src_is_changed {
		foundModifiedTsFiles = true
		yolog.Println("  writing files...")
		if err := os.WriteFile("tsconfig.json", []byte(`{"extends": "../yo/tsconfig.json"}`), os.ModePerm); err != nil {
			panic(err)
		}
		if err := os.WriteFile(sdkGenDstTsFileRelPath, src_to_write, os.ModePerm); err != nil {
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
	if str.Begins(method.Path, "__/") {
		return
	}

	repl := str.Dict{
		"method_name":    ToIdent(method.Path),
		"in_type_ident":  apiGenSdkTypeName(api, method.In),
		"out_type_ident": apiGenSdkTypeName(api, method.Out),
		"method_path":    method.Path,
	}
	_ = repl

	_, _ = buf.WriteString(str.Repl(`
export async function req_{method_name}(payload: {in_type_ident}, query?: {[_:string]:string}): Promise<{out_type_ident}> {
	return req<{in_type_ident}, {out_type_ident}>("{method_path}", payload, query)
}`,
		repl))
}

func apiGenSdkType(buf *str.Buf, api *apiRefl, typeName string, structFields str.Dict, enumMembers []string) bool {
	for str.Begins(typeName, "?") {
		typeName = typeName[1:]
	}
	if api.codeGen.typesEmitted[typeName] || !api.codeGen.typesUsed[typeName] {
		return false
	}
	api.codeGen.typesEmitted[typeName] = true
	if typeName == "time.Time" {
		_, _ = buf.WriteString(str.Repl("\nexport type {lhs} = {rhs}",
			str.Dict{"lhs": apiGenSdkTypeName(api, typeName), "rhs": apiGenSdkTypeName(api, ".string")}))
	} else if structFields != nil {
		_, _ = buf.WriteString(str.Repl("\nexport type {lhs} = {", str.Dict{"lhs": apiGenSdkTypeName(api, typeName)}))
		struct_fields := sl.Sorted(Keys(structFields))
		for _, field_name := range struct_fields {
			field_type := structFields[field_name]
			_, _ = buf.WriteString(str.Repl("\n\t{fld}{?}: {tfld}",
				str.Dict{"fld": ToIdent(field_name), "?": If(str.Begins(field_type, "?"), "?", ""), "tfld": apiGenSdkTypeName(api, field_type)}))
		}
		_, _ = buf.WriteString("\n}\n")
	} else {
		_, _ = buf.WriteString(str.Repl("\nexport type {lhs} = {rhs}\n", str.Dict{
			"lhs": apiGenSdkTypeName(api, typeName),
			"rhs": If(len(enumMembers) == 0, "string", "\""+str.Join(enumMembers, "\" | \"")+"\""),
		}))
	}
	return true
}

func apiGenSdkTypeName(api *apiRefl, typeName string) string {
	for str.Begins(typeName, "?") {
		typeName = typeName[1:]
	}
	api.codeGen.typesUsed[typeName] = true
	if str.Begins(typeName, ".") {
		switch t := typeName[1:]; t {
		case "string":
			return "string"
		case "bool":
			return "boolean"
		case "int8", "int16", "int32", "int64":
			return "I" + t[len("int"):]
		case "uint8", "uint16", "uint32", "uint64":
			return "U" + t[len("uint"):]
		case "float32", "float64":
			return "F" + t[len("float"):]
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
			return str.Repl("{ [key in {lhs}]?: {rhs} }", str.Dict{"lhs": apiGenSdkTypeName(api, key_part), "rhs": apiGenSdkTypeName(api, val_part)})
		}
		return str.Repl("{ [_:{lhs}]: {rhs} }", str.Dict{"lhs": apiGenSdkTypeName(api, key_part), "rhs": apiGenSdkTypeName(api, val_part)})
	}
	return ToIdent(typeName[str.Idx(typeName, '.')+1:])
}
