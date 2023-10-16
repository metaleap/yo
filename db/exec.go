package yodb

import (
	"database/sql"
	"reflect"
	"time"

	. "yo/ctx"
	q "yo/db/query"
	yojson "yo/json"
	. "yo/util"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type dbArgs = pgx.NamedArgs

func ById[T any](ctx *Ctx, id I64) *T {
	if id <= 0 {
		return nil
	}
	return FindOne[T](ctx, ColID.Equal(id))
}

func Exists[T any](ctx *Ctx, query q.Query) bool {
	return (FindOne[T](ctx, query) != nil)
}

func FindOne[T any](ctx *Ctx, query q.Query, orderBy ...q.OrderBy) *T {
	results := FindMany[T](ctx, query, 1, orderBy...)
	if len(results) == 0 {
		return nil
	}
	return results[0]
}

func FindMany[T any](ctx *Ctx, query q.Query, maxResults int, orderBy ...q.OrderBy) []*T {
	desc, args := desc[T](), dbArgs{}
	return doSelect[T](ctx,
		new(sqlStmt).sel("", false, desc.cols...).from(desc.tableName).where(query, desc.fieldNameToColName, args).orderBy(desc.fieldNameToColName, orderBy...).limit(maxResults), args, maxResults)
}

func Each[T any](ctx *Ctx, query q.Query, maxResults int, orderBy []q.OrderBy, onRecord func(rec *T, enough *bool)) {
	desc, args := desc[T](), dbArgs{}
	doStream[T](ctx, new(sqlStmt).sel("", false, desc.cols...).from(desc.tableName).where(query, desc.fieldNameToColName, args).orderBy(desc.fieldNameToColName, orderBy...).limit(maxResults), onRecord, args)
}

func Page[T any](ctx *Ctx, query q.Query, limit int, orderBy q.OrderBy, pageTok any) (resultsPage []*T, nextPageTok any) {
	if pageTok != nil {
		lt_or_gt := If(orderBy.Desc(), q.LessThan, q.GreaterThan)
		query = lt_or_gt(orderBy.Col(), pageTok).And(query)
	}
	resultsPage = FindMany[T](ctx, query, limit, orderBy)
	if len(resultsPage) > 0 {
		if nextPageTok = reflFieldValueOf(resultsPage[len(resultsPage)-1], orderBy.Field()); nextPageTok == nil && IsDevMode {
			panic("buggy Paged call: shouldn't page on nullable field")
		}
	}
	return
}

func Count[T any](ctx *Ctx, query q.Query, max int, nonNullColumn q.C, distinct *q.C) int64 {
	desc, args := desc[T](), dbArgs{}
	col := If((nonNullColumn != ""), nonNullColumn, ColID)
	if distinct != nil {
		col = *distinct
	}
	results := doSelect[int64](ctx, new(sqlStmt).sel(col, distinct != nil).from(desc.tableName).limit(max).where(query, desc.fieldNameToColName, args), args, 1)
	return *results[0]
}

func CreateOne[T any](ctx *Ctx, rec *T) I64 {
	desc := desc[T]()
	args := make(dbArgs, len(desc.cols)-2)
	ForEachField[T](rec, func(fieldName q.F, colName q.C, fieldValue any, isZero bool) {
		if (colName != ColID) && (colName != ColCreated) {
			args["A"+string(colName)] = fieldValue
		}
	})
	result := doSelect[int64](ctx, new(sqlStmt).insert(desc.tableName, 1, desc.cols[2:]...), args, 1)
	if (len(result) > 0) && (result[0] != nil) {
		return I64(*result[0])
	}
	return 0
}

func CreateMany[T any](ctx *Ctx, recs ...*T) {
	if len(recs) == 0 {
		return
	}
	if len(recs) == 1 {
		_ = CreateOne[T](ctx, recs[0])
		return
	}
	desc := desc[T]()
	args := make(dbArgs, len(recs)*(len(desc.cols)-2))
	for j := range recs {
		ForEachField[T](recs[j], func(fieldName q.F, colName q.C, fieldValue any, isZero bool) {
			if (colName != ColID) && (colName != ColCreated) {
				args["A"+string(colName)+str.FromInt(j)] = fieldValue
			}
		})
	}
	_ = doExec(ctx, new(sqlStmt).insert(desc.tableName, len(recs), desc.cols[2:]...), args)
}

func Delete[T any](ctx *Ctx, where q.Query) int64 {
	if where == nil {
		panic(ErrDbDelete_ExpectedQueryForDelete)
	}
	desc, args := desc[T](), dbArgs{}
	result := doExec(ctx, new(sqlStmt).delete(desc.tableName).where(where, desc.fieldNameToColName, args), args)
	num_rows_affected, err := result.RowsAffected()
	if err != nil {
		panic(err)
	}
	return num_rows_affected
}

func Update[T any](ctx *Ctx, upd *T, includingEmptyOrMissingFields bool, where q.Query) int64 {
	desc, args := desc[T](), dbArgs{}
	col_names, col_vals := []string{}, []any{}
	if upd != nil {
		ForEachField[T](upd, func(fieldName q.F, colName q.C, fieldValue any, isZero bool) {
			if (colName != ColID) && (colName != ColCreated) && (includingEmptyOrMissingFields || !isZero) {
				col_names, col_vals = append(col_names, string(colName)), append(col_vals, fieldValue)
			}
		})
	}
	if len(col_names) == 0 {
		panic(ErrDbUpdate_ExpectedChangesForUpdate)
	}
	id_maybe, _ := reflFieldValueOf(upd, "Id").(I64)
	if where == nil && id_maybe == 0 {
		panic(ErrDbUpdate_ExpectedQueryForUpdate)
	}
	if where == nil {
		where = q.C(ColID).Equal(id_maybe)
	}
	for i, col_name := range col_names {
		args[col_name] = col_vals[i]
	}
	result := doExec(ctx, new(sqlStmt).update(desc.tableName, col_names...).where(where, desc.fieldNameToColName, args), args)
	num_rows_affected, err := result.RowsAffected()
	if err != nil {
		panic(err)
	}
	return num_rows_affected
}

func UpdateIfSameVersion[T any](ctx *Ctx, newVersion *T, oldVersion *T) {
	panic("TODO")
	// var conds []q.Query
	// old, new := map[q.C]any{}, map[q.C]any{}
	// ForEachField[T](oldVersion, func(fieldName q.F, colName q.C, fieldValue any, isZero bool) {
	// 	old[colName] = fieldValue
	// })
	// ForEachField[T](newVersion, func(fieldName q.F, colName q.C, fieldValue any, isZero bool) {
	// 	new[colName] = fieldValue
	// })
}

func doExec(ctx *Ctx, stmt *sqlStmt, args dbArgs) sql.Result {
	sql_raw := str.TrimR(stmt.String(), ",")
	if ctx.Timings.Step("dbExec:"); IsDevMode {
		println(sql_raw + "\n\t" + str.From(args))
	}
	do_exec := DB.ExecContext
	if ctx.Db.Tx != nil {
		do_exec = ctx.Db.Tx.ExecContext
	}
	dbArgsCleanUpForPgx(args)
	result, err := do_exec(ctx, sql_raw, args)
	if err != nil {
		panic(err)
	}
	return result
}

func doSelect[T any](ctx *Ctx, stmt *sqlStmt, args dbArgs, maxResults int) (ret []*T) {
	if maxResults > 0 {
		ret = make([]*T, 0, maxResults)
	}
	doStream[T](ctx, stmt, func(rec *T, endNow *bool) {
		ret = append(ret, rec)
	}, args)
	return
}

func doStream[T any](ctx *Ctx, stmt *sqlStmt, onRecord func(*T, *bool), args dbArgs) {
	sql_raw := stmt.String()
	if ctx.Timings.Step("dbQuery:"); IsDevMode {
		println(sql_raw + "\n\t" + str.From(args))
	}
	do_query := DB.QueryContext
	if ctx.Db.Tx != nil {
		do_query = ctx.Db.Tx.QueryContext
	}

	dbArgsCleanUpForPgx(args)
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
	var abort bool
	for rows.Next() {
		var rec T
		if is_i64_returned_from_insert_or_count {
			if err := rows.Scan(&rec); err != nil {
				panic(err)
			}
			onRecord(&rec, &abort)
			break
		}
		rv, col_scanners := reflect.ValueOf(&rec).Elem(), make([]any, len(struct_desc.cols))
		for i := range struct_desc.cols {
			field := rv.Field(i)
			var json_db_val jsonDbValue
			if isDbJsonType(field.Type()) {
				new := reflect.New(field.Type())
				new.Interface().(jsonDbValue).init()
				field.Set(new.Elem())
				json_db_val, _ = field.Addr().Interface().(jsonDbValue)
			}
			col_scanners[i] = scanner{ptr: field.UnsafeAddr(), jsonDbVal: json_db_val, ty: field.Type()}
		}
		if err = rows.Scan(col_scanners...); err != nil {
			panic(err)
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

func dbArgsCleanUpForPgx(args dbArgs) {
	for k, v := range args {
		if b, is := v.(Bytes); is {
			args[k] = ([]byte)(b)
		} else if _, is := v.(DateTime); is {
			panic("non-pointer DateTime")
		} else if dt, is := v.(*DateTime); is {
			args[k] = (*time.Time)(dt)
		} else if rv := reflect.ValueOf(v); rv.IsValid() {
			if isDbJsonType(rv.Type()) {
				if jsonb, err := yojson.Marshal(v); err == nil {
					args[k] = jsonb
				} else {
					panic(err)
				}
			}
		}
	}
}
