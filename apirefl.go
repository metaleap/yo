package yo

import (
	"cmp"
	"reflect"
	"slices"
)

type apiReflect struct {
	Methods []apiReflectMethod
	Types   map[string]map[string]string
	Enums   map[string][]string
}

type apiReflectMethod struct {
	Path string
	In   string
	Out  string
}

func apiHandleRefl(_ *Ctx, _ *Void, ret *apiReflect) error {
	ret.Types = map[string]map[string]string{}
	for method_path, f := range API {
		m := apiReflectMethod{Path: method_path}
		rt_in, rt_out := f.reflTypes()
		m.In, m.Out = apiReflType(ret, rt_in, "", ""), apiReflType(ret, rt_out, "", "")
		if no_in, no_out := (m.In == ""), (m.Out == ""); no_in || no_out {
			panic(method_path + ": invalid " + If(no_in, "In", "Out"))
		}
		ret.Methods = append(ret.Methods, m)
	}
	slices.SortFunc(ret.Methods, func(a apiReflectMethod, b apiReflectMethod) int {
		return cmp.Compare(a.Path, b.Path)
	})
	return nil
}

func apiReflType(it *apiReflect, rt reflect.Type, fldName string, parent string) string {
	rt_kind := rt.Kind()
	if rt_kind == reflect.Pointer {
		return apiReflType(it, rt.Elem(), fldName, parent)
	}
	if rt_kind == reflect.Slice || rt_kind == reflect.Array {
		ty_elem := apiReflType(it, rt.Elem(), fldName, parent)
		return If((ty_elem == ""), "", ("[" + ty_elem + "]"))
	}
	if rt_kind == reflect.Map {
		ty_key, ty_val := apiReflType(it, rt.Key(), fldName, parent), apiReflType(it, rt.Elem(), fldName, parent)
		return If((ty_key == "") || (ty_val == ""), "", ("{" + ty_key + ":" + ty_val + "}"))
	}
	type_ident := rt.PkgPath() + "." + rt.Name()
	if type_ident == "." && rt_kind == reflect.Struct && parent != "" && fldName != "" {
		type_ident = parent + "_" + fldName
	}
	if strBegins(type_ident, ".") {
		return type_ident
	}
	switch rt_kind {
	case reflect.Uint, reflect.Int:
		panic(rt.Name() + " " + fldName + ": uint or int detected, use sized int types instead")
	case reflect.Float32, reflect.Float64, reflect.Bool, reflect.String,
		reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8:
		return "." + rt_kind.String()
	case reflect.Chan, reflect.Complex128, reflect.Complex64, reflect.Func, reflect.Invalid, reflect.Uintptr, reflect.UnsafePointer:
		return ""
	}
	if _, exists := it.Types[type_ident]; !exists {
		ty_refl := map[string]string{}
		it.Types[type_ident] = ty_refl
		if rt_kind == reflect.Struct {
			if !(reflHasMethod(rt, "MarshalJSON") && reflHasMethod(rt, "UnmarshalJSON")) {
				for i := 0; i < rt.NumField(); i++ {
					field := rt.Field(i)
					if ty_field := apiReflType(it, field.Type, field.Name, type_ident); ty_field != "" {
						ty_refl[field.Name] = ty_field
					}
				}
			}
		}
	}
	return type_ident
}
