package yo

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const ApiSdkGenDstTsFilePath = staticFileDirPath + "/yo-sdk.ts"

var foundModifiedTsFiles bool

func apiGenSdk() {
	buf, api := strings.Builder{}, apiReflect{}
	log.Println("\treflect...")
	if err := apiHandleRefl(nil, nil, &api); err != nil {
		panic(err)
	}
	log.Println("\tgenerate...")
	b, err := staticFileDir.ReadFile(staticFileDirPath + "/sdkgen.ts")
	if err != nil {
		panic(err)
	}
	_, _ = buf.Write(b)
	for _, enum_name := range sorted(keys(api.Enums)) {
		apiGenSdkType(&buf, &api, enum_name, nil, api.Enums[enum_name])
	}
	for _, struct_name := range sorted(keys(api.Types)) {
		apiGenSdkType(&buf, &api, struct_name, api.Types[struct_name], nil)
	}
	for _, method := range api.Methods {
		apiGenSdkMethod(&buf, &api, &method)
	}
	src_is_changed, src_to_write := true, []byte(buf.String())
	data, _ := os.ReadFile(ApiSdkGenDstTsFilePath)
	src_is_changed = (len(data) == 0) || (!bytes.Equal(data, src_to_write))
	if src_is_changed {
		foundModifiedTsFiles = true
		log.Println("\twriting files...")
		if err := os.WriteFile("tsconfig.json", []byte(`{"extends": "../yo/tsconfig.json"}`), os.ModePerm); err != nil {
			panic(err)
		}
		if err := os.WriteFile(ApiSdkGenDstTsFilePath, src_to_write, os.ModePerm); err != nil {
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

func apiGenSdkType(buf *strings.Builder, api *apiReflect, typeName string, structFields map[string]string, enumMembers []string) {
	switch typeName {
	case "time.Time":
		_, _ = buf.WriteString(strFmt("\nexport type %s = %s", apiGenSdkTypeName(api, typeName), apiGenSdkTypeName(api, ".string")))
		return
	}
	if structFields != nil {
		_, _ = buf.WriteString(strFmt("\nexport type %s = {", apiGenSdkTypeName(api, typeName)))
		for _, field_name := range sorted(keys(structFields)) {
			field_type := structFields[field_name]
			_, _ = buf.WriteString(strFmt("\n\t%s: %s", toIdent(field_name), apiGenSdkTypeName(api, field_type)))
		}
		_, _ = buf.WriteString("\n}\n")
	} else {
		_, _ = buf.WriteString(strFmt("\nexport type %s = %s\n", apiGenSdkTypeName(api, typeName),
			If(len(enumMembers) == 0, "string", "\""+strJoin(enumMembers, "\" | \"")+"\"")))
	}
}

func apiGenSdkTypeName(api *apiReflect, typeName string) string {
	if strBegins(typeName, ".") {
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
	if strBegins(typeName, "[") && strEnds(typeName, "]") {
		return apiGenSdkTypeName(api, typeName[1:len(typeName)-1]) + "[]"
	}
	if strBegins(typeName, "{") && strEnds(typeName, "}") {
		key_part, val_part, ok := strCut(typeName[1:len(typeName)-1], ":")
		if !ok {
			panic(typeName)
		}
		if _, is_enum := api.Enums[key_part]; is_enum {
			return strFmt("{ [key in %s]?: %s }", apiGenSdkTypeName(api, key_part), apiGenSdkTypeName(api, val_part))
		}
		return strFmt("{ [_:%s]: %s }", apiGenSdkTypeName(api, key_part), apiGenSdkTypeName(api, val_part))
	}
	return "Yo_" + toIdent(typeName[strIdx(typeName, '.')+1:])
}

func apiGenSdkMethod(buf *strings.Builder, api *apiReflect, method *apiReflectMethod) {
	_, _ = buf.WriteString(strFmt(`
export function yoReq_%s(payload: %s, onSuccess: (_: %s) => void, onFailed?: (err: any, resp?: Response, query?: {[_:string]:string}) => void): void {
	yoReq(%s, payload, onSuccess, onFailed)
}`, toIdent(method.Path), apiGenSdkTypeName(api, method.In), apiGenSdkTypeName(api, method.Out), strQ(method.Path)))
}
