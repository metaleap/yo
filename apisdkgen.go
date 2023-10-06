package yo

import (
	_ "embed"
	"os"
	"strings"
)

//go:embed sdkgen.ts
var sdkgen_ts string

func apiGenSdk(tsDstFilePath string) {
	buf, api := strings.Builder{}, apiReflect{}
	if err := apiHandleRefl(nil, nil, &api); err != nil {
		panic(err)
	}
	_, _ = buf.WriteString(sdkgen_ts)
	for type_name, type_fields := range api.Types {
		apiGenSdkType(&buf, &api, type_name, type_fields)
	}
	for _, method := range api.Methods {
		apiGenSdkMethod(&buf, &api, &method)
	}
	if err := os.WriteFile(tsDstFilePath, []byte(buf.String()), os.ModePerm); err != nil {
		panic(err)
	}
}

func apiGenSdkType(buf *strings.Builder, api *apiReflect, typeName string, typeFields map[string]string) {
	_, _ = buf.WriteString(strFmt("\ntype _yo_apiStruct_%s = {", toIdent(typeName)))
	_, _ = buf.WriteString("}\n")
}

func apiGenSdkMethod(buf *strings.Builder, api *apiReflect, method *apiReflectMethod) {
	_, _ = buf.WriteString(strFmt(`
function _yo_apiCall_%s(input: _yo_apiStruct_%s, onSuccess: (_:_yo_apiStruct_%s) => void): void {

}`, toIdent(method.Path), toIdent(method.In), toIdent(method.Out)))
}
