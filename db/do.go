package db

import (
	"reflect"
	"slices"
	"time"
	"unsafe"

	. "yo/ctx"
)

var descs = map[reflect.Type]*structDesc{}
var okTypes = []reflect.Type{
	reflect.TypeOf(Bool(false)),
	reflect.TypeOf(Bytes(nil)),
	reflect.TypeOf(Int(0)),
	reflect.TypeOf(Float(0)),
	reflect.TypeOf(Str("")),
	reflect.TypeOf(Time(time.Time{})),
}

type structDesc struct {
	ty        reflect.Type
	tableName string   // defaults to db.NameFrom(structTypeName)
	fields    []string // struct fields marked persistish by being of a type in `okTypes`
	cols      []string // for each field above, its db.NameFrom()
}

func desc[T any]() (ret *structDesc) {
	var it T
	ty := reflect.TypeOf(it)
	if ret = descs[ty]; ret == nil {
		ret = &structDesc{ty: ty, tableName: NameFrom(ty.Name()), cols: make([]string, 0, ty.NumField())}
		descs[ty] = ret
		for i := 0; i < ty.NumField(); i++ {
			field := ty.Field(i)
			if field_type := field.Type; slices.Contains(okTypes, field_type) {
				println("OK:", field.Name)
				ret.fields, ret.cols = append(ret.fields, field.Name), append(ret.cols, NameFrom(field.Name))
			} else {
				println("SKIP:", field.Name)
			}
		}
	}
	return
}

func doSelect[T any](ctx *Ctx, stmt *Stmt) (ret []T) {
	desc := desc[T]()
	sql_raw := stmt.String()
	ctx.Timings.Step("DB: " + sql_raw)
	rows, err := DB.QueryContext(ctx, sql_raw)
	if rows != nil {
		defer rows.Close()
	}
	if err != nil {
		panic(err)
	}
	for rows.Next() {
		var rec T
		rv, col_ptrs := reflect.ValueOf(&rec).Elem(), make([]any, len(desc.cols))
		for i := range desc.cols {
			col_ptrs[i] = (scanner)(rv.Field(i).UnsafeAddr())
		}
		if err = rows.Scan(col_ptrs...); err != nil {
			panic(err)
		}
		ret = append(ret, rec)
	}
	if err = rows.Err(); err != nil {
		panic(err)
	}
	return
}

type scanner uintptr

func setPtr[T any](at uintptr, value T) {
	var it *T = (*T)((unsafe.Pointer)(at))
	*it = value
}

func (me scanner) Scan(src any) error {
	if src == nil {
		return nil
	}
	switch it := src.(type) {
	case int64:
		setPtr(uintptr(me), it)
	case float64:
		setPtr(uintptr(me), it)
	case bool:
		setPtr(uintptr(me), it)
	case []byte:
		dup := make([]byte, len(it))
		copy(dup, it)
		setPtr(uintptr(me), dup)
	case string:
		setPtr(uintptr(me), it)
	case time.Time:
		setPtr(uintptr(me), it)
	default:
		panic(it)
	}
	return nil
}
