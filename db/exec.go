package yodb

import (
	"database/sql"
	"reflect"

	. "yo/ctx"
	q "yo/db/query"
	. "yo/util"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type dbArgs = pgx.NamedArgs

func Get[T any](ctx *Ctx, id I64) *T {
	if id <= 0 {
		return nil
	}
	return FindOne[T](ctx, ColID.Equals(id))
}

func FindOne[T any](ctx *Ctx, query q.Query, orderBy ...q.OrderBy) *T {
	results := FindAll[T](ctx, query, 1, orderBy...)
	if len(results) == 0 {
		return nil
	}
	return results[0]
}

func FindAll[T any](ctx *Ctx, query q.Query, maxResults int, orderBy ...q.OrderBy) []*T {
	desc, args := desc[T](), dbArgs{}
	return doSelect[T](ctx,
		new(Stmt).sel("", false, desc.cols...).from(desc.tableName).where(query, args).orderBy(orderBy...).limit(maxResults), args, maxResults)
}

func Each[T any](ctx *Ctx, query q.Query, maxResults int, orderBy []q.OrderBy, onRecord func(rec *T, enough *bool)) {
	desc, args := desc[T](), dbArgs{}
	doStream[T](ctx, new(Stmt).sel("", false, desc.cols...).from(desc.tableName).where(query, args).orderBy(orderBy...).limit(maxResults), onRecord, args)
}

func Count[T any](ctx *Ctx, query q.Query, max int, nonNullColumn q.C, distinct *q.C) int64 {
	desc, args := desc[T](), dbArgs{}
	col := If((nonNullColumn != ""), nonNullColumn, ColID)
	if distinct != nil {
		col = *distinct
	}
	results := doSelect[int64](ctx, new(Stmt).sel(col, distinct != nil).from(desc.tableName).limit(max).where(query, args), args, 1)
	return *results[0]
}

func Delete[T any](ctx *Ctx, query q.Query) int64 {
	if query == nil {
		panic("Delete without query")
	}
	desc, args := desc[T](), dbArgs{}
	result := doExec(ctx, new(Stmt).delete(desc.tableName).where(query, args), args)
	num_rows_affected, err := result.RowsAffected()
	if err != nil {
		panic(err)
	}
	return num_rows_affected
}

func CreateOne[T any](ctx *Ctx, rec *T) I64 {
	desc := desc[T]()
	args := make(dbArgs, len(desc.cols)-2)
	rv := reflect.ValueOf(rec).Elem()
	for i, col_name := range desc.cols {
		if i >= 2 { // skip 'id' and 'created'
			field := rv.Field(i)
			args["A"+string(col_name)] = reflFieldValue(field)
		}
	}

	result := doSelect[int64](ctx, new(Stmt).insert(desc.tableName, 1, desc.cols[2:]...), args, 1)
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
	for j, rec := range recs {
		rv := reflect.ValueOf(rec).Elem()
		for i, col_name := range desc.cols {
			if i >= 2 { // skip 'id' and 'created'
				field := rv.Field(i)
				args["A"+string(col_name)+str.FromInt(j)] = reflFieldValue(field)
			}
		}
	}
	_ = doExec(ctx, new(Stmt).insert(desc.tableName, len(recs), desc.cols[2:]...), args)
}

func doExec(ctx *Ctx, stmt *Stmt, args dbArgs) (result sql.Result) {
	sql_raw := str.TrimR(stmt.String(), ",")
	ctx.Timings.Step("dbExec: `" + sql_raw + "`")
	exec := DB.ExecContext
	if ctx.Db.Tx != nil {
		exec = ctx.Db.Tx.ExecContext
	}
	println(sql_raw)
	var err error
	result, err = exec(ctx, sql_raw, args)
	if err != nil {
		panic(err)
	}
	return result
}

func doSelect[T any](ctx *Ctx, stmt *Stmt, args dbArgs, maxResults int) (ret []*T) {
	if maxResults > 0 {
		ret = make([]*T, 0, maxResults)
	}
	doStream[T](ctx, stmt, func(rec *T, endNow *bool) {
		ret = append(ret, rec)
	}, args)
	return
}

func doStream[T any](ctx *Ctx, stmt *Stmt, onRecord func(*T, *bool), args dbArgs) {
	sql_raw := stmt.String()
	ctx.Timings.Step("dbQuery: `" + sql_raw + "`")
	query := DB.QueryContext
	if ctx.Db.Tx != nil {
		query = ctx.Db.Tx.QueryContext
	}
	rows, err := query(ctx, sql_raw, args)
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
			col_scanners[i] = scanner{ptr: field.UnsafeAddr(), ty: field.Type()}
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
