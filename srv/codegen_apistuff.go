//go:build debug

package yosrv

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	yolog "yo/log"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

var (
	foundModifiedTsFiles bool
	pkgsFound            = str.Dict{}
	pkgsImportingSrv     = map[string]bool{}
)

func init() {
	codegenMaybe = func() {
		yolog.Println("API codegen...")
		yolog.Println("  reflect...")
		api_refl := apiRefl{}
		apiHandleReflReq(&ApiCtx[Void, apiRefl]{Args: &Void{}, Ret: &api_refl})
		codegenGo(&api_refl)
		codegenTsSdk(&api_refl)
	}
	enum_pkgs := str.Dict{}
	WalkCodeFiles(true, true, func(path string, dirEntry fs.DirEntry) {
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
					pkgsFound[pkg_name] = filepath.Dir(path)
				} else if str.Begins(line, "\t") && str.Ends(line, `"yo/srv"`) && pkg_name != "" {
					pkgsImportingSrv[pkg_name] = true
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

func codegenGo(apiRefl *apiRefl) {
	did_write_files := false
	for pkg_name, pkg_dir_path := range pkgsFound {
		out_file_path := filepath.Join(pkg_dir_path, "ˍapi_generated_code.go")
		if !pkgsImportingSrv[pkg_name] {
			_ = os.Remove(out_file_path)
			continue
		}

		pkg_methods := map[string]ApiMethod{}
		for method_path, method := range api {
			if method.PkgName() == pkg_name {
				pkg_methods[method_path] = method
			}
		}
		var buf str.Buf
		buf.WriteString("// Code generated by `yo/srv/codegen_apistuff.go` DO NOT EDIT\n")
		buf.WriteString("package " + pkg_name + "\n")
		buf.WriteString("import (util \"yo/util\")\n")
		buf.WriteString("type apiPkgInfo util.Void\n")
		buf.WriteString("func (apiPkgInfo) PkgName() string { return \"" + pkg_name + "\" }\n")
		buf.WriteString("var PkgInfo = apiPkgInfo{}\n")

		for _, method_path := range sl.Sorted(Keys(pkg_methods)) {
			for _, err := range sl.Sorted(Keys(apiRefl.KnownErrs[method_path])) {
				buf.WriteString("const Err" + string(err) + " util.Err = \"" + string(err) + "\"\n")
			}
		}

		src_raw, err := format.Source([]byte(buf.String()))
		if err != nil {
			panic(err)
		}

		src_old, _ := os.ReadFile(out_file_path)
		if !bytes.Equal(src_old, src_raw) {
			_ = os.WriteFile(out_file_path, src_raw, os.ModePerm)
			did_write_files = true
		}
	}
	if did_write_files {
		panic("apicodegen'd, please restart")
	}
}

func codegenTsSdk(apiRefl *apiRefl) {
	const sdkGenDstTsFileRelPath = StaticFilesDirName + "/yo-sdk.ts"
	buf := str.Buf{}
	apiRefl.codeGen.typesUsed, apiRefl.codeGen.typesEmitted = map[string]bool{}, map[string]bool{}

	yolog.Println("  generate *.ts...")
	b, err := os.ReadFile("../yo/" + sdkGenDstTsFileRelPath)
	if err != nil {
		panic(err)
	}
	buf.Write(b)
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

func codegenTsSdkMethod(buf *str.Buf, apiRefl *apiRefl, method *apiReflMethod) {
	if str.Begins(method.Path, "__/") {
		return
	}

	repl := str.Dict{
		"method_name":    ToIdent(method.Path),
		"in_type_ident":  codegenTsSdkTypeName(apiRefl, method.In),
		"out_type_ident": codegenTsSdkTypeName(apiRefl, method.Out),
		"method_path":    method.Path,
	}
	_ = repl

	buf.WriteString(str.Repl(`
export async function req_{method_name}(payload: {in_type_ident}, query?: {[_:string]:string}): Promise<{out_type_ident}> {
	return req<{in_type_ident}, {out_type_ident}>("{method_path}", payload, query)
}`,
		repl))

	if method_errs := apiRefl.KnownErrs[method.Path]; len(method_errs) > 0 {
		method_name := str.Up0(method.Path)
		ts_enum_type_name, ts_err_type_name := method_name+"Err", "Err"+method_name
		buf.WriteString("\nexport type " + ts_enum_type_name)
		for i, err := range sl.Sorted(Keys(method_errs)) {
			buf.WriteString(If(i == 0, " = ", " | ") + "\"" + string(err) + "\"")
		}
		buf.WriteString(`
export class ` + ts_err_type_name + ` extends Error {
	knownErr: ` + ts_enum_type_name + `
	constructor(err: ` + ts_enum_type_name + `) {
		super(err)
		this.knownErr = err
	}
}
`,
		)
		/*
		   export type AuthLoginErr = "Foo" | "Bar" | "Baz"
		   export class ErrAuthLogin extends Error {
		   	err: AuthLoginErr
		   	constructor(err: AuthLoginErr) {
		   		super(err)
		   		this.err = err
		   	}
		   }
		*/
	}
}

func codegenTsSdkType(buf *str.Buf, apiRefl *apiRefl, typeName string, structFields str.Dict, enumMembers []string) bool {
	for str.Begins(typeName, "?") {
		typeName = typeName[1:]
	}
	if apiRefl.codeGen.typesEmitted[typeName] || !apiRefl.codeGen.typesUsed[typeName] {
		return false
	}
	apiRefl.codeGen.typesEmitted[typeName] = true
	if typeName == "time.Time" {
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
			"rhs": If(len(enumMembers) == 0, "string", "\""+str.Join(enumMembers, "\" | \"")+"\""),
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
