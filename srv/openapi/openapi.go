//go:build debug

package yopenapi

import (
	"math"
	"reflect"
	"time"

	. "yo/cfg"
	. "yo/util"
	"yo/util/str"
)

// yoValiOnly yoFail

const JsMaxNum = 9007199254740991
const Version = "3.1.0"

type OpenApi struct {
	OpenApi    string          `json:"openapi"`
	Info       Info            `json:"info"`
	Paths      map[string]Path `json:"paths"`
	Components struct {
		Schemas map[string]*SchemaModel `json:"schemas"`
	} `json:"components"`
}

type Info struct {
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
	Descr   string `json:"description,omitempty"`
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
	Headers map[string]Header `json:"headers,omitempty"`
	Content map[string]Media  `json:"content"`
}

type Header struct {
	Descr      string           `json:"description,omitempty"`
	Content    map[string]Media `json:"content"`
	Required   bool             `json:"required,omitempty"`
	Deprecated bool             `json:"deprecated,omitempty"`
}

type Media struct {
	Schema   *SchemaModel       `json:"schema,omitempty"`
	Example  any                `json:"example,omitempty"`
	Examples map[string]Example `json:"examples,omitempty"`
}

type Example struct {
	Summary string `json:"summary,omitempty"`
	Descr   string `json:"description,omitempty"`
	Value   any    `json:"value"`
}

type SchemaModel struct {
	ty       reflect.Type
	Descr    string                 `json:"description,omitempty"`
	Type     string                 `json:"type"` // object
	Ref      string                 `json:"$ref,omitempty"`
	Fields   map[string]SchemaField `json:"properties,omitempty"`
	Examples []any                  `json:"examples,omitempty"`
}

type SchemaField struct {
	Type   string                 `json:"type,omitempty"`
	Fields map[string]SchemaField `json:"properties,omitempty"`
	Format string                 `json:"format,omitempty"`
	IMin   *int64                 `json:"minimum,omitempty"`
	IMax   int64                  `json:"maximum,omitempty"`
	FMin   *float64               `json:"exclusiveMinimum,omitempty"`
	FMax   *float64               `json:"exclusiveMaximum,omitempty"`
	SMin   int                    `json:"minLength,omitempty"`
	SMax   int                    `json:"maxLength,omitempty"`
	ArrOf  *SchemaField           `json:"items,omitempty"`
	Ref    string                 `json:"$ref,omitempty"`
	Map    *SchemaField           `json:"additionalProperties,omitempty"`
}

func (me *OpenApi) EnsureSchemaModel(ty reflect.Type) string {
	type_key := ToIdent(ty.String())
	if _, got := me.Components.Schemas[type_key]; !got {
		schema_model := SchemaModel{
			ty:       ty,
			Descr:    ty.String(),
			Type:     "object",
			Examples: []any{DummyOf(ty)},
			Fields:   map[string]SchemaField{},
		}
		me.Components.Schemas[type_key] = &schema_model
		// populate fields only now, after, in case of circular/self-referencing `struct`s
		schema_model.Fields = me.schemaField(ty).Fields
	}
	return type_key
}

func (me *OpenApi) schemaField(ty reflect.Type) SchemaField {
	if ty.Kind() == reflect.Pointer {
		return me.schemaField(ty.Elem())
	}
	if ty.ConvertibleTo(ReflTypeTime) || ReflTypeTime.ConvertibleTo(ty) || ty.AssignableTo(ReflTypeTime) || ReflTypeTime.AssignableTo(ty) {
		return SchemaField{Type: "string", Format: "date-time"}
	}
	switch ty.Kind() {
	case reflect.Bool:
		return SchemaField{Type: "boolean"}
	case reflect.Int8:
		return SchemaField{Type: "integer", Format: "int32", IMin: ToPtr(int64(math.MinInt8)), IMax: math.MaxInt8}
	case reflect.Int16:
		return SchemaField{Type: "integer", Format: "int32", IMin: ToPtr(int64(math.MinInt16)), IMax: math.MaxInt16}
	case reflect.Int32:
		return SchemaField{Type: "integer", Format: "int32", IMin: ToPtr(int64(math.MinInt32)), IMax: math.MaxInt32}
	case reflect.Int64:
		return SchemaField{Type: "integer", Format: "int64", IMin: ToPtr(int64(-JsMaxNum)), IMax: JsMaxNum}
	case reflect.Int:
		return SchemaField{Type: "integer", Format: "int64", IMin: ToPtr(int64(-JsMaxNum)), IMax: JsMaxNum}
	case reflect.Uint8:
		return SchemaField{Type: "integer", Format: "int32", IMin: ToPtr(int64(0)), IMax: math.MaxUint8}
	case reflect.Uint16:
		return SchemaField{Type: "integer", Format: "int32", IMin: ToPtr(int64(0)), IMax: math.MaxUint16}
	case reflect.Uint32:
		return SchemaField{Type: "integer", Format: "int32", IMin: ToPtr(int64(0)), IMax: math.MaxUint32}
	case reflect.Uint64:
		return SchemaField{Type: "integer", Format: "int64", IMin: ToPtr(int64(0)), IMax: JsMaxNum}
	case reflect.Uint:
		return SchemaField{Type: "integer", Format: "int64", IMin: ToPtr(int64(0)), IMax: JsMaxNum}
	case reflect.Float32:
		return SchemaField{Type: "number", Format: "float", FMin: ToPtr(float64(-JsMaxNum)), FMax: ToPtr(float64(JsMaxNum))}
	case reflect.Float64:
		return SchemaField{Type: "number", Format: "double", FMin: ToPtr(float64(-JsMaxNum)), FMax: ToPtr(float64(JsMaxNum))}
	case reflect.String:
		return SchemaField{Type: "string"}
	case reflect.Slice:
		return SchemaField{Type: "array", ArrOf: ToPtr(me.schemaField(ty.Elem()))}
	case reflect.Map:
		return SchemaField{Type: "object", Map: ToPtr(me.schemaField(ty.Elem()))}
	case reflect.Struct:
		schema_obj := SchemaField{Type: "object", Fields: map[string]SchemaField{}}
		for i := 0; i < ty.NumField(); i++ {
			field := ty.Field(i)
			if !field.IsExported() {
				continue
			}
			ty_field := field.Type
			if str.Has(ty_field.String(), "yodb.Ref[") {
				ty_field = reflect.TypeOf(int64(0))
			}
			var schema_field SchemaField
			if ty_field.Kind() == reflect.Struct {
				schema_field.Type, schema_field.Ref = "object", SchemaRef(me.EnsureSchemaModel(ty_field))
			} else {
				schema_field = me.schemaField(ty_field)
				if ty_field.Kind() == reflect.String {
					switch field_name_lo := str.Lo(field.Name); true {
					case str.Has(field_name_lo, "emailaddr") && !(str.Has(field_name_lo, "oremailaddr") || str.Has(field_name_lo, "emailaddror")):
						schema_field.Format, schema_field.SMin, schema_field.SMax = "email", 5, 255
					case str.Has(field_name_lo, "password"):
						schema_field.Format, schema_field.SMin, schema_field.SMax = "password", Cfg.YO_AUTH_PWD_MIN_LEN, Cfg.YO_AUTH_PWD_MAX_LEN
					}
				}
			}
			schema_obj.Fields[field.Name] = schema_field
		}
		return schema_obj
	}
	panic(ty.Kind().String())
}

