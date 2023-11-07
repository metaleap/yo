package util

import (
	"cmp"
	"reflect"
	"unsafe"

	"yo/util/str"
)

func ReflType[T any]() reflect.Type {
	var none T
	return reflect.TypeOf(none)
}

func getPtr[T any](at uintptr) *T {
	return (*T)((unsafe.Pointer)(at))
}

func setPtr[T any](at uintptr, value T) {
	it := getPtr[T](at)
	*it = value
}

func ReflHasMethod(ty reflect.Type, name string) bool {
	for ty.Kind() == reflect.Pointer {
		ty = ty.Elem()
	}
	_, ok := ty.MethodByName(name)
	if !ok {
		_, ok = reflect.PointerTo(ty).MethodByName(name)
	}
	return ok
}

func ReflField(obj any, fieldName string) *reflect.Value {
	rv := reflect.ValueOf(obj)
	for rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rt := rv.Type(); rt.Kind() == reflect.Struct {
		embeds := map[*reflect.StructField]reflect.Value{}
		do_fields := func(rt reflect.Type, rv reflect.Value) *reflect.Value {
			for i, l := 0, rt.NumField(); i < l; i++ {
				if field := rt.Field(i); field.Name == fieldName {
					return ToPtr(rv.Field(i))
				} else if field.Anonymous {
					embeds[&field] = rv.Field(i)
				}
			}
			return nil
		}
		if ret := do_fields(rt, rv); ret != nil {
			return ret
		}
		for len(embeds) > 0 {
			for embed_field, embed_value := range embeds {
				delete(embeds, embed_field)
				if ret := do_fields(embed_field.Type, embed_value); ret != nil {
					return ret
				}
			}
		}
	}
	panic("no field '" + fieldName + "' in type '" + rv.Type().String() + "'")
}

func ReflGet[T any](rv reflect.Value) T {
	return *getPtr[T](rv.UnsafeAddr())
}

func ReflSet[T any](dst reflect.Value, to T) {
	setPtr(dst.UnsafeAddr(), to)
}

func ReflWalk(rv reflect.Value, path []any, skipArrTraversals bool, skipMapTraversals bool, onValue func(path []any, curVal reflect.Value)) {
	rv_kind := rv.Kind()
	if (rv_kind == reflect.Invalid) || !rv.IsValid() {
		return
	}

	switch rv_kind {
	case reflect.String, reflect.Bool,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.Int, reflect.Uint, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		onValue(path, rv)
	case reflect.Pointer, reflect.Interface:
		ReflWalk(rv.Elem(), append(path, "*"), skipArrTraversals, skipMapTraversals, onValue)
	case reflect.Slice, reflect.Array:
		if skipArrTraversals {
			onValue(path, rv)
		} else {
			for l, i := rv.Len(), 0; i < l; i++ {
				ReflWalk(rv.Index(i), append(path, i), skipArrTraversals, skipMapTraversals, onValue)
			}
		}
	case reflect.Map:
		if skipMapTraversals {
			onValue(path, rv)
		} else {
			for map_range := rv.MapRange(); map_range.Next(); {
				ReflWalk(map_range.Value(), append(path, map_range.Key().Interface()), skipArrTraversals, skipMapTraversals, onValue)
			}
		}
	case reflect.Struct:
		for l, i := rv.NumField(), 0; i < l; i++ {
			ReflWalk(rv.Field(i), append(path, rv.Type().Field(i).Name), skipArrTraversals, skipMapTraversals, onValue)
		}
	default:
		panic("unhandled reflect.Kind at " + str.GoLike(path) + ": " + rv_kind.String())
	}
}

func ReflGt(lhs reflect.Value, rhs reflect.Value) bool { return reflCmp(lhs, rhs, false, false) }
func ReflGe(lhs reflect.Value, rhs reflect.Value) bool { return reflCmp(lhs, rhs, false, true) }
func ReflLe(lhs reflect.Value, rhs reflect.Value) bool { return reflCmp(lhs, rhs, true, true) }
func ReflLt(lhs reflect.Value, rhs reflect.Value) bool { return reflCmp(lhs, rhs, true, false) }

func reflCmp(lhs reflect.Value, rhs reflect.Value, less bool, orEq bool) bool {
	switch {
	case lhs.CanFloat() && rhs.CanFloat():
		return cmpHow(lhs.Float(), rhs.Float(), less, orEq)
	case lhs.CanUint() && rhs.CanUint():
		return cmpHow(lhs.Uint(), rhs.Uint(), less, orEq)
	case lhs.CanInt() && rhs.CanInt():
		return cmpHow(lhs.Int(), rhs.Int(), less, orEq)
	case lhs.CanConvert(str.ReflType) && rhs.CanConvert(str.ReflType):
		return cmpHow(lhs.String(), rhs.String(), less, orEq)
	}
	return false
}

func cmpHow[T cmp.Ordered](x T, y T, less bool, orEq bool) bool {
	if less && orEq {
		return x <= y
	} else if less {
		return x < y
	} else if orEq {
		return x >= y
	}
	return x > y
}
