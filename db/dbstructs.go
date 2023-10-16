package yodb

import (
	"reflect"
	"time"
	"unsafe"

	. "yo/cfg"
	yoctx "yo/ctx"
	q "yo/db/query"
	yojson "yo/json"
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
type DateTime time.Time
type Map[T any] map[string]T
type Arr[T any] sl.Slice[T]

type jsonDbValue interface {
	init()
	get() any
	scan([]byte) error
}

func (me *Map[T]) init()                   { *me = Map[T]{} }
func (me *Map[T]) scan(jsonb []byte) error { return yojson.Unmarshal(jsonb, me) }
func (me *Map[T]) get() any {
	if (me == nil) || (*me == nil) {
		return Map[T]{}
	}
	return Map[T](*me)
}
func (me *Arr[T]) init()                   { *me = []T{} }
func (me *Arr[T]) scan(jsonb []byte) error { return yojson.Unmarshal(jsonb, me) }
func (me *Arr[T]) get() any {
	if (me == nil) || (*me == nil) {
		return Arr[T]{}
	}
	return Arr[T](*me)
}

var (
	tyBool     = reflect.TypeOf(Bool(false))
	tyBytes    = reflect.TypeOf(Bytes(nil))
	tyI8       = reflect.TypeOf(I8(0))
	tyI16      = reflect.TypeOf(I16(0))
	tyI32      = reflect.TypeOf(I32(0))
	tyI64      = reflect.TypeOf(I64(0))
	tyU8       = reflect.TypeOf(U8(0))
	tyU16      = reflect.TypeOf(U16(0))
	tyU32      = reflect.TypeOf(U32(0))
	tyF32      = reflect.TypeOf(F32(0))
	tyF64      = reflect.TypeOf(F64(0))
	tyText     = reflect.TypeOf(Text(""))
	tyDateTime = reflect.TypeOf(&DateTime{})
	okTypes    = []reflect.Type{
		tyBool,
		tyBytes,
		tyI8, tyI16, tyI32, tyI64, tyU8, tyU16, tyU32,
		tyF32, tyF64,
		tyText,
		tyDateTime,
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
	return sl.Has(okTypes, fieldType) || isDbJsonType(fieldType)
}

func isDbJsonType(ty reflect.Type) bool {
	is_db_json_obj_type, is_db_json_arr_type := isWhatDbJsonType(ty)
	return is_db_json_obj_type || is_db_json_arr_type
}

func isWhatDbJsonType(fieldType reflect.Type) (isDbJsonObjType bool, isDbJsonArrType bool) {
	if field_type_name := fieldType.Name(); fieldType.PkgPath() == PkgInfo.PkgPath() {
		if isDbJsonArrType = str.Begins(field_type_name, "Arr[") && str.Ends(field_type_name, "]"); !isDbJsonArrType {
			isDbJsonObjType = str.Begins(field_type_name, "Map[") && str.Ends(field_type_name, "]")
		}
	}
	return
}

func desc[T any]() (ret *structDesc) {
	var it T
	ty := reflect.TypeOf(it)
	if ret = descs[ty]; ret == nil {
		ret = &structDesc{ty: ty, tableName: NameFrom(ty.Name()), cols: make([]q.C, 0, ty.NumField())}
		descs[ty] = ret
		for i, l := 0, ty.NumField(); i < l; i++ {
			field := ty.Field(i)
			col_name := q.C(NameFrom(field.Name))
			if isColField(field.Type) {
				if !str.IsPrtAscii(field.Name) {
					panic("DB-column fields' names should be ASCII")
				}
				ret.fields, ret.cols = append(ret.fields, q.F(field.Name)), append(ret.cols, col_name)
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
	ptr       uintptr
	jsonDbVal jsonDbValue
	ty        reflect.Type
}

func reflFieldValueOf[T any](it *T, fieldName q.F) any {
	return reflFieldValue(reflect.ValueOf(it).Elem().FieldByName(string(fieldName)))
}

func reflFieldValue(rvField reflect.Value) any {
	if !rvField.IsValid() {
		return nil
	}
	if rvField.CanInterface() { // below unsafe-pointering won't do for jsonDbValue impls, so they must be in public/exported fields
		return rvField.Interface()
	}
	addr := rvField.UnsafeAddr()
	switch val := reflect.New(rvField.Type()).Interface().(type) {
	case *Bool:
		return *getPtr[Bool](addr)
	case *Text:
		return *getPtr[Text](addr)
	case *Bytes:
		return *getPtr[Bytes](addr)
	case *I8:
		return *getPtr[I8](addr)
	case *I16:
		return *getPtr[I16](addr)
	case *I32:
		return *getPtr[I32](addr)
	case *I64:
		return *getPtr[I64](addr)
	case *U8:
		return *getPtr[U8](addr)
	case *U16:
		return *getPtr[U16](addr)
	case *U32:
		return *getPtr[U32](addr)
	case **DateTime:
		return *getPtr[*DateTime](addr)
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

func (me scanner) Scan(it any) error {
	if it == nil {
		return nil
	}
	switch it := it.(type) {
	case bool:
		switch me.ty {
		case tyBool:
			setPtr(me.ptr, it)
		default:
			panic(str.Fmt("scanner.Scan %T into %s", it, me.ty.String()))
		}
	case float64:
		switch me.ty {
		case tyF32:
			setPtr(me.ptr, (F32)(it))
		case tyF64:
			setPtr(me.ptr, (F64)(it))
		default:
			panic(str.Fmt("scanner.Scan %T into %s", it, me.ty.String()))
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
		case tyI64:
			setPtr(me.ptr, (I64)(it))
		default:
			panic(str.Fmt("scanner.Scan %T into %s", it, me.ty.String()))
		}
	case []byte:
		switch me.ty {
		case tyBytes:
			dup := make([]byte, len(it))
			copy(dup, it)
			setPtr(me.ptr, dup)
		default:
			if isDbJsonType(me.ty) && me.jsonDbVal != nil {
				if err := me.jsonDbVal.scan(it); err != nil {
					panic(str.Fmt("scanner.Scan %T into %s: %s", it, me.ty.String(), err.Error()))
				}
			} else {
				panic(str.Fmt("scanner.Scan %T into %s", it, me.ty.String()))
			}
		}
	case string:
		switch me.ty {
		case tyText:
			setPtr(me.ptr, it)
		default:
			panic(str.Fmt("scanner.Scan %T into %s", it, me.ty.String()))
		}
	case time.Time:
		switch me.ty {
		case tyDateTime:
			dup := (DateTime)(it)
			setPtr(me.ptr, &dup)
		default:
			panic(str.Fmt("scanner.Scan %T into %s", it, me.ty.String()))
		}
	default:
		panic(str.Fmt("scanner.Scan %T into %s", it, me.ty.String()))
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
			col_eqs = append(col_eqs, colName.Equal(fieldValue))
		}
	})
	return q.AllTrue(col_eqs...)
}

func doEnsureDbStructTables() {
	for _, desc := range ensureDescs {
		ctx := yoctx.NewDebugNoCatch(Cfg.DB_REQ_TIMEOUT) // yoctx.NewNonHttp(Cfg.DB_REQ_TIMEOUT)
		defer ctx.Dispose()
		ctx.DbTx()
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

func (me *DateTime) UnmarshalJSON(data []byte) error {
	return ((*time.Time)(me)).UnmarshalJSON(data)
}

func (me *DateTime) MarshalJSON() ([]byte, error) {
	return ((*time.Time)(me)).MarshalJSON()
}
