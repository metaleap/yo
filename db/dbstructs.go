package yodb

import (
	"reflect"
	"time"
	"unsafe"

	. "yo/cfg"
	yoctx "yo/ctx"
	q "yo/db/query"
	yojson "yo/json"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

const (
	ColID         = q.C("id_")
	ColCreatedAt  = q.C("dt_made_")
	ColModifiedAt = q.C("dt_mod_")
	numStdCols    = 3

	FieldID         = q.F("Id")
	FieldCreatedAt  = q.F("DtMade")
	FieldModifiedAt = q.F("DtMod")
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
type Arr[T any] sl.Of[T]
type JsonMap[T any] map[string]T
type JsonArr[T any] sl.Of[T]

type IsJsonOf[T any] struct{ self *T }

type Ref[T any, OnDel refOnDel] struct {
	id   I64
	self *T
}
type refOnDel interface{ onDelSql() string }
type RefOnDelCascade struct{}
type RefOnDelPrevent struct{}
type RefOnDelSetNull struct{}
type refOnDelNone struct{}

func (refOnDelNone) onDelSql() string    { panic("unreachable") }
func (RefOnDelCascade) onDelSql() string { return "CASCADE" }
func (RefOnDelPrevent) onDelSql() string { return "RESTRICT" }
func (RefOnDelSetNull) onDelSql() string { return "SET NULL" }
func (me *Ref[_, OnDel]) onDelSql() string {
	var dummy OnDel
	return dummy.onDelSql()
}
func (me Ref[_, _]) Id() I64       { return me.id }
func (me Ref[_, _]) IdRaw() int64  { return int64(me.id) }
func (me Ref[_, _]) IsDbRef() bool { return true } // no direct callers, but checked for by `yo/srv` during codegen
func (me *Ref[_, _]) SetId(id I64) { me.self, me.id = nil, id }
func (me *Ref[T, _]) Get(ctx *yoctx.Ctx) *T {
	if (me.self == nil) && (me.id != 0) && (ctx != nil) {
		me.self = ById[T](ctx, me.id)
	}
	return me.self
}
func (me *Ref[T, _]) structDesc() (ret *structDesc) {
	if ret = desc[T](); !sl.Has(ensureDescs, ret) {
		panic(reflect.TypeOf(me).Elem().String() + " refs a non-`Ensure`d type, which hence has no db table yet/anymore")
	}
	return
}
func (me *Ref[_, _]) MarshalJSON() ([]byte, error) { return []byte(str.FromI64(int64(me.id), 10)), nil }
func (me *Ref[_, _]) UnmarshalJSON(json []byte) error {
	me.self, me.id = nil, 0
	i64, err := str.ToI64(string(json), 10, 64)
	if err == nil {
		me.id = I64(i64)
	}
	return err
}

func DbRef[T any](id I64) dbRef { return Ref[T, refOnDelNone]{id: id} }

type dbRef interface {
	Id() I64
	q.DbRef
}
type dbJsonValue interface {
	init(selfPtr any)
	get() any
	scan([]byte) error
	initOther(unsafe.Pointer) dbJsonValue
	getOther(unsafe.Pointer) any
}

var (
	_ dbJsonValue = &IsJsonOf[any]{}
	_ dbJsonValue = &JsonMap[any]{}
	_ dbJsonValue = &JsonArr[any]{}
)

func (me *IsJsonOf[T]) init(selfPtr any)                               { me.self = selfPtr.(*T) }
func (me *IsJsonOf[T]) scan(jsonb []byte) error                        { return yojson.Unmarshal(jsonb, me.self) }
func (me *IsJsonOf[T]) get() any                                       { return me.self }
func (*IsJsonOf[T]) getOther(ptr unsafe.Pointer) any                   { panic("TODO") }
func (me *IsJsonOf[T]) initOther(dst unsafe.Pointer) (ret dbJsonValue) { panic("TODO") }
func (me *JsonMap[T]) init(any)                                        { *me = JsonMap[T]{} }
func (me *JsonMap[T]) scan(jsonb []byte) error                         { return yojson.Unmarshal(jsonb, me) }
func (me *JsonMap[T]) get() any {
	if (me == nil) || (*me == nil) {
		return JsonMap[T]{}
	}
	return JsonMap[T](*me)
}
func (*JsonMap[T]) getOther(ptr unsafe.Pointer) any { return ((*JsonMap[T])(ptr)).get() }
func (*JsonArr[T]) getOther(ptr unsafe.Pointer) any { return ((*JsonArr[T])(ptr)).get() }
func (me *JsonMap[T]) initOther(dst unsafe.Pointer) (ret dbJsonValue) {
	ret = ((*JsonMap[T])(dst))
	ret.init(nil)
	return
}
func (me *JsonArr[T]) initOther(dst unsafe.Pointer) (ret dbJsonValue) {
	ret = ((*JsonArr[T])(dst))
	ret.init(nil)
	return
}
func (me *JsonArr[T]) init(any)                { *me = []T{} }
func (me *JsonArr[T]) scan(jsonb []byte) error { return yojson.Unmarshal(jsonb, me) }
func (me *JsonArr[T]) get() any {
	if (me == nil) || (*me == nil) {
		return JsonArr[T]{}
	}
	return JsonArr[T](*me)
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

type Constraints interface{ qFs() []q.F }
type Unique[T q.Field] []T
type Index[T q.Field] []T
type ReadOnly[T q.Field] []T
type NoUpdTrigger[T q.Field] []T

func (me Unique[TFld]) qFs() []q.F       { return sl.To(me, func(it TFld) q.F { return it.F() }) }
func (me Index[TFld]) qFs() []q.F        { return sl.To(me, func(it TFld) q.F { return it.F() }) }
func (me ReadOnly[TFld]) qFs() []q.F     { return sl.To(me, func(it TFld) q.F { return it.F() }) }
func (me NoUpdTrigger[TFld]) qFs() []q.F { return sl.To(me, func(it TFld) q.F { return it.F() }) }

type structDesc struct {
	ty          reflect.Type
	tableName   string // defaults to db.NameFrom(structTypeName)
	fields      []q.F  // struct fields marked persistish by being of a `yo/db`-exported type
	cols        []q.C  // for each field above, its db.NameFrom()
	constraints struct {
		uniques      []q.F
		indexed      []q.F
		readOnly     []q.F
		noUpdTrigger []q.F
	}
	mig struct {
		oldTableName            string
		renamesOldColToNewField map[q.C]q.F
		constraintsChanged      bool
	}
}

func isColField(fieldType reflect.Type) bool {
	return sl.Has(okTypes, fieldType) || isDbJsonType(fieldType) || isDbRefType(fieldType) || isDbArrType(fieldType)
}

func isDbJsonType(ty reflect.Type) bool {
	is_db_json_dict_type, is_db_json_arr_type, is_db_json_obj_type := isWhatDbJsonType(ty)
	return is_db_json_obj_type || is_db_json_arr_type || is_db_json_dict_type
}

func isWhatDbJsonType(ty reflect.Type) (isDbJsonDictType bool, isDbJsonArrType bool, isDbJsonObjType bool) {
	if field_type_name := ty.Name(); ty.PkgPath() == yodbPkg.PkgPath() {
		if isDbJsonArrType = str.Begins(field_type_name, "JsonArr[") && str.Ends(field_type_name, "]"); !isDbJsonArrType {
			isDbJsonDictType = str.Begins(field_type_name, "JsonMap[") && str.Ends(field_type_name, "]")
		}
	}
	if (!(isDbJsonArrType || isDbJsonDictType)) && (ty.Kind() == reflect.Struct) {
		for i, l := 0, ty.NumField(); i < l; i++ {
			if field := ty.Field(i); field.Anonymous {
				if field_type := field.Type; (field_type.PkgPath() == yodbPkg.PkgPath()) &&
					str.Begins(field_type.Name(), "IsJsonOf[") && str.Ends(field_type.Name(), "]") {
					isDbJsonObjType = true
					break
				}
			}
		}
	}
	return
}

func isDbRefType(ty reflect.Type) bool { return (dbRefType(ty) != "") }
func dbRefType(ty reflect.Type) string {
	type_name := ty.Name()
	if idx := str.IdxSub(type_name, "Ref["); (idx == 0) && ty.PkgPath() == yodbPkg.PkgPath() && str.Ends(type_name, "]") {
		ret := type_name[idx+len("Ref[") : len(type_name)-1]
		return ret[:str.IdxLast(ret, ',')]
	}
	return ""
}

func isDbArrType(ty reflect.Type) bool { return (dbArrType(ty) != nil) }
func dbArrType(ty reflect.Type) reflect.Type {
	type_name := ty.Name()
	if idx := str.IdxSub(type_name, "Arr["); (idx == 0) && ty.PkgPath() == yodbPkg.PkgPath() && str.Ends(type_name, "]") {
		ret := type_name[idx+len("Arr[") : len(type_name)-1]
		item_type := sl.Where(okTypes, func(it reflect.Type) bool { return ret == (it.PkgPath() + "." + it.Name()) })
		if len(item_type) == 0 {
			return nil
		}
		return item_type[0]
	}
	return nil
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
			if field.Type == tyDateTime.Elem() {
				panic("non-pointer DateTime field '" + field.Name + "' in '" + ty.Name() + "'")
			}
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
	jsonDbVal dbJsonValue
	ty        reflect.Type
}

func reflFieldValueOf[T any](it *T, fieldName q.F) any {
	field, _ := reflect.TypeOf(it).Elem().FieldByName(string(fieldName))
	return reflFieldValue(reflect.ValueOf(it).Elem().FieldByName(string(fieldName)), field.Type)
}

func reflFieldValue(rvField reflect.Value, fieldType reflect.Type) any {
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
		if isDbRefType(fieldType) {
			return *getPtr[I64](addr)
		} else if json_db_val, _ := val.(dbJsonValue); json_db_val != nil {
			return json_db_val.getOther(unsafe.Pointer(addr))
		}
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
			if isDbRefType(me.ty) {
				setPtr(me.ptr, (I64)(it))
			} else {
				panic(str.Fmt("scanner.Scan %T into %s", it, me.ty.String()))
			}
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
			} else if arr_item_type := dbArrType(me.ty); arr_item_type != nil {
				dst := reflect.NewAt(me.ty, (unsafe.Pointer)(me.ptr)).Interface()
				yojson.Load(it, dst)
				return nil
			} else {
				panic(str.Fmt("scanner.Scan %T into %s", it, me.ty.String()))
			}
		}
	case string:
		switch me.ty {
		case tyText:
			setPtr(me.ptr, it)
		default:
			panic(str.Fmt("scanner.Scan %T '%q' into %s", it, it, me.ty.String()))
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

func Ensure[TObj any, TFld q.Field](oldTableName string, renamesOldColToNewField map[q.C]q.F, constraintsChanged bool, constraints ...Constraints) {
	if inited {
		panic("db.Ensure called after db.Init")
	}
	desc := desc[TObj]()
	for _, constraints := range constraints {
		switch constraints := constraints.(type) {
		case Index[TFld]:
			desc.constraints.indexed = constraints.qFs()
		case Unique[TFld]:
			desc.constraints.uniques = constraints.qFs()
		case NoUpdTrigger[TFld]:
			desc.constraints.noUpdTrigger = constraints.qFs()
		case ReadOnly[TFld]:
			desc.constraints.readOnly = constraints.qFs()
		default:
			panic(str.Fmt("%T %#v", constraints, constraints))
		}
	}
	desc.mig.oldTableName, desc.mig.renamesOldColToNewField, desc.mig.constraintsChanged = oldTableName, renamesOldColToNewField, constraintsChanged
	if (len(desc.cols) < 1) || (desc.cols[0] != ColID) {
		panic(desc.tableName + ": first column must be '" + string(ColID))
	} else if (len(desc.cols) < 2) || (desc.cols[1] != ColCreatedAt) {
		panic(desc.tableName + ": second column must be '" + string(ColCreatedAt))
	} else if (len(desc.cols) < 3) || (desc.cols[2] != ColModifiedAt) {
		panic(desc.tableName + ": third column must be '" + string(ColModifiedAt))
	} else if len(desc.cols) < 4 {
		panic(desc.tableName + ": no custom columns")
	}
	ensureDescs = append(ensureDescs, desc)
	registerApiHandlers[TObj, TFld](desc)
}

func Is(ty reflect.Type) (ret bool) {
	_, ret = descs[ty]
	return
}

func ForEachColField[T any](it *T, do func(fieldName q.F, colName q.C, fieldValue any, isZero bool)) {
	desc, rv := desc[T](), reflect.ValueOf(it).Elem()
	if it == nil {
		panic("ForEachField called with nil, check at call-site")
	}
	for i, field_name := range desc.fields {
		field, _ := desc.ty.FieldByName(string(field_name))
		value := reflFieldValue(rv.FieldByName(string(field_name)), field.Type)
		frv := reflect.ValueOf(value)
		do(field_name, desc.cols[i], value, (!frv.IsValid()) || frv.IsZero())
	}
}

func Q[T any](it *T) q.Query {
	var col_eqs []q.Query
	ForEachColField(it, func(fieldName q.F, colName q.C, fieldValue any, isZero bool) {
		if !isZero {
			col_eqs = append(col_eqs, colName.Equal(fieldValue))
		}
	})
	return q.AllTrue(col_eqs...)
}

func doEnsureDbStructTables() {
	var did_write_upd_trigger_func_yet, did_alterations bool
	for _, desc := range ensureDescs {
		ctx := yoctx.NewCtxNonHttp(Cfg.DB_REQ_TIMEOUT, false, "db.Mig: "+desc.tableName) // yoctx.NewNonHttp(Cfg.DB_REQ_TIMEOUT)
		defer ctx.OnDone(nil)
		ctx.DevModeNoCatch = IsDevMode
		ctx.TimingsNoPrintInDevMode = true
		ctx.Timings.Step("open TX")
		ctx.DbTx()

		is_table_rename := (desc.mig.oldTableName != "")
		ctx.Timings.Step("get cur table")
		cur_table := GetTable(ctx, If(is_table_rename, desc.mig.oldTableName, desc.tableName))
		if cur_table == nil {
			ctx.Db.PrintRawSqlInDevMode, ctx.TimingsNoPrintInDevMode = IsDevMode, false
			if is_table_rename {
				panic("outdated table rename: '" + desc.mig.oldTableName + "'")
			}
			ctx.Timings.Step("createTable")
			for _, stmt := range schemaCreateTable(desc, &did_write_upd_trigger_func_yet) {
				_ = doExec(ctx, stmt, nil)
			}

		} else if stmts := schemaAlterTable(desc, cur_table); len(stmts) > 0 {
			ctx.Db.PrintRawSqlInDevMode, ctx.TimingsNoPrintInDevMode, did_alterations = IsDevMode, false, true
			for i, stmt := range stmts {
				ctx.Timings.Step("alterTable " + str.FromInt(i+1) + "/" + str.FromInt(len(stmts)))
				_ = doExec(ctx, stmt, nil)
			}
		}
	}
	if did_alterations {
		panic("performed DB alterations, redeploy & restart after cleaning up the provoking `Ensure` calls")
	}
}

func (me *DateTime) UnmarshalJSON(data []byte) error {
	return ((*time.Time)(me)).UnmarshalJSON(data)
}

func (me *DateTime) MarshalJSON() ([]byte, error) {
	return ((*time.Time)(me)).MarshalJSON()
}

func (me *Text) Set(f func(string) string) {
	*me = (Text)(f((string)(*me)))
}

func (me Text) But(f func(string) string) Text {
	return (Text)(f((string)(me)))
}

func DtFrom(t time.Time) *DateTime {
	var ret DateTime
	ret.SetFrom(t)
	return &ret
}

func DtNow() *DateTime { return DtFrom(time.Now()) }

func (me *DateTime) SetFrom(t time.Time) {
	if me != nil {
		if !t.IsZero() {
			*me = (DateTime)(t)
		}
	}
}
func (me *DateTime) Set(f func(time.Time) time.Time) {
	*me = (DateTime)(f((time.Time)(*me)))
}

func (me DateTime) SinceNow() time.Duration         { return time.Since((time.Time)(me)) }
func (me *DateTime) Sub(it *DateTime) time.Duration { return me.Time().Sub(*it.Time()) }
func (me *DateTime) Time() *time.Time               { return (*time.Time)(me) }
