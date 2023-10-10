package db

import (
	"reflect"
	"slices"
	"time"
	"unsafe"

	. "yo/config"
	"yo/ctx"
	"yo/str"
)

const (
	ColNameID      string = "id"
	ColNameCreated string = "created"
)

type Bool bool
type Bytes []byte
type Int int64
type Float float64
type Text string
type DateTime *time.Time

type Base struct {
	ID      Int
	Created DateTime
}

var (
	tyBool      = reflect.TypeOf(Bool(false))
	tyBytes     = reflect.TypeOf(Bytes(nil))
	tyInt       = reflect.TypeOf(Int(0))
	tyFloat     = reflect.TypeOf(Float(0))
	tyText      = reflect.TypeOf(Text(""))
	tyTimestamp = reflect.TypeOf(DateTime(nil))
	okTypes     = []reflect.Type{
		tyBool,
		tyBytes,
		tyInt,
		tyFloat,
		tyText,
		tyTimestamp,
	}
	descs = map[reflect.Type]*structDesc{}
)

type structDesc struct {
	ty        reflect.Type
	tableName string   // defaults to db.NameFrom(structTypeName)
	fields    []string // struct fields marked persistish by being of a type in `okTypes`
	cols      []string // for each field above, its db.NameFrom()
	idBig     bool     // allow up to 9223372036854775807 instead of up to 2147483647
}

func desc[T any]() (ret *structDesc) {
	var it T
	ty := reflect.TypeOf(it)
	if ret = descs[ty]; ret == nil {
		ret = &structDesc{ty: ty, tableName: NameFrom(ty.Name()), cols: make([]string, 0, ty.NumField())}
		descs[ty] = ret
		for i, l := 0, ty.NumField(); i < l; i++ {
			field := ty.Field(i)
			if field_type := field.Type; slices.Contains(okTypes, field_type) {
				ret.fields, ret.cols = append(ret.fields, field.Name), append(ret.cols, NameFrom(field.Name))
			}
		}
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
		ptr_dt := new(time.Time)
		*ptr_dt = it
		setPtr(uintptr(me), ptr_dt)
	default:
		panic(it)
	}
	return nil
}

func Ensure[T any](idBig bool, colRenames map[string]string) {
	ctx := ctx.New(nil, nil, Cfg.DB_REQ_TIMEOUT)
	defer ctx.Dispose()
	desc := desc[T]()
	desc.idBig = idBig
	table := GetTable(ctx, desc.tableName)
	if table == nil {
		_ = doExec(ctx, new(Stmt).CreateTable(desc), nil)
	} else {
		cols_new, cols_gone := []string{}, []string{}
		for _, table_col := range table {
			if !slices.Contains(desc.cols, string(table_col.ColumnName)) {
				cols_gone = append(cols_gone, string(table_col.ColumnName))
			}
		}
		for _, struct_col_name := range desc.cols {
			if !slices.ContainsFunc(table, func(t *TableColumn) bool { return t.ColumnName == Text(struct_col_name) }) {
				cols_new = append(cols_new, struct_col_name)
			}
		}
		if colRenames != nil {
			for old_name, new_name := range colRenames {
				idx_old, idx_new := slices.Index(cols_gone, old_name), slices.Index(cols_new, new_name)
				if idx_old < 0 || idx_new < 0 {
					panic(str.Fmt("outdated column rename: '%s' => '%s'", old_name, new_name))
				}
				cols_gone, cols_new = slices.Delete(cols_gone, idx_old, idx_old+1), slices.Delete(cols_new, idx_new, idx_new+1)
			}
		}
	}
}
