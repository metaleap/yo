//go:build debug

package yosrv

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	yolog "yo/log"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

const (
	codegenEmitTopCommentLine = "// Code generated by `yo/srv/codegen_apistuff.go` DO NOT EDIT\n"
)

var (
	foundModifiedTsFilesYoSide  bool
	foundModifiedTsFilesAppSide bool
	pkgsFound                   = str.Dict{}
	pkgsImportingSrv            = map[string]bool{}
	curMainDir                  = CurDirPath()
	curMainName                 = filepath.Base(curMainDir)
)

func init() {
	codegenMaybe = func() {
		yolog.Println("codegen (api stuff)")

		for _, rt := range apiReflAllDbStructs {
			if pkg_path := rt.PkgPath(); str.Begins(pkg_path, "yo/") {
				apiReflYoDbStructs = append(apiReflYoDbStructs, rt)
			} else if str.Begins(pkg_path, curMainName+"/") {
				apiReflAppDbStructs = append(apiReflAppDbStructs, rt)
			} else {
				panic(rt.String())
			}
		}

		api_refl := apiRefl{}
		apiHandleReflReq(&ApiCtx[Void, apiRefl]{Ret: &api_refl})
		codegenGo(&api_refl)
		codegenTsSdk(&api_refl)
	}
}

