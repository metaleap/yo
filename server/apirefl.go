package yoserve

import (
	"reflect"

	. "yo/ctx"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

var apiReflAllEnums = map[string][]string{}
var apiReflAllDbStructs []reflect.Type

type apiRefl struct {
	Methods   []apiReflMethod
	Types     map[string]map[string]string
	Enums     map[string][]string
	DbStructs []string
}

type apiReflMethod struct {
	Path string
	In   string
	Out  string
}

func apiHandleReflReq(_ *Ctx, _ *Void, ret *apiRefl) any {
	ret.Types, ret.Enums = map[string]map[string]string{}, map[string][]string{}
	for _, method_path := range sl.Sorted(Keys(API)) {
		if !str.IsPrtAscii(method_path) {
			panic("not printable ASCII: '" + method_path + "'")
		}
		method_name, method := ToIdent(method_path), apiReflMethod{Path: method_path}
		rt_in, rt_out := API[method_path].reflTypes()
		method.In, method.Out = apiReflType(ret, rt_in, "In", method_name), apiReflType(ret, rt_out, "Out", method_name)
		if no_in, no_out := (method.In == ""), (method.Out == ""); no_in || no_out {
			panic(method_path + ": invalid " + If(no_in, "In", "Out"))
		}
		ret.Methods = append(ret.Methods, method)
	}
	return ret
}

func apiReflType(it *apiRefl, rt reflect.Type, fldName string, parent string) string {
	rt_kind, type_ident := rt.Kind(), rt.PkgPath()+"."+rt.Name()
	if type_ident == "." && rt_kind == reflect.Struct && parent != "" && fldName != "" {
		type_ident = parent + "_" + fldName
	}
	fail := func(msg string) {
		panic(If(type_ident != "" && type_ident != ".", type_ident+" ", "") + If(parent != "", parent+".", "") + fldName + ": " + msg)
	}
	if !str.IsPrtAscii(type_ident) {
		fail("not printable ASCII: '" + type_ident + "'")
	}

	if rt_kind == reflect.Pointer {
		return apiReflType(it, rt.Elem(), fldName, parent)
	}
	if rt_kind == reflect.Slice || rt_kind == reflect.Array {
		ty_elem := apiReflType(it, rt.Elem(), fldName, parent)
		return If((ty_elem == ""), "", ("[" + ty_elem + "]"))
	}
	if rt_kind == reflect.Map {
		rt_key := rt.Key()
		if rt_key.Kind() != reflect.String {
			fail("non-string map key type '" + rt_key.PkgPath() + "." + rt_key.Name() + "' not supported")
		}
		ty_key, ty_val := apiReflType(it, rt_key, fldName, parent), apiReflType(it, rt.Elem(), fldName, parent)
		return If((ty_key == "") || (ty_val == ""), "", ("{" + ty_key + ":" + ty_val + "}"))
	}
	if type_ident == "time.Duration" {
		fail("time.Duration not supported, use numeric unit-communicating field (like timeoutSec or retainHrs)")
	}
	if type_ident != "." && str.Begins(type_ident, ".") {
		return type_ident
	}
	if rt_kind == reflect.String {
		if enum_type_name := apiReflEnum(it, rt, type_ident); enum_type_name != "" {
			return enum_type_name
		}
	}
	switch rt_kind {
	case reflect.Uint, reflect.Int:
		fail("uint or int not supported, use sized int types instead")
	case reflect.Float32, reflect.Float64, reflect.Bool, reflect.String,
		reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8:
		return "." + rt_kind.String()
	case reflect.Interface:
		fail("any not supported")
	case reflect.Chan, reflect.Complex128, reflect.Complex64, reflect.Func, reflect.Invalid, reflect.Uintptr, reflect.UnsafePointer:
		return ""
	}
	if _, exists := it.Types[type_ident]; !exists {
		ty_refl := map[string]string{}
		it.Types[type_ident] = ty_refl
		if (rt_kind == reflect.Struct) && !(ReflHasMethod(rt, "MarshalJSON") && ReflHasMethod(rt, "UnmarshalJSON")) {
			if sl.Has(apiReflAllDbStructs, rt) && !sl.Has(it.DbStructs, type_ident) {
				it.DbStructs = append(it.DbStructs, type_ident)
			}
			for i := 0; i < rt.NumField(); i++ {
				field := rt.Field(i)
				if str.IsUp(str.Sub(field.Name, 0, 1)) {
					if !str.IsPrtAscii(field.Name) {
						fail("not printable ASCII: '" + field.Name + "'")
					}
					if ty_field := apiReflType(it, field.Type, field.Name, type_ident); ty_field != "" {
						ty_refl[field.Name] = ty_field
					}
				}
			}
		}
	}
	return type_ident
}

func apiReflEnum(it *apiRefl, rt reflect.Type, typeIdent string) string {
	if !str.IsPrtAscii(typeIdent) {
		panic("not printable ASCII: '" + typeIdent + "'")
	}
	if IsDevMode {
		if found, exists := apiReflAllEnums[typeIdent]; exists {
			it.Enums[typeIdent] = found
			return typeIdent
		}
	}
	return ""
}
