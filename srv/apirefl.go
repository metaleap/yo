package yosrv

import (
	"cmp"
	"reflect"
	"slices"

	. "yo/util"
	"yo/util/kv"
	"yo/util/sl"
	"yo/util/str"
)

var apiReflAllEnums = map[string][]string{}
var apiReflAllDbStructs []reflect.Type
var apiReflYoDbStructs []reflect.Type
var apiReflAppDbStructs []reflect.Type

func init() {
	Apis(ApiMethods{
		yoAdminApisUrlPrefix + "refl": Api(apiHandleReflReq),
	})
}

type apiReflect struct {
	Methods       []apiReflMethod
	Types         map[string]str.Dict
	Enums         map[string][]string
	DbStructs     []string
	KnownErrs     map[string]map[Err]int
	allInputTypes map[string]bool

	codeGen struct {
		typesUsed    map[string]bool
		typesEmitted map[string]bool
	}
}

func (me *apiReflect) method(methodPath string) *apiReflMethod {
	return &me.Methods[sl.IdxWhere(me.Methods, func(it apiReflMethod) bool { return (it.Path == methodPath) })]
}

type apiReflMethod struct {
	Path string
	In   string
	Out  string
}

func (me *apiReflMethod) ident() string    { return ToIdent(me.Path) }
func (me *apiReflMethod) identUp0() string { return str.Up0(me.ident()) }

func apiHandleReflReq(this *ApiCtx[None, apiReflect]) {
	is_at_codegen_time := IsDevMode && (this.Ctx == nil) && (this.Args == nil)
	this.Ret.Types, this.Ret.Enums, this.Ret.KnownErrs, this.Ret.allInputTypes = map[string]str.Dict{}, map[string][]string{}, map[string]map[Err]int{}, map[string]bool{}
	for _, method_path := range sl.Sorted(kv.Keys(api)) {
		if !str.IsPrtAscii(method_path) {
			panic("not printable ASCII: '" + method_path + "'")
		}
		method := apiReflMethod{Path: method_path}
		method_name := method.ident()
		rt_in, rt_out := api[method_path].reflTypes()
		method.In, method.Out = apiReflType(this.Ret, rt_in, "In", method_name), apiReflType(this.Ret, rt_out, "Out", method_name)
		if no_in, no_out := (method.In == ""), (method.Out == ""); no_in || no_out {
			panic(method_path + ": invalid " + If(no_in, "In", "Out"))
		}
		this.Ret.Methods = append(this.Ret.Methods, method)
		this.Ret.KnownErrs[method.Path] = apiReflErrs(api[method_path], method, is_at_codegen_time)
	}
	slices.SortFunc(this.Ret.Methods, func(a apiReflMethod, b apiReflMethod) int {
		if str.Begins(a.Path, "__") != str.Begins(b.Path, "__") { // bring those `__/` internal APIs to the end of the this.Ret.Methods
			return cmp.Compare(b.Path, a.Path)
		}
		return cmp.Compare(a.Path, b.Path)
	})

	var mark_as_input func(string)
	mark_as_input = func(typeName string) {
		this.Ret.allInputTypes[typeName] = true
		for _, field_type_name := range this.Ret.Types[typeName] {
			mark_as_input(field_type_name)
		}
	}
	for _, method := range this.Ret.Methods {
		mark_as_input(method.In)
	}
}

func apiReflErrs(method ApiMethod, _ apiReflMethod, isForCodegenGo bool) (ret map[Err]int) {
	ret = map[Err]int{}
	for _, err := range method.KnownErrs(isForCodegenGo) {
		ret[err] = err.HttpStatusCodeOr(500)
	}
	return
}

func apiReflType(it *apiReflect, rt reflect.Type, fldName string, parent string) string {
	if maybe_db_ref, _ := reflect.New(rt).Interface().(interface{ IsDbRef() bool }); (maybe_db_ref != nil) && maybe_db_ref.IsDbRef() {
		return apiReflType(it, reflect.TypeOf(int64(0)), fldName, parent)
	}
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
		return "?" + apiReflType(it, rt.Elem(), fldName, parent)
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
		if enum_type_name := apiReflEnum(it, type_ident); enum_type_name != "" {
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
		return ".any"
	case reflect.Chan, reflect.Complex128, reflect.Complex64, reflect.Func, reflect.Invalid, reflect.Uintptr, reflect.UnsafePointer:
		return ""
	}
	if _, exists := it.Types[type_ident]; !exists {
		ty_refl := str.Dict{}
		it.Types[type_ident] = ty_refl
		if (rt_kind == reflect.Struct) && !(ReflHasMethod(rt, "MarshalJSON") && ReflHasMethod(rt, "UnmarshalJSON")) {
			if sl.Has(apiReflAllDbStructs, rt) && !str.In(type_ident, it.DbStructs...) {
				it.DbStructs = append(it.DbStructs, type_ident)
			}
			var do_field func(field reflect.StructField)
			do_field = func(field reflect.StructField) {
				if !str.IsPrtAscii(field.Name) {
					fail("not printable ASCII: '" + field.Name + "'")
				}
				if field.Anonymous {
					for sub_field_name := range it.Types[apiReflType(it, field.Type, field.Name, type_ident)] {
						sub_field, _ := field.Type.FieldByName(sub_field_name)
						do_field(sub_field)
					}
				} else if str.IsUp(str.Sub(field.Name, 0, 1)) {
					if ty_field := apiReflType(it, field.Type, field.Name, type_ident); ty_field != "" {
						ty_refl[field.Name] = ty_field
					}
				}
			}
			for i := 0; i < rt.NumField(); i++ {
				do_field(rt.Field(i))
			}
		}
	}
	return type_ident
}

func apiReflEnum(it *apiReflect, typeIdent string) string {
	if !str.IsPrtAscii(typeIdent) {
		panic("not printable ASCII: '" + typeIdent + "'")
	}
	if IsDevMode {
		if found, exists := apiReflAllEnums[typeIdent]; exists {
			it.Enums[typeIdent] = found
			return typeIdent
		} else if found, exists := apiReflAllEnums[typeIdent[str.IdxLast(typeIdent, '.')+1:]]; exists {
			it.Enums[typeIdent] = found
			return typeIdent
		}
	}
	return ""
}