func reflEnumsOnceOnInit() {
	enum_pkgs := str.Dict{}
	WalkCodeFiles(true, true, func(path string, dirEntry fs.DirEntry) {
		is_app_side := str.Begins(path, str.TrimR(curMainDir, "/")+"/")
		if (!(foundModifiedTsFilesYoSide && foundModifiedTsFilesAppSide)) && (!dirEntry.IsDir()) &&
			str.Ends(path, ".ts") && (!str.Ends(path, ".d.ts")) {
			fileinfo_ts, err := dirEntry.Info()
			if err != nil || fileinfo_ts == nil {
				panic(err)
			}
			fileinfo_js, err := os.Stat(path[:len(path)-len(".ts")] + ".js")
			is_modified := ((fileinfo_js == nil) || (err != nil) ||
				(fileinfo_ts.ModTime().After(fileinfo_js.ModTime())))
			foundModifiedTsFilesYoSide = foundModifiedTsFilesYoSide || (is_modified && !is_app_side)
			foundModifiedTsFilesAppSide = foundModifiedTsFilesAppSide || (is_modified && is_app_side)
		}

		if str.Ends(path, ".go") { // looking for enums' enumerants
			data := ReadFile(path)
			pkg_name := ""
			for _, line := range str.Split(str.Trim(string(data)), "\n") {
				if str.Begins(line, "package ") {
					pkg_name = line[len("package "):]
					pkgsFound[pkg_name] = filepath.Dir(path)
				} else if str.Begins(line, "\t") && str.Ends(line, `"yo/srv"`) && pkg_name != "" {
					pkgsImportingSrv[pkg_name] = true
				} else if str.Begins(line, "\t") && str.Ends(line, "\"") && str.Has(line, " = \"") {
					if name_and_type, value, ok := str.Cut(line[1:len(line)-1], " = \""); ok && (value != "") && (str.Idx(value, '.') < 0) {
						if name, type_name, ok := str.Cut(name_and_type, " "); ok {
							if name, type_name = str.Trim(name), str.Trim(type_name); type_name != "" && type_name != "string" && name != type_name {
								if type_name_stripped := str.TrimR(type_name, "Field"); str.Begins(name, type_name) || str.Begins(name, type_name_stripped) {
									enumerant_name := name[len(type_name_stripped):]
									if str.IsLo(enumerant_name[:1]) {
										continue
									}
									if (str.Lo(enumerant_name) != str.Lo(value)) && (str.Lo(name) != str.Lo(value) && !str.Has(value, ".")) {
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
		}
	})
}

func codegenGo(apiRefl *apiRefl) {
	var did_write_files []string
	for pkg_name, pkg_dir_path := range pkgsFound {
		out_file_path := filepath.Join(pkg_dir_path, "ˍapi_generated_code.go")

		pkg_methods := map[string]ApiMethod{}
		for method_path, method := range api {
			if pkg_info := method.pkgInfo(); (pkg_info != nil) && (pkg_info.PkgName() == pkg_name) {
				pkg_methods[method_path] = method
			}
		}

		if !pkgsImportingSrv[pkg_name] {
			DelFile(out_file_path)
			continue
		}

		var buf str.Buf
		buf.WriteString(codegenEmitTopCommentLine)
		buf.WriteString("package " + pkg_name + "\n")
		buf.WriteString("import reflect \"reflect\"\n")
		buf.WriteString("import yosrv \"yo/srv\"\n")
		buf.WriteString("import util \"yo/util\"\n")
		buf.WriteString("import q \"yo/db/query\"\n")
		buf.WriteString("type _ = q.F // just in case of no other generated import users\n")
		buf.WriteString("type apiPkgInfo util.Void\n")
		buf.WriteString("func (apiPkgInfo) PkgName() string { return \"" + pkg_name + "\" }\n")
		buf.WriteString("func (me apiPkgInfo) PkgPath() string { return reflect.TypeOf(me).PkgPath() }\n")
		buf.WriteString("var " + pkg_name + "Pkg = apiPkgInfo{}\n")
		buf.WriteString("func api[TIn any,TOut any](f func(*yosrv.ApiCtx[TIn, TOut]), failIfs ...yosrv.Fails) yosrv.ApiMethod{return yosrv.Api[TIn,TOut](f,failIfs...).From(" + pkg_name + "Pkg)}\n")

		// emit known `Err`s
		err_emitted := map[Err]bool{}
		for _, err := range errsNoCodegen {
			err_emitted[err] = true
		}
		for _, method_path := range sl.Sorted(Keys(pkg_methods)) {
			for _, err := range sl.Sorted(Keys(apiRefl.KnownErrs[method_path])) {
				if !err_emitted[err] {
					err_emitted[err] = true
					buf.WriteString("const Err" + string(err) + " util.Err = \"" + string(err) + "\"\n")
				}
			}
		}

		// emit api method input fields for FailIf conditions
		for _, method_path := range sl.Sorted(Keys(pkg_methods)) {
			method := apiRefl.method(method_path)
			is_app_dep, name_prefix, input_type := false, method.identUp0(), apiRefl.Types[method.In]
			if pkg_name == "yodb" && str.Begins(method_path, yoAdminApisUrlPrefix+"db/") {
				for _, rt := range apiReflAppDbStructs {
					if is_app_dep = str.Begins(method_path, yoAdminApisUrlPrefix+"db/"+rt.Name()+"/"); is_app_dep {
						break
					}
				}
			}
			if is_app_dep {
				continue
			}
			for _, field_name := range sl.Sorted(Keys(input_type)) {
				buf.WriteString("const " + name_prefix + ToIdent(field_name) + " = q.F(\"" + field_name + "\")\n")
			}
		}

		src_raw, err := format.Source([]byte(buf.String()))
		if err != nil {
			panic(err)
		}

		if src_old := ReadFile(out_file_path); !bytes.Equal(src_old, src_raw) {
			WriteFile(out_file_path, src_raw)
			did_write_files = append(did_write_files, str.TrimL(filepath.Dir(out_file_path), os.Getenv("GOPATH")+"/"))
		}
	}
	if len(did_write_files) > 0 {
		panic("apicodegen'd, please restart (" + str.Join(did_write_files, ", ") + ")")
	}
}

func codegenTsRunTsc(inDirPath string) {
	yolog.Println("tsc in " + inDirPath)
	cmd_tsc := exec.Command("tsc")
	cmd_tsc.Dir = inDirPath
	if output, err := cmd_tsc.CombinedOutput(); err != nil {
		panic(err.Error() + "\n" + string(output))
	}
}

func codegenTsSdk(apiRefl *apiRefl) (didFsWrites bool) {
	if EnsureDir(StaticFilesDirNameYo) {
		didFsWrites = true
	}

	const yo_dir_path = "../yo/"
	const yo_static_dir_path = yo_dir_path + StaticFilesDirNameYo
	const yo_sdk_ts_file_name = "/yo-sdk.ts"
	const yo_sdk_js_file_name = "/yo-sdk.js"

	buf := str.Buf{}
	apiRefl.codeGen.typesUsed, apiRefl.codeGen.typesEmitted, apiRefl.codeGen.strLits = map[string]bool{}, map[string]bool{}, str.Dict{}

	buf.Write([]byte(codegenEmitTopCommentLine))
	buf.Write(ReadFile(yo_static_dir_path + yo_sdk_ts_file_name))
	for _, method := range apiRefl.Methods {
		codegenTsSdkMethod(&buf, apiRefl, &method)
	}
	for again := true; again; {
		again = false
		for _, enum_name := range sl.Sorted(Keys(apiRefl.Enums)) {
			if (!apiRefl.codeGen.typesUsed[enum_name]) && str.Ends(enum_name, "Field") {
				apiRefl.codeGen.typesUsed[enum_name] = true
			}
			if codegenTsSdkType(&buf, apiRefl, enum_name, nil, apiRefl.Enums[enum_name]) {
				again = true
			}
		}
		for _, struct_name := range sl.Sorted(Keys(apiRefl.Types)) {
			if codegenTsSdkType(&buf, apiRefl, struct_name, apiRefl.Types[struct_name], nil) {
				again = true
			}
		}
	}

	var buf_prepend str.Buf
	for value, name := range apiRefl.codeGen.strLits {
		buf_prepend.WriteString("const " + name + ": string = " + str.Q(value) + "\n")
	}
	src_to_write := []byte(buf_prepend.String() + buf.String())

	out_file_path := StaticFilesDirNameYo + yo_sdk_ts_file_name
	data := ReadFile(out_file_path)
	src_is_changed := (len(data) == 0) || (!bytes.Equal(data, src_to_write))
	if src_is_changed {
		foundModifiedTsFilesYoSide = true
		WriteFile("tsconfig.json", []byte(`{"extends": "../yo/tsconfig.json"}`))
		WriteFile(out_file_path, src_to_write)
	}

	if foundModifiedTsFilesYoSide {
		codegenTsRunTsc(yo_dir_path)
		didFsWrites = true
	}

	// post-generate: clean up app-side, by removing files no longer in yo side
	WalkDir(StaticFilesDirNameYo, func(path string, dirEntry fs.DirEntry) {
		yo_side_path := yo_dir_path + path
		if !(IsFile(yo_side_path) || IsDir(yo_side_path)) {
			if dirEntry.IsDir() {
				DelDir(path)
			} else {
				DelFile(path)
			}
			didFsWrites = true
		}
	})

	// post-generate: ensure files are linked app-side (and folders mirrored)
	WalkDir(yo_static_dir_path, func(path string, dirEntry fs.DirEntry) {
		if path == (yo_static_dir_path+yo_sdk_ts_file_name) || (path == (yo_static_dir_path + yo_sdk_js_file_name)) {
			return // skip yo-sdk.?s
		}

		is_dir, app_side_link_path := dirEntry.IsDir(), path[len(yo_dir_path):]
		if EnsureLink(app_side_link_path, path, is_dir) {
			didFsWrites = true
		}
	})

	if foundModifiedTsFilesYoSide || foundModifiedTsFilesAppSide || didFsWrites {
		codegenTsRunTsc(curMainDir)
		didFsWrites = true
	}
	return
}

func codegenTsSdkMethod(buf *str.Buf, apiRefl *apiRefl, method *apiReflMethod) {
	is_app_api := !str.Begins(method.Path, yoAdminApisUrlPrefix)
	if !is_app_api {
		return
	}

	method_name, method_errs := method.identUp0(), sl.Sorted(Keys(apiRefl.KnownErrs[method.Path]))
	ts_enum_type_name := method_name + "Err"
	repl := str.Dict{
		"method_name":        method_name,
		"in_type_ident":      codegenTsSdkTypeName(apiRefl, method.In),
		"out_type_ident":     codegenTsSdkTypeName(apiRefl, method.Out),
		"method_path":        method.Path,
		"method_path_prefix": If(is_app_api, AppApiUrlPrefix, ""),
		"enum_type_name":     ts_enum_type_name,
		"known_errs":         "['" + str.Join(sl.To(method_errs, Err.String), "', '") + "']",
	}

	buf.WriteString(str.Repl(`
const errs{method_name} = {known_errs} as const
export type {enum_type_name} = typeof errs{method_name}[number]
export async function api{method_name}(payload: {in_type_ident}, query?: {[_:string]:string}): Promise<{out_type_ident}> {
	try {
		return req<{in_type_ident}, {out_type_ident}>('{method_path_prefix}{method_path}', payload, query)
	} catch(err) {
		if (err && err['body_text'] && (errs{method_name}.indexOf(err.body_text) >= 0))
			throw(new Err<{enum_type_name}>(err.body_text as {enum_type_name}))
		throw(err)
	}
}
`, repl))
}

func codegenTsSdkType(buf *str.Buf, apiRefl *apiRefl, typeName string, structFields str.Dict, enumMembers []string) bool {
	for str.Begins(typeName, "?") {
		typeName = typeName[1:]
	}
	if apiRefl.codeGen.typesEmitted[typeName] || !apiRefl.codeGen.typesUsed[typeName] {
		return false
	}
	apiRefl.codeGen.typesEmitted[typeName] = true
	if (typeName == "time.Time") || (typeName == "yo/db.DateTime") {
		buf.WriteString(str.Repl("\nexport type {lhs} = {rhs}",
			str.Dict{"lhs": codegenTsSdkTypeName(apiRefl, typeName), "rhs": codegenTsSdkTypeName(apiRefl, ".string")}))
	} else if structFields != nil {
		buf.WriteString(str.Repl("\nexport type {lhs} = {", str.Dict{"lhs": codegenTsSdkTypeName(apiRefl, typeName)}))
		struct_fields := sl.Sorted(Keys(structFields))
		for _, field_name := range struct_fields {
			field_type := structFields[field_name]
			buf.WriteString(str.Repl("\n\t{fld}{?}: {tfld}",
				str.Dict{"fld": ToIdent(field_name), "?": If(str.Begins(field_type, "?"), "?", ""), "tfld": codegenTsSdkTypeName(apiRefl, field_type)}))
		}
		buf.WriteString("\n}\n")
	} else {
		buf.WriteString(str.Repl("\nexport type {lhs} = {rhs}\n", str.Dict{
			"lhs": codegenTsSdkTypeName(apiRefl, typeName),
			"rhs": If(len(enumMembers) == 0, "string", "'"+str.Join(enumMembers, "' | '")+"'"),
		}))
	}
	return true
}

func codegenTsSdkTypeName(apiRefl *apiRefl, typeName string) string {
	for str.Begins(typeName, "?") {
		typeName = typeName[1:]
	}
	apiRefl.codeGen.typesUsed[typeName] = true
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
		return codegenTsSdkTypeName(apiRefl, typeName[1:len(typeName)-1]) + "[]"
	}
	if str.Begins(typeName, "{") && str.Ends(typeName, "}") {
		key_part, val_part, ok := str.Cut(typeName[1:len(typeName)-1], ":")
		if !ok {
			panic(typeName)
		}
		if _, is_enum := apiRefl.Enums[key_part]; is_enum {
			return str.Repl("{ [key in {lhs}]?: {rhs} }", str.Dict{"lhs": codegenTsSdkTypeName(apiRefl, key_part), "rhs": codegenTsSdkTypeName(apiRefl, val_part)})
		}
		return str.Repl("{ [_:{lhs}]: {rhs} }", str.Dict{"lhs": codegenTsSdkTypeName(apiRefl, key_part), "rhs": codegenTsSdkTypeName(apiRefl, val_part)})
	}
	return ToIdent(typeName[str.Idx(typeName, '.')+1:])
}

// currently unused, but if ever numbered identifiers need to be generated into the emitted output file...
func (me *apiRefl) strLit(value string) (name string) {
	if name = me.codeGen.strLits[value]; name == "" {
		name = "__s" + str.Base36(len(me.codeGen.strLits))
		me.codeGen.strLits[value] = name
	}
	return
}