func SchemaRef(id string) string { return "#/components/schemas/" + id }

// TODO: type-recursion-safety
func dummyOf(ty reflect.Type, level int) reflect.Value {
	dummy_ptr := func(dummy reflect.Value) reflect.Value {
		alloc := reflect.New(dummy.Type())
		alloc.Elem().Set(dummy)
		return alloc
	}

	if ty.Kind() == reflect.Pointer {
		return dummyOf(ty.Elem(), level)
	}
	if ty.ConvertibleTo(ReflTypeTime) || ReflTypeTime.ConvertibleTo(ty) || ty.AssignableTo(ReflTypeTime) || ReflTypeTime.AssignableTo(ty) {
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
		dummy.SetInt(-JsMaxNum)
	case reflect.Int:
		dummy.SetInt(-JsMaxNum)
	case reflect.Uint8:
		dummy.SetUint(math.MaxUint8)
	case reflect.Uint16:
		dummy.SetUint(math.MaxUint16)
	case reflect.Uint32:
		dummy.SetUint(math.MaxUint32)
	case reflect.Uint64:
		dummy.SetUint(JsMaxNum)
	case reflect.Uint:
		dummy.SetUint(JsMaxNum)
	case reflect.Float32:
		dummy.SetFloat(math.MaxFloat32)
	case reflect.Float64:
		dummy.SetFloat(math.MaxFloat64)
	case reflect.String:
		dummy.SetString("someStr")
	case reflect.Slice:
		dummy = reflect.MakeSlice(ty, 0, 1)
		ty_item := ty.Elem()
		append_item := dummyOf(ty_item, level+1)
		if ty_item.Kind() == reflect.Pointer {
			append_item = dummy_ptr(append_item)
		}
		dummy = reflect.Append(dummy, append_item)
	case reflect.Map:
		dummy = reflect.MakeMap(ty)
		ty_item := ty.Elem()
		append_item := dummyOf(ty_item, level+1)
		if ty_item.Kind() == reflect.Pointer {
			append_item = dummy_ptr(append_item)
		}
		dummy.SetMapIndex(reflect.ValueOf("someMapKey1"), append_item)
		dummy.SetMapIndex(reflect.ValueOf("some_map_key_2"), append_item)
	case reflect.Struct:
		for i := 0; i < ty.NumField(); i++ {
			field := ty.Field(i)
			if !field.IsExported() {
				continue
			}
			if str.Has(str.Lo(field.Name), "password") {
				dummy.Field(i).Set(reflect.ValueOf("lenOfMin" + str.FromInt(Cfg.YO_AUTH_PWD_MIN_LEN) + "AndMax" + str.FromInt(Cfg.YO_AUTH_PWD_MAX_LEN)))
				continue
			}
			field_dummy := dummyOf(field.Type, level+1)
			if field.Type.Kind() == reflect.Pointer && field_dummy.Kind() != reflect.Pointer {
				field_dummy = dummy_ptr(field_dummy)
			}
			if !field_dummy.IsZero() {
				dummy.Field(i).Set(field_dummy)
			}
		}
	}
	return dummy
}

func DummyOf(ty reflect.Type) any {
	return dummyOf(ty, 0).Interface()
}
