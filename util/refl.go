package util

import (
	"reflect"
	"unsafe"

	"yo/util/str"
)

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

func ReflGet[T any](rv reflect.Value) T {
	return *getPtr[T](rv.UnsafeAddr())
}

func ReflSet[T any](rv reflect.Value, to T) {
	setPtr(rv.UnsafeAddr(), to)
}

func ReflWalk(rv reflect.Value, path []any, skipMaps bool, onValue func(path []any, curVal reflect.Value)) {
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
		ReflWalk(rv.Elem(), append(path, "*"), skipMaps, onValue)
	case reflect.Slice, reflect.Array:
		for l, i := rv.Len(), 0; i < l; i++ {
			ReflWalk(rv.Index(i), append(path, i), skipMaps, onValue)
		}
	case reflect.Map:
		if !skipMaps {
			for map_range := rv.MapRange(); map_range.Next(); {
				ReflWalk(map_range.Value(), append(path, map_range.Key().Interface()), skipMaps, onValue)
			}
		}
	case reflect.Struct:
		for l, i := rv.NumField(), 0; i < l; i++ {
			ReflWalk(rv.Field(i), append(path, rv.Type().Field(i).Name), skipMaps, onValue)
		}
	default:
		panic("unhandled reflect.Kind at " + str.From(path) + ": " + rv_kind.String())
	}
}

func ReflGt(lhs reflect.Value, rhs reflect.Value) bool {
	return (lhs.Kind() == rhs.Kind()) && !ReflLe(lhs, rhs)
}

func ReflGe(lhs reflect.Value, rhs reflect.Value) bool {
	return (lhs.Kind() == rhs.Kind()) && (ReflGt(lhs, rhs) || reflect.DeepEqual(lhs.Interface(), rhs.Interface()))
}

func ReflLe(lhs reflect.Value, rhs reflect.Value) bool {
	return (lhs.Kind() == rhs.Kind()) && (ReflLt(lhs, rhs) || reflect.DeepEqual(lhs.Interface(), rhs.Interface()))
}

func ReflLt(lhs reflect.Value, rhs reflect.Value) bool {
	if lhs.Kind() != rhs.Kind() {
		return false
	}
	lv, rv := lhs.Interface(), rhs.Interface()
	switch lv := lv.(type) {
	case uint8:
		return lv < rv.(uint8)
	case uint16:
		return lv < rv.(uint16)
	case uint32:
		return lv < rv.(uint32)
	case uint64:
		return lv < rv.(uint64)
	case uint:
		return lv < rv.(uint)
	case int8:
		return lv < rv.(int8)
	case int16:
		return lv < rv.(int16)
	case int32:
		return lv < rv.(int32)
	case int64:
		return lv < rv.(int64)
	case int:
		return lv < rv.(int)
	case float32:
		return lv < rv.(float32)
	case float64:
		return lv < rv.(float64)
	default:
		return lv.(string) < rv.(string)
	}
}
