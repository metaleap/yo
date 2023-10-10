package db

import (
	"reflect"
	"time"
	"unsafe"

	. "yo/ctx"
)

var descs = map[reflect.Type]*structDesc{}

type structDesc struct {
	ty        reflect.Type
	tableName string
	cols      []string
}

func desc[T any]() (ret *structDesc) {
	var it T
	ty := reflect.TypeOf(it)
	if ret = descs[ty]; ret == nil {
		ret = &structDesc{ty: ty, tableName: ty.Name(), cols: make([]string, 0, ty.NumField())}
		descs[ty] = ret
		for i := 0; i < ty.NumField(); i++ {
			field := ty.Field(i)
			ret.cols = append(ret.cols, field.Name)
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
		setPtr[any](uintptr(me), nil)
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
