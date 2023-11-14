//go:build debug

package yopenapi

import (
	"math"
	"reflect"
	"time"

	. "yo/util"
)

// yoValiOnly yoFail

const Version = "3.1.0"

type OpenApi struct {
	OpenApi string          `json:"openapi"`
	Info    Info            `json:"info"`
	Paths   map[string]Path `json:"paths"`
}

type Info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
	Contact struct {
		Name string `json:"name"`
		Url  string `json:"url"`
	} `json:"contact"`
}

type Path struct {
	Post Op `json:"post"`
}

type Op struct {
	Id         string          `json:"operationId"`
	Summary    string          `json:"summary,omitempty"`
	Descr      string          `json:"description,omitempty"`
	Deprecated bool            `json:"deprecated,omitempty"`
	Params     []Param         `json:"parameters"`
	ReqBody    ReqBody         `json:"requestBody"`
	Responses  map[string]Resp `json:"responses"`
}

type Param struct {
	Name       string           `json:"name"`
	In         string           `json:"in"` // query|header|cookie
	Descr      string           `json:"description,omitempty"`
	Required   bool             `json:"required,omitempty"`
	Deprecated bool             `json:"deprecated,omitempty"`
	Content    map[string]Media `json:"content"`
}

type ReqBody struct {
	Descr    string           `json:"description,omitempty"`
	Required bool             `json:"required,omitempty"`
	Content  map[string]Media `json:"content"`
}

type Resp struct {
	Descr   string            `json:"description"`
	Headers map[string]Header `json:"headers"`
	Content map[string]Media  `json:"content"`
}

type Header struct {
	Descr      string           `json:"description,omitempty"`
	Content    map[string]Media `json:"content"`
	Required   bool             `json:"required,omitempty"`
	Deprecated bool             `json:"deprecated,omitempty"`
}

type Media struct {
	Example any `json:"example"`
}

var tyTime = reflect.TypeOf(time.Time{})

func dummyOf(ty reflect.Type, level int, typesDone map[reflect.Type]bool) reflect.Value {
	if ty.Kind() == reflect.Pointer {
		return dummyOf(ty.Elem(), level, typesDone)
	}
	if ty.ConvertibleTo(tyTime) || tyTime.ConvertibleTo(ty) || ty.AssignableTo(tyTime) || tyTime.AssignableTo(ty) {
		return reflect.ValueOf(time.Now()).Convert(ty)
	}
	dummy := reflect.New(ty).Elem()
	Assert(dummy.IsValid(), func() any { return ty.String() })
	switch ty.Kind() {
	case reflect.Bool:
		dummy.SetBool(true)
	case reflect.Int8:
		dummy.SetInt(math.MinInt8)
	case reflect.Int16:
		dummy.SetInt(math.MinInt16)
	case reflect.Int32:
		dummy.SetInt(math.MinInt32)
	case reflect.Int64:
		dummy.SetInt(-9007199254740991)
	case reflect.Int:
		dummy.SetInt(-9007199254740991)
	case reflect.Uint8:
		dummy.SetUint(math.MaxUint8)
	case reflect.Uint16:
		dummy.SetUint(math.MaxUint16)
	case reflect.Uint32:
		dummy.SetUint(math.MaxUint32)
	case reflect.Uint64:
		dummy.SetUint(9007199254740991)
	case reflect.Uint:
		dummy.SetUint(9007199254740991)
	case reflect.String:
		dummy.SetString("someStr")
	// case reflect.Slice:
	// 	if ty.Elem().Kind() == reflect.Pointer {
	// 		return dummyOf(ty.Elem().Elem(), level, typesDone)
	// 	}
	// 	dummy = reflect.MakeSlice(ty, 1, 1)
	// 	append_item := dummyOf(ty.Elem(), level+1, typesDone)
	// 	reflect.Append(dummy, *append_item)
	// case reflect.Map:
	// if ty.Elem().Kind() == reflect.Pointer {
	// 	return dummyOf(ty.Elem().Elem(), level, typesDone)
	// }
	// dummy = reflect.MakeMap(ty)
	// dummy.SetMapIndex(reflect.ValueOf("someKey"), dummyOf(ty.Elem(), level+1, typesDone))
	case reflect.Struct:
		for i := 0; i < ty.NumField(); i++ {
			field := ty.Field(i)
			if !field.IsExported() {
				continue
			}
			field_dummy := dummyOf(field.Type, level+1, typesDone)
			if field.Type.Kind() == reflect.Pointer {
				alloc := reflect.New(field.Type.Elem())
				alloc.Elem().Set(field_dummy)
				field_dummy = alloc
			}
			if !field_dummy.IsZero() {
				dummy.Field(i).Set(field_dummy)
			}
		}
	}

	return dummy
}

func DummyOf(ty reflect.Type) any {
	return dummyOf(ty, 0, map[reflect.Type]bool{}).Interface()
}
