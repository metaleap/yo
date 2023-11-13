//go:build debug

package yosrv

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	. "yo/cfg"
	yolog "yo/log"
	. "yo/util"
	"yo/util/kv"
	"yo/util/sl"
	"yo/util/str"

	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"
)

const (
	codegenEmitTopCommentLine = "// Code generated by `yo/srv/codegen_apistuff.go` DO NOT EDIT\n"
	codegenForceFull          = true // for rare temporary local-dev toggling, usually false

	yoDirPath              = "../yo/"
	yoStaticDirPath        = yoDirPath + StaticFilesDirNameYo
	yoSdkTsFileName        = "yo-sdk.ts"
	yoSdkJsFileName        = "yo-sdk.js"
	yoSdkTsPreludeFileName = "prelude-yo-sdk.ts"
	yoSdkJsPreludeFileName = "prelude-yo-sdk.js"
)

var (
	foundModifiedTsFilesYoSide = codegenForceFull
	pkgsFound                  = str.Dict{}
	pkgsImportingSrv           = map[string]bool{}
	curMainDir                 = FsDirPathCur()
	curMainName                = filepath.Base(curMainDir)
	curMainStaticDirPathYo     = filepath.Join(curMainDir, StaticFilesDirNameYo)
	curMainStaticDirPathApp    = filepath.Join(curMainDir, StaticFilesDirNameApp)
)

