package yodb

import (
	"database/sql"
	"reflect"

	. "yo/ctx"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type dbArgs = pgx.NamedArgs

func Get[T any](ctx *Ctx, id I64) *T {
	if id <= 0 {
		return nil
	}
	desc := desc[T]()
	results := doSelect[T](ctx,
		new(Stmt).Select(desc.cols...).From(desc.tableName).Where(ColNameID+" = @"+ColNameID).Limit(1),
		dbArgs{ColNameID: id})
	if len(results) == 0 {
		return nil
	}
	return results[0]
}

func CreateOne[T any](ctx *Ctx, rec *T) I64 {
	desc := desc[T]()
	args := make(dbArgs, len(desc.cols)-2)
	rv := reflect.ValueOf(rec).Elem()
	for i, col_name := range desc.cols {
		if i >= 2 { // skip 'id' and 'created'
			field := rv.Field(i)
			args[col_name] = reflFieldValue(field)
		}
	}

	result := doSelect[int64](ctx, new(Stmt).Insert(desc.tableName, 1, desc.cols[2:]...), args)
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
				args[col_name+str.FromInt(j)] = reflFieldValue(field)
			}
		}
	}
	_ = doExec(ctx, new(Stmt).Insert(desc.tableName, len(recs), desc.cols[2:]...), args)
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

func doSelect[T any](ctx *Ctx, stmt *Stmt, args dbArgs) (ret []*T) {
	doStream[T](ctx, stmt, func(rec *T) {
		ret = append(ret, rec)
	}, args)
	return
}

func doStream[T any](ctx *Ctx, stmt *Stmt, onRecord func(*T), args dbArgs) {
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
	_, is_id_returned_from_insert := ((any)(new(T))).(*int64)
	if !is_id_returned_from_insert {
		struct_desc = desc[T]()
	}
	for rows.Next() {
		var rec T
		if is_id_returned_from_insert {
			if err := rows.Scan(&rec); err != nil {
				panic(err)
			}
			onRecord(&rec)
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
		onRecord(&rec)
	}
	if err = rows.Err(); err != nil {
		panic(err)
	}
}
