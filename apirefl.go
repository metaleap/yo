package yo

import (
	"cmp"
	"reflect"
	"slices"
)

type apiReflect struct {
	Methods []apiReflectMethod
	Types   map[string]map[string]string
}

type apiReflectMethod struct {
	Path string
	In   string
	Out  string
}

func apiHandleRefl(_ *Ctx, _ *Void, ret *apiReflect) error {
	ret.Types = map[string]map[string]string{}
	for methodPath, f := range API {
		m := apiReflectMethod{Path: methodPath}
		ty_in, ty_out := f.reflTypes()
		m.In, m.Out = apiReflType(ret, ty_in, "", ""), apiReflType(ret, ty_out, "", "")
		ret.Methods = append(ret.Methods, m)
	}
	slices.SortFunc(ret.Methods, func(a apiReflectMethod, b apiReflectMethod) int {
		return cmp.Compare(a.Path, b.Path)
	})
	return nil
}

func apiReflType(ctx *apiReflect, ty reflect.Type, fldName string, parent string) string {
	if ty.Kind() == reflect.Pointer {
		return apiReflType(ctx, ty.Elem(), fldName, parent)
	}
	if ty.Kind() == reflect.Slice || ty.Kind() == reflect.Array {
		return "[" + apiReflType(ctx, ty.Elem(), fldName, parent) + "]"
	}
	if ty.Kind() == reflect.Map {
		return "{" + apiReflType(ctx, ty.Key(), fldName, parent) + ":" + apiReflType(ctx, ty.Elem(), fldName, parent) + "}"
	}
	type_ident := ty.PkgPath() + "." + ty.Name()
	if type_ident == "." && ty.Kind() == reflect.Struct && parent != "" && fldName != "" {
		type_ident = parent + "_" + fldName
	}
	if strBegins(type_ident, ".") {
		return type_ident
	}
	if _, exists := ctx.Types[type_ident]; !exists {
		ty_refl := map[string]string{}
		ctx.Types[type_ident] = ty_refl
		if ty.Kind() == reflect.Struct {
			if !(reflHasMethod(ty, "MarshalJSON") && reflHasMethod(ty, "UnmarshalJSON")) {
				for i := 0; i < ty.NumField(); i++ {
					fld := ty.Field(i)
					ty_refl[fld.Name] = apiReflType(ctx, fld.Type, fld.Name, type_ident)
				}
			}
		}
	}
	return type_ident
}