func init() {
	detectEnumsAndMaybeCodegen = func() {
		yolog.Println("codegen (api stuff)")

		{ // initial dir-walk & enums-detection
			enum_pkgs := str.Dict{}
			FsWalkCodeDirs(true, true, func(fsPath string, dirEntry fs.DirEntry) {
				if is_yo_side := str.Begins(FsPathAbs(fsPath), str.TrimSuff(FsPathAbs(yoStaticDirPath), "/")+"/"); is_yo_side &&
					(!foundModifiedTsFilesYoSide) && (!dirEntry.IsDir()) &&
					str.Ends(fsPath, ".ts") && (!str.Ends(fsPath, ".d.ts")) {

					is_modified := FsIsNewerThan(fsPath, FsPathSwapExt(fsPath, ".ts", ".js"))
					foundModifiedTsFilesYoSide = foundModifiedTsFilesYoSide || is_modified
				}

				if str.Ends(fsPath, ".go") { // looking for enums' enumerants
					data := FsRead(fsPath)
					pkg_name := ""
					for _, line := range str.Split(str.Trim(string(data)), "\n") {
						if str.Begins(line, "package ") {
							pkg_name = line[len("package "):]
							if str.Begins(pkg_name, "_") {
								return
							}
							pkgsFound[pkg_name] = filepath.Dir(fsPath)
						} else if str.Begins(line, "\t") && str.Ends(line, `"yo/srv"`) && pkg_name != "" {
							pkgsImportingSrv[pkg_name] = true
						} else if str.Begins(line, "\t") && str.Ends(line, "\"") && str.Has(line, " = \"") {
							if name_and_type, value, ok := str.Cut(line[1:len(line)-1], " = \""); ok && (value != "") && (str.Idx(value, '.') < 0) {
								if name, type_name, ok := str.Cut(name_and_type, " "); ok {
									if name, type_name = str.Trim(name), str.Trim(type_name); type_name != "" && type_name != "string" && name != type_name {
										if type_name_stripped := str.TrimSuff(type_name, "Field"); str.Begins(name, type_name) || str.Begins(name, type_name_stripped) {
											enumerant_name := name[len(type_name_stripped):]
											if str.IsLo(enumerant_name[:1]) {
												continue
											}
											if (str.Lo(enumerant_name) != str.Lo(value)) && (str.Lo(name) != str.Lo(value) && !str.Has(value, ".")) {
												continue
												// panic(value + "!=" + enumerant_name + " && " + value + "!=" + name)
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

		for _, rt := range apiReflAllDbStructs {
			if pkg_path := rt.PkgPath(); (pkg_path == "yo") || str.Begins(pkg_path, "yo/") {
				apiReflYoDbStructs = append(apiReflYoDbStructs, rt)
			} else if str.Begins(pkg_path, curMainName+"/") {
				apiReflAppDbStructs = append(apiReflAppDbStructs, rt)
			} else {
				panic("[" + pkg_path + "]" + rt.String())
			}
		}

		api_refl := apiReflect{}
		apiHandleReflReq(&ApiCtx[None, apiReflect]{Ret: &api_refl})
		codegenGo(&api_refl)
		_ = codegenTsSdk(&api_refl)
		_ = codegenOpenApi(&api_refl)
	}
}

func codegenGo(apiRefl *apiReflect) {
	var did_write_files []string
	for pkg_name, pkg_dir_path := range pkgsFound {
		out_file_path := filepath.Join(pkg_dir_path, "ˍgenerated_apistuff.go")

		pkg_methods := map[string]ApiMethod{}
		for method_path, method := range api {
			if pkg_info := method.pkgInfo(); (pkg_info != nil) && (pkg_info.PkgName() == pkg_name) {
				pkg_methods[method_path] = method
			}
		}

		if !pkgsImportingSrv[pkg_name] {
			FsDelFile(out_file_path)
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
		buf.WriteString("type apiPkgInfo util.None\n")
		buf.WriteString("func (apiPkgInfo) PkgName() string { return \"" + pkg_name + "\" }\n")
		buf.WriteString("func (me apiPkgInfo) PkgPath() string { return reflect.TypeOf(me).PkgPath() }\n")
		buf.WriteString("var " + pkg_name + "Pkg = apiPkgInfo{}\n")
		buf.WriteString("func api[TIn any,TOut any](f func(*yosrv.ApiCtx[TIn, TOut]), failIfs ...yosrv.Fails) yosrv.ApiMethod{return yosrv.Api[TIn,TOut](f,failIfs...).From(" + pkg_name + "Pkg)}\n")

		// emit known `Err`s
		err_emitted := map[Err]bool{}
		for _, err := range errsNoCodegen {
			err_emitted[err] = true
		}
		for _, method_path := range sl.Sorted(kv.Keys(pkg_methods)) {
			for _, err := range sl.Sorted(kv.Keys(apiRefl.KnownErrs[method_path])) {
				if !err_emitted[err] {
					err_emitted[err] = true
					buf.WriteString("const Err" + string(err) + " util.Err = \"" + string(err) + "\"\n")
				}
			}
		}

		var do_fields func(str.Dict, string, string)
		do_fields = func(typeRefl str.Dict, namePrefix string, fieldStrPrefix string) {
			for _, field_name := range sl.Sorted(kv.Keys(typeRefl)) {
				ident := namePrefix + ToIdent(field_name)
				buf.WriteString("const " + ident + " = q.F(\"" + fieldStrPrefix + field_name + "\")\n")
				// this below would generate dotted literal constants for sub-field paths, but without the q side able to handle this, we for now dont emit those
				// do_fields(apiRefl.Types[typeRefl[field_name]], ident, fieldStrPrefix+field_name+".")
			}
		}

		// emit api method input fields for FailIf conditions
		for _, method_path := range sl.Sorted(kv.Keys(pkg_methods)) {
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
			do_fields(input_type, name_prefix, "")
		}

		src_raw, err := format.Source([]byte(buf.String()))
		if err != nil {
			panic(err)
		}

		if src_old := FsRead(out_file_path); !bytes.Equal(src_old, src_raw) {
			FsWrite(out_file_path, src_raw)
			did_write_files = append(did_write_files, str.TrimPref(filepath.Dir(out_file_path), os.Getenv("GOPATH")+"/"))
		}
	}
	if len(did_write_files) > 0 {
		panic("apicodegen'd, please restart (" + str.Join(did_write_files, ", ") + ")")
	}
}

func codegenOpenApi(apiRefl *apiReflect) (didFsWrites []string) {
	out_file_path := curMainStaticDirPathApp + "/openapi.json"
	oarefl := openapi31.NewReflector()

	oarefl.Spec.Info.WithTitle(Cfg.YO_APP_DOMAIN)
	oarefl.JSONSchemaReflector().DefaultOptions = append(oarefl.JSONSchemaReflector().DefaultOptions, jsonschema.ProcessWithoutTags)
	for _, method := range apiRefl.Methods {
		if is_app_api := !str.Begins(method.Path, yoAdminApisUrlPrefix); !is_app_api {
			continue
		}
		op, err := oarefl.NewOperationContext("POST", "/"+method.Path)
		if err != nil {
			panic(err)
		}
		api_method := api[method.Path]
		ty_args, ty_ret := api_method.reflTypes()
		op.AddReqStructure(reflect.New(ty_args).Interface(), openapi.WithContentType(apisContentType))
		op.AddRespStructure(reflect.New(ty_ret).Interface(), openapi.WithHTTPStatus(200))
		if err = oarefl.AddOperation(op); err != nil {
			panic(err)
		}
	}

	src_json, err := oarefl.Spec.MarshalJSON()
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(FsRead(out_file_path), src_json) {
		didFsWrites = append(didFsWrites, out_file_path)
		FsWrite(out_file_path, src_json)
	}
	return
}

func codegenTsSdk(apiRefl *apiReflect) (didFsWrites []string) {
	if FsDirEnsure(StaticFilesDirNameYo) {
		didFsWrites = append(didFsWrites, "MK:"+StaticFilesDirNameYo)
	}

	const out_file_path_1 = StaticFilesDirNameYo + "/" + yoSdkTsFileName // app-side path since cur-dir is always app-side
	out_file_path_2 := curMainStaticDirPathApp + "/" + yoSdkTsFileName   // app-side path since cur-dir is always app-side
	buf := str.Buf{}                                                     // into this we emit the new source for out_file_path
	apiRefl.codeGen.typesUsed, apiRefl.codeGen.typesEmitted = map[string]bool{}, map[string]bool{}

	buf.Write([]byte(codegenEmitTopCommentLine))
	buf.WriteString("export const Cfg_YO_API_IMPL_TIMEOUT_MS = " + str.GoLike(Cfg.YO_API_IMPL_TIMEOUT.Milliseconds()) + "\n")
	buf.WriteString("export const Cfg_YO_AUTH_PWD_MIN_LEN = " + str.GoLike(Cfg.YO_AUTH_PWD_MIN_LEN) + "\n")
	buf.WriteString("\n// " + yoSdkTsPreludeFileName + " (non-generated) below, more generated code afterwards\n")
	buf.Write(FsRead(filepath.Join(yoStaticDirPath, yoSdkTsPreludeFileName))) // emit yo-side code prelude
	buf.WriteString("\n// " + yoSdkTsPreludeFileName + " ends, the rest below is fully generated code only:\n")

	buf.WriteString("\nreqTimeoutMsForJsonApis = Cfg_YO_API_IMPL_TIMEOUT_MS\n")
	buf.WriteString("\nerrMaxReqPayloadSizeExceeded = '" + string(ErrMissingOrExcessiveContentLength) + "'\n")
	if Cfg.YO_API_MAX_REQ_CONTENTLENGTH_MB > 0 {
		buf.WriteString("\nreqMaxReqPayloadSizeMb = " + str.FromInt(Cfg.YO_API_MAX_REQ_CONTENTLENGTH_MB) + "\n")
	}
	if Cfg.YO_API_MAX_REQ_MULTIPART_LENGTH_MB > 0 {
		buf.WriteString("\nreqMaxReqMultipartSizeMb = " + str.FromInt(Cfg.YO_API_MAX_REQ_MULTIPART_LENGTH_MB) + "\n")
	}

	// emit methods
	for _, method := range apiRefl.Methods {
		codegenTsSdkMethod(&buf, apiRefl, &method)
	}
	// emit types (enums + structs)
	for again := true; again; {
		again = false
		for _, enum_name := range sl.Sorted(kv.Keys(apiRefl.Enums)) {
			if (!apiRefl.codeGen.typesUsed[enum_name]) && str.Ends(enum_name, "Field") {
				apiRefl.codeGen.typesUsed[enum_name] = true
			}
			if codegenTsSdkType(&buf, apiRefl, enum_name, nil, apiRefl.Enums[enum_name]) {
				again = true
			}
		}
		for _, struct_name := range sl.Sorted(kv.Keys(apiRefl.Types)) {
			if codegenTsSdkType(&buf, apiRefl, struct_name, apiRefl.Types[struct_name], nil) {
				again = true
			}
		}
	}

	src_to_write := []byte(buf.String())

	src_prev := FsRead(out_file_path_1)
	src_is_changed := codegenForceFull || (len(src_prev) == 0) || (!bytes.Equal(src_prev, src_to_write))
	if src_is_changed {
		foundModifiedTsFilesYoSide = true
		FsWrite("tsconfig.json", []byte(`{"extends": "../yo/tsconfig.json"}`))
		FsWrite(out_file_path_1, src_to_write)
		FsWrite(out_file_path_2, src_to_write)
	}

	if foundModifiedTsFilesYoSide {
		codegenTsToJs(yoStaticDirPath, false, "modTsYo")
		didFsWrites = append(didFsWrites, "GEN:"+yoStaticDirPath)
	}

	// post-generate: clean up app-side, by removing files no longer in yo side
	FsDirWalk(StaticFilesDirNameYo, func(path string, dirEntry fs.DirEntry) {
		if filepath.Base(path) == yoSdkJsFileName {
			return
		}
		yo_side_equiv_path := yoDirPath + path
		if (path != out_file_path_1) && !(FsIsFile(yo_side_equiv_path) || FsIsDir(yo_side_equiv_path)) {
			if dirEntry.IsDir() {
				FsDelDir(path)
			} else {
				FsDelFile(path)
			}
			didFsWrites = append(didFsWrites, "RM:"+path)
		}
	})

	// post-generate: ensure files are linked app-side (and folders mirrored).
	// about symlinks: ALL app-side equivs to yo-side __yostatic/* are symlinks EXCEPT for yo-sdk.*s that were just emitted app-side with its app-specific types/methods/enums
	FsDirWalk(yoStaticDirPath, func(path string, dirEntry fs.DirEntry) {
		if (path == (filepath.Join(yoStaticDirPath, yoSdkTsFileName))) ||
			(path == (filepath.Join(yoStaticDirPath, yoSdkTsPreludeFileName))) ||
			(path == (filepath.Join(yoStaticDirPath, yoSdkJsPreludeFileName))) ||
			(path == (filepath.Join(yoStaticDirPath, yoSdkJsFileName))) {
			return
		}

		is_dir, app_side_link_path := dirEntry.IsDir(), path[len(yoDirPath):]
		if FsLinkEnsure(app_side_link_path, path, is_dir) {
			didFsWrites = append(didFsWrites, "LN:"+app_side_link_path)
		}
	})

	if foundModifiedTsFilesYoSide || (len(didFsWrites) > 0) {
		codegenTsToJs(curMainStaticDirPathYo, true, append(didFsWrites, If(foundModifiedTsFilesYoSide, "modTsYo", ""))...)
		didFsWrites = append(didFsWrites, curMainStaticDirPathYo)
	}
	return
}

func codegenTsSdkMethod(buf *str.Buf, apiRefl *apiReflect, method *apiReflMethod) {
	is_app_api := !str.Begins(method.Path, yoAdminApisUrlPrefix)
	if !is_app_api {
		return
	}

	method_name, method_errs := method.identUp0(), sl.Sorted(kv.Keys(apiRefl.KnownErrs[method.Path]))
	ts_enum_type_name := method_name + "Err"
	repl := str.Dict{
		"method_name":    method_name,
		"in_type_ident":  codegenTsSdkTypeName(apiRefl, method.In),
		"out_type_ident": codegenTsSdkTypeName(apiRefl, method.Out),
		"method_path":    method.Path,
		"enum_type_name": ts_enum_type_name,
		"known_errs":     "['" + str.Join(sl.As(method_errs, Err.String), "', '") + "']",
	}

	buf.WriteString(str.Repl(`
const errs{method_name} = {known_errs} as const
export async function api{method_name}(payload?: {in_type_ident}, formData?: FormData, query?: {[_:string]:string}): Promise<{out_type_ident}> {
	try {
		return await req<{in_type_ident}, {out_type_ident}, {enum_type_name}>('{method_path}', payload, formData, query)
	} catch(err: any) {
		if (err && err['body_text'] && (errs{method_name}.indexOf(err.body_text) >= 0))
			throw(new Err<{enum_type_name}>(err.body_text as {enum_type_name}))
		throw(err)
	}
}
export type {enum_type_name} = typeof errs{method_name}[number]
`, repl))
}

func codegenTsSdkType(buf *str.Buf, apiRefl *apiReflect, typeName string, structFields str.Dict, enumMembers []string) bool {
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
		struct_fields := sl.Sorted(kv.Keys(structFields))
		for _, field_name := range struct_fields {
			field_type := structFields[field_name]
			is_optional := apiRefl.allInputTypes[typeName] // str.Begins(field_type, "?") || (is_api_input && (str.Begins(field_type, ".") || str.Begins(field_type, "{")))
			buf.WriteString(str.Repl("\n\t{fld}{?}: {tfld}",
				str.Dict{"fld": ToIdent(field_name), "?": If(is_optional, "?", ""), "tfld": codegenTsSdkTypeName(apiRefl, field_type)}))
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

func codegenTsSdkTypeName(apiRefl *apiReflect, typeName string) string {
	for str.Begins(typeName, "?") {
		typeName = typeName[1:]
	}
	apiRefl.codeGen.typesUsed[typeName] = true
	if str.Begins(typeName, ".") {
		switch t := typeName[1:]; t {
		case "any", "string":
			return t
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

func codegenTsToJs(inDirPath string, isAppSide bool, reasons ...string) {
	const use_tsc = false // ideally false, as our ts typechecking happens editor-side, and we want a *rapid*, dumb type-stripping-only transpilation in local dev, no bundling, no cross-checking, no polyfill-emits etc

	yolog.Println("ts2js in " + inDirPath + " (" + str.Join(reasons, " , ") + ")")

	if use_tsc { // the usually-not-taken "slow path" via tsc
		cmd_tsc := exec.Command("tsc")
		cmd_tsc.Dir = inDirPath
		if output, err := cmd_tsc.CombinedOutput(); err != nil {
			panic(err.Error() + "\n" + string(output))
		}
		return
	}

	// the much-faster "esbuild Transform API" way
	FsDirWalk(inDirPath, func(path string, fsEntry fs.DirEntry) {
		if (fsEntry.IsDir()) || str.Ends(path, ".d.ts") || (!str.Ends(path, ".ts")) ||
			(isAppSide && !str.Ends(path, "/"+filepath.Join(StaticFilesDirNameYo, yoSdkTsFileName))) {
			return // nothing to do for this `path`
		}
		TsFile2JsFileViaEsbuild(path)
	})
}
