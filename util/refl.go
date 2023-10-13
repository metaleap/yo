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
	addr := rv.UnsafeAddr()
	return *getPtr[T](addr)
}

func ReflSet[T any](rv reflect.Value, to T) {
	setPtr(rv.UnsafeAddr(), to)
}

func ReflWalk(rv reflect.Value, path []any, onValue func([]any, reflect.Value)) {
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
		ReflWalk(rv.Elem(), append(path, "*"), onValue)
	case reflect.Slice, reflect.Array:
		for l, i := rv.Len(), 0; i < l; i++ {
			ReflWalk(rv.Index(i), append(path, i), onValue)
		}
	case reflect.Map:
		for map_range := rv.MapRange(); map_range.Next(); {
			ReflWalk(map_range.Value(), append(path, map_range.Key().Interface()), onValue)
		}
	case reflect.Struct:
		for l, i := rv.NumField(), 0; i < l; i++ {
			ReflWalk(rv.Field(i), append(path, rv.Type().Field(i).Name), onValue)
		}
	default:
		panic("unhandled reflect.Kind at " + str.From(path) + ": " + rv_kind.String())
	}
}
