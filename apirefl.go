package yo

import (
	"reflect"
)

type apiReflect struct {
	Types   map[string]map[string]string
	Methods []apiReflectMethod
}

type apiReflectMethod struct {
	Path string
	In   string
	Out  string
}

func apiHandleRefl(it *Ctx, _ *T0, ret *apiReflect) error {
	ret.Types = map[string]map[string]string{}
	for methodPath, f := range API {
		ret.Methods = append(ret.Methods, apiReflectMethod{Path: methodPath})
		ty_in, ty_out := f.reflTypes()
		apiReflType(ret, ty_in)
		apiReflType(ret, ty_out)
	}
	return nil
}

func apiReflType(ctx *apiReflect, ty reflect.Type) string {
	type_ident := ty.PkgPath() + "." + ty.Name()
	if _, exists := ctx.Types[type_ident]; !exists {
		ty_refl := map[string]string{}
		if ctx.Types[type_ident] = ty_refl; !strBegins(type_ident, ".") {
			for i := 0; i < ty.NumField(); i++ {
				fld := ty.Field(i)
				ty_refl[fld.Name] = apiReflType(ctx, fld.Type)
			}
		}
		// ctx.Types[type_ident] = ty_refl
	}
	return type_ident
}
