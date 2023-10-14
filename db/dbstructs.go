package yodb

import (
	"reflect"
	"time"
	"unsafe"

	. "yo/cfg"
	yoctx "yo/ctx"
	q "yo/db/query"
	yolog "yo/log"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

const (
	ColID      = q.C("id_")
	ColCreated = q.C("created_")
)

type Bool bool
type Bytes []byte
type I64 int64
type I32 int32
type I16 int16
type I8 int8
type U32 uint32
type U16 uint32
type U8 uint8
type F32 float32
type F64 float64
type Text string
type DateTime *time.Time

var (
	tyBool      = reflect.TypeOf(Bool(false))
	tyBytes     = reflect.TypeOf(Bytes(nil))
	tyI8        = reflect.TypeOf(I8(0))
	tyI16       = reflect.TypeOf(I16(0))
	tyI32       = reflect.TypeOf(I32(0))
	tyI64       = reflect.TypeOf(I64(0))
	tyU8        = reflect.TypeOf(U8(0))
	tyU16       = reflect.TypeOf(U16(0))
	tyU32       = reflect.TypeOf(U32(0))
	tyF32       = reflect.TypeOf(F32(0))
	tyF64       = reflect.TypeOf(F64(0))
	tyText      = reflect.TypeOf(Text(""))
	tyTimestamp = reflect.TypeOf(DateTime(nil))
	okTypes     = []reflect.Type{
		tyBool,
		tyBytes,
		tyI8, tyI16, tyI32, tyI64, tyU8, tyU16, tyU32,
		tyF32, tyF64,
		tyText,
		tyTimestamp,
	}
	descs       = map[reflect.Type]*structDesc{}
	ensureDescs []*structDesc
)

type structDesc struct {
	ty        reflect.Type
	tableName string // defaults to db.NameFrom(structTypeName)
	fields    []q.F  // struct fields marked persistish by being of a type in `okTypes`
	cols      []q.C  // for each field above, its db.NameFrom()
	idBigInt  bool   // allow up to 9223372036854775807 instead of up to 2147483647
	mig       struct {
		oldTableName            string
		renamesOldColToNewField map[q.C]q.F
	}
}

func (me *structDesc) fieldNameToColName(fieldName q.F) q.C {
	for i, field_name := range me.fields {
		if field_name == fieldName {
			return me.cols[i]
		}
	}
	panic(fieldName)
}

func isColField(fieldType reflect.Type) bool {
	return sl.Has(okTypes, fieldType)
}

func desc[T any]() (ret *structDesc) {
	var it T
	ty := reflect.TypeOf(it)
	if ret = descs[ty]; ret == nil {
		ret = &structDesc{ty: ty, tableName: NameFrom(ty.Name()), cols: make([]q.C, 0, ty.NumField())}
		descs[ty] = ret
		for i, l := 0, ty.NumField(); i < l; i++ {
			field := ty.Field(i)
			col_name := NameFrom(field.Name)
			if isColField(field.Type) {
				if !str.IsPrtAscii(field.Name) {
					panic("DB-column fields' names should be ASCII")
				}
				ret.fields, ret.cols = append(ret.fields, q.F(field.Name)), append(ret.cols, q.C(col_name))
			}
		}
	}
	for i := len(ret.cols) - 1; i >= 0; i-- {
		if sl.IdxOf(ret.cols, ret.cols[i]) != i {
			panic("duplicate column: '" + ret.cols[i] + "'")
		}
	}
	return
}

type scanner struct {
	ptr uintptr
	ty  reflect.Type
}

func reflFieldValueOf[T any](it *T, fieldName q.F) any {
	return reflFieldValue(reflect.ValueOf(it).Elem().FieldByName(string(fieldName)))
}

func reflFieldValue(rvField reflect.Value) any {
	if !rvField.IsValid() {
		return nil
	}
	addr := rvField.UnsafeAddr()
	switch val := reflect.New(rvField.Type()).Interface().(type) {
	case *Bool:
		return *getPtr[Bool](addr)
	case *Text:
		return *getPtr[Text](addr)
	case *Bytes:
		return *getPtr[Bytes](addr)
	case *I64:
		return *getPtr[I64](addr)
	case *DateTime:
		return *getPtr[DateTime](addr)
	default:
		panic(str.Fmt("reflFieldValue:%T", val))
	}
}

func getPtr[T any](at uintptr) *T {
	return (*T)((unsafe.Pointer)(at))
}

func setPtr[T any](at uintptr, value T) {
	it := getPtr[T](at)
	*it = value
}

func (me scanner) Scan(src any) error {
	if src == nil {
		return nil
	}
	switch it := src.(type) {
	case bool:
		setPtr(me.ptr, it)
	case float64:
		switch me.ty {
		case tyF32:
			setPtr(me.ptr, (F32)(it))
		default:
			setPtr(me.ptr, (F64)(it))
		}
	case int64:
		switch me.ty {
		case tyI16:
			setPtr(me.ptr, (I16)(it))
		case tyI32:
			setPtr(me.ptr, (I32)(it))
		case tyI8:
			setPtr(me.ptr, (I8)(it))
		case tyU8:
			setPtr(me.ptr, (U8)(it))
		case tyU16:
			setPtr(me.ptr, (U16)(it))
		case tyU32:
			setPtr(me.ptr, (U32)(it))
		default:
			setPtr(me.ptr, (I64)(it))
		}
	case []byte:
		dup := make([]byte, len(it))
		copy(dup, it)
		setPtr(me.ptr, dup)
	case string:
		setPtr(me.ptr, it)
	case time.Time:
		ptr_dt := new(time.Time)
		*ptr_dt = it
		setPtr(me.ptr, ptr_dt)
	default:
		panic(it)
	}
	return nil
}

func Ensure[TObj any, TFld ~string](idBigInt bool, oldTableName string, renamesOldColToNewField map[q.C]q.F) {
	if inited {
		panic("db.Ensure called after db.Init")
	}
	desc := desc[TObj]()
	ensureDescs = append(ensureDescs, desc)
	desc.idBigInt, desc.mig.oldTableName, desc.mig.renamesOldColToNewField = idBigInt, oldTableName, renamesOldColToNewField
	if (len(desc.cols) < 1) || (desc.cols[0] != ColID) {
		panic(desc.tableName + ": first column must be '" + string(ColID))
	} else if (len(desc.cols) < 2) || (desc.cols[1] != ColCreated) {
		panic(desc.tableName + ": second column must be '" + string(ColCreated))
	} else if len(desc.cols) < 3 {
		panic(desc.tableName + ": no custom columns")
	}
	registerApiHandlers[TObj, TFld](desc)
}

func Is(ty reflect.Type) (ret bool) {
	_, ret = descs[ty]
	return
}

func ForEachField[T any](it *T, do func(fieldName q.F, colName q.C, fieldValue any, isZero bool)) {
	desc, rv := desc[T](), reflect.ValueOf(it).Elem()
	if it == nil {
		panic("ForEachField called with nil, check at call-site")
	}
	for i, field := range desc.fields {
		value := reflFieldValue(rv.FieldByName(string(field)))
		frv := reflect.ValueOf(value)
		do(field, desc.cols[i], value, (!frv.IsValid()) || frv.IsZero())
	}
}

func Q[T any](it *T) q.Query {
	var col_eqs []q.Query
	ForEachField(it, func(fieldName q.F, colName q.C, fieldValue any, isZero bool) {
		if !isZero {
			col_eqs = append(col_eqs, colName.Equal(q.Lit(fieldValue)))
		}
	})
	return q.AllTrue(col_eqs...)
}

func doEnsureDbStructTables() {
	ctx := yoctx.NewNonHttp(Cfg.DB_REQ_TIMEOUT)
	defer ctx.Dispose()

	for _, desc := range ensureDescs {
		yolog.Println("db.Mig: " + desc.tableName)
		is_table_rename := (desc.mig.oldTableName != "")
		cur_table := GetTable(ctx, If(is_table_rename, desc.mig.oldTableName, desc.tableName))
		if cur_table == nil {
			if !is_table_rename {
				_ = doExec(ctx, new(sqlStmt).createTable(desc), nil)
			} else {
				panic("outdated table rename: '" + desc.mig.oldTableName + "'")
			}
		} else if stmts := alterTable(desc, cur_table, desc.mig.oldTableName, desc.mig.renamesOldColToNewField); len(stmts) > 0 {
			for _, stmt := range stmts {
				_ = doExec(ctx, stmt, nil)
			}
		}
	}
}
