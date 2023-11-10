package yodb

import (
	"bytes"
	"database/sql"
	"reflect"
	"time"
	"unsafe"

	. "yo/ctx"
	q "yo/db/query"
	yojson "yo/json"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type dbArgs = pgx.NamedArgs

type SelfVersioningObj interface {
	OnAfterLoaded()
	OnBeforeStoring(isCreate bool) (q.Query, []q.F)
}

func ById[T any](ctx *Ctx, id I64) *T {
	if id <= 0 {
		return nil
	}
	return FindOne[T](ctx, ColID.Equal(id))
}

func Ids[T any](ctx *Ctx, query q.Query) (ret sl.Of[I64]) {
	type obj struct{ Id I64 }
	Each[T](ctx, query, 0, nil, func(rec *T, enough *bool) {
		obj := (*obj)(unsafe.Pointer(rec))
		if obj.Id <= 0 {
			panic("Ids: unsafe.Pointer a no-go after all")
		}
		ret = append(ret, obj.Id)
	}, q.F("Id"))
	return
}

func Exists[T any](ctx *Ctx, query q.Query) bool {
	desc, args := desc[T](), dbArgs{}
	result := doSelect[T](ctx, new(sqlStmt).selCols(desc, &[]q.C{ColID}, true).where(desc, false, query, args).limit(1), args, 1, ColID)
	return (len(result) > 0)
}

func FindOne[T any](ctx *Ctx, query q.Query, orderBy ...q.OrderBy) *T {
	results := FindMany[T](ctx, query, 1, nil, orderBy...)
	if len(results) == 0 {
		return nil
	}
	return results[0]
}

func FindMany[T any](ctx *Ctx, query q.Query, maxResults int, onlyFields []q.F, orderBy ...q.OrderBy) []*T {
	desc, args := desc[T](), dbArgs{}
	cols := make([]q.C, len(onlyFields))
	for i, field_name := range onlyFields {
		cols[i] = desc.colNameOfField(field_name)
	}
	return doSelect[T](ctx,
		new(sqlStmt).selCols(desc, &cols, false).where(desc, false, query, args, orderBy...).limit(maxResults), args, maxResults, cols...)
}

func Each[T any](ctx *Ctx, query q.Query, maxResults int, orderBy []q.OrderBy, onRecord func(rec *T, enough *bool), onlyFields ...q.F) {
	desc, args := desc[T](), dbArgs{}
	cols := make([]q.C, len(onlyFields))
	for i, field_name := range onlyFields {
		cols[i] = desc.colNameOfField(field_name)
	}
	doStream[T](ctx, new(sqlStmt).selCols(desc, &cols, false).where(desc, false, query, args, orderBy...).limit(maxResults), onRecord, args, cols...)
}

func Page[T any](ctx *Ctx, query q.Query, limit int, orderBy q.OrderBy, pageTok any) (resultsPage []*T, nextPageTok any) {
	if pageTok != nil {
		lt_or_gt := If(orderBy.Desc(), q.LessThan, q.GreaterThan)
		query = lt_or_gt(orderBy.Col(), pageTok).And(query)
	}
	resultsPage = FindMany[T](ctx, query, limit, nil, orderBy)
	if len(resultsPage) > 0 {
		if nextPageTok = reflFieldValueOf(resultsPage[len(resultsPage)-1], orderBy.Field()); nextPageTok == nil && IsDevMode {
			panic("buggy Paged call: shouldn't page on nullable field")
		}
	}
	return
}

func Count[T any](ctx *Ctx, query q.Query, nonNullColumn q.C, distinct *q.C) int64 {
	desc, args := desc[T](), dbArgs{}
	col := If((nonNullColumn != ""), nonNullColumn, ColID)
	if distinct != nil {
		col = *distinct
	}
	results := doSelect[int64](ctx, new(sqlStmt).selCount(desc, col, distinct != nil).where(desc, false, query, args), args, 1)
	return *results[0]
}

func Delete[T any](ctx *Ctx, where q.Query) int64 {
	if where == nil {
		panic(ErrDbDelete_ExpectedQueryForDelete)
	}
	desc, args := desc[T](), dbArgs{}
	result := doExec(ctx, new(sqlStmt).delete(desc.tableName).where(desc, true, where, args), args)
	num_rows_affected, err := result.RowsAffected()
	if err != nil {
		panic(err)
	}
	return num_rows_affected
}

func Update[T any](ctx *Ctx, upd *T, where q.Query, skipNullsyFields bool, onlyFields ...q.F) int64 {
	desc, args := desc[T](), dbArgs{}
	col_names, col_vals := make([]q.C, 0, len(onlyFields)), make([]any, 0, len(onlyFields))
	var query_and q.Query

	self_versioning, is_self_versioning := ((any)(upd)).(SelfVersioningObj)
	if self_versioning != nil {
		var only_fields_add []q.F
		if query_and, only_fields_add = self_versioning.OnBeforeStoring(false); len(onlyFields) > 0 {
			onlyFields = sl.With(onlyFields, only_fields_add...)
		}
	}
	ForEachColField[T](upd, func(fieldName q.F, colName q.C, fieldValue any, isZero bool) {
		if only := (len(onlyFields) > 0); (colName != ColID) && (colName != ColCreatedAt) && (colName != ColModifiedAt) &&
			((!only) || sl.Has(onlyFields, fieldName)) &&
			((!isZero) || (!skipNullsyFields) || only) {
			col_names, col_vals = append(col_names, colName), append(col_vals, fieldValue)
		}
	})

	if len(col_names) == 0 {
		panic(ErrDbUpdate_ExpectedChangesForUpdate)
	}

	if where == nil { // ensuring the query has either the obj id...
		id_maybe, _ := reflFieldValueOf(upd, FieldID).(I64)
		if lower := q.F(str.Lo(string(FieldID))); (id_maybe <= 0) && sl.Has(desc.fields, lower) {
			id_maybe = reflFieldValueOf(upd, lower).(I64)
		}
		dt_maybe, _ := reflFieldValueOf(upd, FieldModifiedAt).(*DateTime)
		if lower := q.F(str.Lo(string(FieldModifiedAt))); (dt_maybe == nil) && sl.Has(desc.fields, lower) {
			dt_maybe = reflFieldValueOf(upd, lower).(*DateTime)
		}
		if id_maybe > 0 {
			where = q.C(ColID).Equal(id_maybe)
		} else if len(onlyFields) > 0 { // ...or else another unique field (that isnt in onlyFields and so is an exists-in-db queryable)
			for _, unique_field_name := range desc.constraints.uniques {
				if !sl.Has(onlyFields, unique_field_name) {
					if field, _ := desc.ty.FieldByName(string(unique_field_name)); isDbRefType(field.Type) {
						if id_other := reflFieldValueOf(upd, q.F(field.Name)).(dbRef).Id(); id_other != 0 {
							where = unique_field_name.Equal(id_other)
							break
						}
					}
				}
			}
		}
		if where == nil {
			panic(ErrDbUpdate_ExpectedQueryForUpdate)
		} else if (dt_maybe != nil) && !is_self_versioning {
			where = where.And(ColModifiedAt.Equal(dt_maybe.Time()))
		}
	}

	for i, col_name := range col_names {
		args[string(col_name)] = col_vals[i]
	}
	result := doExec(ctx, new(sqlStmt).update(desc, col_names...).where(desc, true, where.And(query_and), args), args)
	num_rows_affected, err := result.RowsAffected()
	if err != nil {
		panic(err)
	}
	return num_rows_affected
}

func CreateOne[T any](ctx *Ctx, rec *T) (ret I64) {
	if self_versioning, _ := ((any)(rec)).(SelfVersioningObj); self_versioning != nil {
		_, _ = self_versioning.OnBeforeStoring(true)
	}
	desc := desc[T]()
	args := dbArgsFillForInsertClassic[T](desc, make(dbArgs, len(desc.fields)), []*T{rec})
	result := doSelect[int64](ctx, new(sqlStmt).insert(desc, 1, false, true), args, 1)
	if (len(result) > 0) && (result[0] != nil) {
		ret = I64(*result[0])
	}
	if ret <= 0 {
		panic("unreachable")
	}
	return
}

func CreateMany[T any](ctx *Ctx, recs ...*T) {
	if len(recs) == 0 {
		return
	}
	// if len(recs) == 1 {
	// 	_ = CreateOne[T](ctx, recs[0])
	// 	return
	// }
	upOrInsert[T](ctx, false, recs...)
}

func Upsert[TObj any](ctx *Ctx, obj *TObj) {
	upOrInsert[TObj](ctx, true, obj)
}

func upOrInsert[T any](ctx *Ctx, upsert bool, recs ...*T) {
	if len(recs) == 0 {
		return
	}
	if upsert && (len(recs) > 1) {
		panic("TODO not yet supported until needed: multiple-upserts-in-one-stmt")
	}
	desc := desc[T]()
	args := make(dbArgs, len(desc.fields))
	if _, is_self_versioning := any(recs[0]).(SelfVersioningObj); is_self_versioning {
		for i := range recs {
			self_versioning := any(recs[i]).(SelfVersioningObj)
			_, _ = self_versioning.OnBeforeStoring(!upsert)
		}
	}
	if upsert {
		args = dbArgsFillForInsertClassic(desc, args, recs)
		_ = doExec(ctx, new(sqlStmt).insert(desc, len(recs), true, false), args)
	} else {
		args = dbArgsFillForInsertViaUnnest(desc, args, recs)
		_ = doExec(ctx, new(sqlStmt).insertViaUnnest(desc, false), args)
	}
}

func doExec(ctx *Ctx, stmt *sqlStmt, args dbArgs) sql.Result {
	sql_raw := str.TrimSuff(stmt.String(), ",")
	do_exec := DB.ExecContext
	if ctx.Db.Tx != nil {
		do_exec = ctx.Db.Tx.ExecContext
	}

	args = dbArgsCleanUpForPgx(args)
	printIfDbgMode(ctx, sql_raw, args)
	result, err := do_exec(ctx, sql_raw, args)
	if err != nil {
		panic(err)
	}
	return result
}

func doSelect[T any](ctx *Ctx, stmt *sqlStmt, args dbArgs, maxResults int, cols ...q.C) (ret []*T) {
	if maxResults > 0 {
		ret = make([]*T, 0, Clamp(4, 128, maxResults))
	}
	doStream[T](ctx, stmt, func(rec *T, endNow *bool) {
		if ret = append(ret, rec); (maxResults > 0) && (len(ret) == maxResults) {
			*endNow = true
		}
	}, args, cols...)
	return
}

func doStream[T any](ctx *Ctx, stmt *sqlStmt, onRecord func(*T, *bool), args dbArgs, cols ...q.C) {
	sql_raw := stmt.String()
	do_query := DB.QueryContext
	if ctx.Db.Tx != nil {
		do_query = ctx.Db.Tx.QueryContext
	}

	args = dbArgsCleanUpForPgx(args)
	printIfDbgMode(ctx, sql_raw, args)
	rows, err := do_query(ctx, sql_raw, args)
	if rows != nil {
		defer rows.Close()
	}
	if err != nil {
		panic(err)
	}
	var struct_desc *structDesc
	_, is_i64_returned_from_insert_or_count := ((any)(new(T))).(*int64)
	if !is_i64_returned_from_insert_or_count {
		struct_desc = desc[T]()
	}
	if (len(cols) == 0) && (struct_desc != nil) {
		cols = struct_desc.cols
	}
	var abort bool
	for rows.Next() {
		var rec T
		if is_i64_returned_from_insert_or_count {
			if err := rows.Scan(&rec); err != nil {
				panic(err)
			}
			abort = true
			onRecord(&rec, &abort)
			break
		}
		rv, col_scanners := reflect.ValueOf(&rec).Elem(), make([]any, len(cols))
		for i, col_name := range cols {
			field_name := struct_desc.fieldNameOfCol(col_name)
			field := rv.FieldByName(string(field_name))
			field_t, _ := struct_desc.ty.FieldByName(string(field_name))
			var json_db_val dbJsonValue
			unsafe_addr := field.UnsafeAddr()
			if isDbRefType(field.Type()) {
				unsafe_addr = field.FieldByName("id").UnsafeAddr()
			} else if is_db_json_dict_type, is_db_json_arr_type, is_db_json_obj_type := isWhatDbJsonType(field.Type()); is_db_json_dict_type || is_db_json_arr_type {
				ptr := reflect.New(field.Type())
				dummy := ptr.Interface().(dbJsonValue)
				dummy.init(nil)
				if field_t.IsExported() {
					field.Set(ptr.Elem())
					json_db_val = field.Addr().Interface().(dbJsonValue)
				} else {
					json_db_val = dummy.initOther(unsafe.Pointer(unsafe_addr))
				}
			} else if is_db_json_obj_type {
				ptr := field.Addr().Interface()
				json_db_val = ptr.(dbJsonValue)
				json_db_val.init(ptr)
			}
			col_scanners[i] = scanner{ptr: unsafe_addr, jsonDbVal: json_db_val, ty: field.Type()}
		}
		if err = rows.Scan(col_scanners...); err != nil {
			panic(err)
		}
		if self_versioning, _ := ((any)(&rec)).(SelfVersioningObj); self_versioning != nil {
			self_versioning.OnAfterLoaded()
		}
		onRecord(&rec, &abort)
		if abort {
			break
		}
	}
	if err = rows.Err(); err != nil {
		panic(err)
	}
}

func dbArgsCleanUpForPgx(args dbArgs) dbArgs {
	var do_cleanup func(any) any
	do_cleanup = func(v any) any {
		if v == nil {
			return nil
		}
		if arr_anys, is := v.([]any); is {
			for i, it := range arr_anys {
				arr_anys[i] = do_cleanup(it)
			}
		} else if b, is := v.(Bytes); is {
			return ([]byte)(b)
		} else if _, is := v.(DateTime); is {
			panic("buggy code: non-pointer DateTime met in dbArgsCleanUpForPgx")
		} else if dt, is := v.(*DateTime); is {
			if dt == nil {
				return nil
			} else {
				return time.Time(*dt)
			}
		} else if db_ref, _ := v.(dbRef); db_ref != nil {
			id := db_ref.Id()
			return If[any](id == 0, nil, id)
		} else if rv := reflect.ValueOf(v); !rv.IsValid() {
			panic(v)
		} else if rvt := rv.Type(); isDbJsonType(rvt) {
			jsonb := yojson.From(v, false)
			if bytes.Equal(jsonb, yojson.JsonTokEmptyArr) || bytes.Equal(jsonb, yojson.JsonTokEmptyObj) || bytes.Equal(jsonb, yojson.JsonTokNull) {
				return nil
			} else {
				return jsonb
			}
		}
		return v
	}
	for k, v := range args {
		args[k] = do_cleanup(v)
	}
	return args
}

func dbArgsFillForInsertViaUnnest[T any](desc *structDesc, args dbArgs, recs []*T, cols ...q.C) dbArgs {
	if len(cols) == 0 {
		cols = desc.cols[numStdCols:]
	}
	for _, col_name := range cols {
		arg_name := "C" + string(col_name)
		args[arg_name] = []any{}
	}
	for _, rec := range recs {
		ForEachColField[T](rec, func(fieldName q.F, colName q.C, fieldValue any, isZero bool) {
			if (colName != ColID) && (colName != ColCreatedAt) && (colName != ColModifiedAt) {
				arg_name := "C" + string(colName)
				args[arg_name] = append(args[arg_name].([]any), fieldValue)
			}
		})
	}
	return args
}

func dbArgsFillForInsertClassic[T any](desc *structDesc, args dbArgs, recs []*T) dbArgs {
	for i, rec := range recs {
		ForEachColField[T](rec, func(fieldName q.F, colName q.C, fieldValue any, isZero bool) {
			if (colName != ColID) && (colName != ColCreatedAt) && (colName != ColModifiedAt) {
				args["A"+string(colName)+str.FromInt(i)] = fieldValue
			}
		})
	}
	return args
}

func printIfDbgMode(ctx *Ctx, sqlRaw string, args dbArgs) {
	if (IsDevMode || !IsUp) && ctx.Db.PrintRawSqlInDevMode {
		println("\n" + sqlRaw + "\n\t" + str.GoLike(args))
	}
}
