package api

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"sync"

	"yo/log"
	"yo/str"
	. "yo/util"
)

const SdkGenDstTsFilePath = StaticFileDirPath + "/yo-sdk.ts"

var foundModifiedTsFiles bool

func genSdk() {
	buf, api := strings.Builder{}, refl{}
	log.Println("\treflect...")
	handleReflReq(nil, nil, &api)
	log.Println("\tgenerate...")
	b, err := StaticFileDir.ReadFile(StaticFileDirPath + "/sdkgen.ts")
	if err != nil {
		panic(err)
	}
	_, _ = buf.Write(b)
	for _, enum_name := range Sorted(Keys(api.Enums)) {
		genSdkType(&buf, &api, enum_name, nil, api.Enums[enum_name])
	}
	for _, struct_name := range Sorted(Keys(api.Types)) {
		genSdkType(&buf, &api, struct_name, api.Types[struct_name], nil)
	}
	for _, method := range api.Methods {
		genSdkMethod(&buf, &api, &method)
	}
	src_is_changed, src_to_write := true, []byte(buf.String())
	data, _ := os.ReadFile(SdkGenDstTsFilePath)
	src_is_changed = (len(data) == 0) || (!bytes.Equal(data, src_to_write))
	if src_is_changed {
		foundModifiedTsFiles = true
		log.Println("\twriting files...")
		if err := os.WriteFile("tsconfig.json", []byte(`{"extends": "../yo/tsconfig.json"}`), os.ModePerm); err != nil {
			panic(err)
		}
		if err := os.WriteFile(SdkGenDstTsFilePath, src_to_write, os.ModePerm); err != nil {
			panic(err)
		}
	}
	if foundModifiedTsFiles {
		log.Println("\t2x tsc...")
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

func genSdkType(buf *strings.Builder, api *refl, typeName string, structFields map[string]string, enumMembers []string) {
	switch typeName {
	case "time.Time":
		_, _ = buf.WriteString(str.Fmt("\nexport type %s = %s", genSdkTypeName(api, typeName), genSdkTypeName(api, ".string")))
		return
	}
	if structFields != nil {
		_, _ = buf.WriteString(str.Fmt("\nexport type %s = {", genSdkTypeName(api, typeName)))
		for _, field_name := range Sorted(Keys(structFields)) {
			field_type := structFields[field_name]
			_, _ = buf.WriteString(str.Fmt("\n\t%s: %s", ToIdent(field_name), genSdkTypeName(api, field_type)))
		}
		_, _ = buf.WriteString("\n}\n")
	} else {
		_, _ = buf.WriteString(str.Fmt("\nexport type %s = %s\n", genSdkTypeName(api, typeName),
			If(len(enumMembers) == 0, "string", "\""+str.Join(enumMembers, "\" | \"")+"\"")))
	}
}

func genSdkTypeName(api *refl, typeName string) string {
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
		return genSdkTypeName(api, typeName[1:len(typeName)-1]) + "[]"
	}
	if str.Begins(typeName, "{") && str.Ends(typeName, "}") {
		key_part, val_part, ok := str.Cut(typeName[1:len(typeName)-1], ":")
		if !ok {
			panic(typeName)
		}
		if _, is_enum := api.Enums[key_part]; is_enum {
			return str.Fmt("{ [key in %s]?: %s }", genSdkTypeName(api, key_part), genSdkTypeName(api, val_part))
		}
		return str.Fmt("{ [_:%s]: %s }", genSdkTypeName(api, key_part), genSdkTypeName(api, val_part))
	}
	return "Yo_" + ToIdent(typeName[str.Idx(typeName, '.')+1:])
}

func genSdkMethod(buf *strings.Builder, api *refl, method *reflMethod) {
	_, _ = buf.WriteString(str.Fmt(`
export function yoReq_%s(payload: %s, onSuccess: (_: %s) => void, onFailed?: (err: any, resp?: Response, query?: {[_:string]:string}) => void): void {
	yoReq(%s, payload, onSuccess, onFailed)
}`, ToIdent(method.Path), genSdkTypeName(api, method.In), genSdkTypeName(api, method.Out), str.Q(method.Path)))
}
