package yo

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

const ApiSdkGenDstTsFilePath = staticFileDirPath + "/yo-sdk.ts"

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
	for enum_name, enum_members := range api.Enums {
		apiGenSdkType(&buf, &api, enum_name, nil, enum_members)
	}
	for struct_name, struct_fields := range api.Types {
		apiGenSdkType(&buf, &api, struct_name, struct_fields, nil)
	}
	for _, method := range api.Methods {
		apiGenSdkMethod(&buf, &api, &method)
	}
	log.Println("\twriting files...")
	if err := os.WriteFile("tsconfig.json", []byte(`{"extends": "../yo/tsconfig.json"}`), os.ModePerm); err != nil {
		panic(err)
	}
	if err := os.WriteFile(ApiSdkGenDstTsFilePath, []byte(buf.String()), os.ModePerm); err != nil {
		panic(err)
	}
	for _, dir_path := range []string{"", "../yo"} {
		log.Println("\ttsc...")
		tsc := exec.Command("tsc")
		tsc.Dir = dir_path
		if output, err := tsc.CombinedOutput(); err != nil {
			panic(err.Error() + "\n" + string(output))
		}
	}
}

func apiGenSdkType(buf *strings.Builder, api *apiReflect, typeName string, structFields map[string]string, enumMembers []string) {
	switch typeName {
	case "time.Time":
		_, _ = buf.WriteString(strFmt("\nexport type %s = %s", apiGenSdkTypeName(typeName), apiGenSdkTypeName(".string")))
		return
	case "time.Duration":
		panic(typeName)
	}
	if structFields != nil {
		_, _ = buf.WriteString(strFmt("\nexport type %s = {", apiGenSdkTypeName(typeName)))
		for field_name, field_type := range structFields {
			_, _ = buf.WriteString(strFmt("\n\t%s: %s", toIdent(field_name), apiGenSdkTypeName(field_type)))
		}
		_, _ = buf.WriteString("\n}\n")
	} else {
		_, _ = buf.WriteString(strFmt("\nexport type %s = %s\n", apiGenSdkTypeName(typeName),
			If(len(enumMembers) == 0, "string", "\""+strings.Join(enumMembers, "\" | \"")+"\"")))
	}
}

func apiGenSdkTypeName(typeRef string) string {
	if strBegins(typeRef, ".") {
		switch t := typeRef[1:]; t {
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
			panic(t)
		}
	}
	if strBegins(typeRef, "[") && strEnds(typeRef, "]") {
		return apiGenSdkTypeName(typeRef[1:len(typeRef)-1]) + "[]"
	}
	if strBegins(typeRef, "{") && strEnds(typeRef, "}") {
		key_part, val_part, ok := strings.Cut(typeRef[1:len(typeRef)-1], ":")
		if !ok {
			panic(typeRef)
		}
		return strFmt("{ [_:%s]: %s }", apiGenSdkTypeName(key_part), apiGenSdkTypeName(val_part))
	}
	return "Yo_" + toIdent(typeRef[strIdx(typeRef, '.')+1:])
}

func apiGenSdkMethod(buf *strings.Builder, api *apiReflect, method *apiReflectMethod) {
	_, _ = buf.WriteString(strFmt(`
export function yoReq_%s(payload: %s, onSuccess: (_:%s) => void): void {
	yoReq(%s, payload, onSuccess)
}`, toIdent(method.Path), apiGenSdkTypeName(method.In), apiGenSdkTypeName(method.Out), strQ(method.Path)))
}
