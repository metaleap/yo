package yo

import (
	"os"
	"os/exec"
	"strings"
)

const ApiSdkGenDstTsFilePath = staticFileDirPath + "/yo-sdk.ts"

func apiGenSdk() {
	buf, api := strings.Builder{}, apiReflect{}
	if err := apiHandleRefl(nil, nil, &api); err != nil {
		panic(err)
	}
	b, err := staticFileDir.ReadFile(staticFileDirPath + "/sdkgen.ts")
	if err != nil {
		panic(err)
	}
	_, _ = buf.Write(b)
	for type_name, type_fields := range api.Types {
		apiGenSdkType(&buf, &api, type_name, type_fields)
	}
	for _, method := range api.Methods {
		apiGenSdkMethod(&buf, &api, &method)
	}
	if err := os.WriteFile("tsconfig.json", []byte(`{"extends": "../yo/tsconfig.json"}`), os.ModePerm); err != nil {
		panic(err)
	}
	if err := os.WriteFile(ApiSdkGenDstTsFilePath, []byte(buf.String()), os.ModePerm); err != nil {
		panic(err)
	}
	for _, dir_path := range []string{"", "../yo"} {
		tsc := exec.Command("tsc")
		tsc.Dir = dir_path
		if output, err := tsc.CombinedOutput(); err != nil {
			panic(err.Error() + "\n" + string(output))
		}
	}
}

func apiGenSdkType(buf *strings.Builder, api *apiReflect, typeName string, typeFields map[string]string) {
	switch typeName {
	case "time.Time":
		_, _ = buf.WriteString(strFmt("\ntype %s = %s", apiGenSdkTypeName(typeName), apiGenSdkTypeName(".string")))
		return
	case "time.Duration":
		_, _ = buf.WriteString(strFmt("\ntype %s = %s", apiGenSdkTypeName(typeName), apiGenSdkTypeName(".int64")))
		return
	}
	_, _ = buf.WriteString(strFmt("\ntype %s = {", apiGenSdkTypeName(typeName)))
	for field_name, field_type := range typeFields {
		_, _ = buf.WriteString(strFmt("\n\t%s: %s", toIdent(field_name), apiGenSdkTypeName(field_type)))
	}
	_, _ = buf.WriteString("\n}\n")
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
function yoReq_%s(payload: %s, onSuccess: (_:%s) => void): void {
	yoReq(%s, payload, onSuccess)
}`, toIdent(method.Path), apiGenSdkTypeName(method.In), apiGenSdkTypeName(method.Out), strQ(method.Path)))
}
