package db

import (
	"database/sql"
	"reflect"

	. "yo/ctx"
	"yo/str"

	"github.com/jackc/pgx/v5"
)

func Tx(ctx *Ctx, do func(*Ctx)) {
	doTx(ctx, do)
}

func doTx(ctx *Ctx, do func(*Ctx), stmts ...*Stmt) {
	if do != nil && len(stmts) > 0 {
		panic("either `do` or `stmts` should be nil")
	}
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		panic(err)
	}
	ctx.Db.Tx = tx
	defer func() {
		fail := recover()
		if fail == nil {
			fail = tx.Commit()
		}
		if fail != nil {
			_ = tx.Rollback()
			panic(fail)
		}
	}()
	for _, stmt := range stmts {
		_ = doExec(ctx, stmt, nil)
	}
	if do != nil {
		do(ctx)
	}
}

func doExec(ctx *Ctx, stmt *Stmt, args pgx.NamedArgs) sql.Result {
	exec := DB.ExecContext
	if ctx.Db.Tx != nil {
		exec = ctx.Db.Tx.ExecContext
	}
	sql_raw := str.TrimR(stmt.String(), ",")
	println(sql_raw)
	ctx.Timings.Step("dbExec: `" + sql_raw + "`")
	result, err := exec(ctx, sql_raw, args)
	if err != nil {
		panic(err)
	}
	return result
}

func doInsert[T any](ctx *Ctx, it *T) sql.Result {
	panic("TODO")
}

func doSelect[T any](ctx *Ctx, stmt *Stmt, args pgx.NamedArgs) (ret []*T) {
	doStream[T](ctx, stmt, func(rec *T) {
		ret = append(ret, rec)
	}, args)
	return
}

func doStream[T any](ctx *Ctx, stmt *Stmt, onRecord func(*T), args pgx.NamedArgs) {
	query := DB.QueryContext
	if ctx.Db.Tx != nil {
		query = ctx.Db.Tx.QueryContext
	}
	desc := desc[T]()
	sql_raw := stmt.String()
	ctx.Timings.Step("dbQuery: `" + sql_raw + "`")
	rows, err := query(ctx, sql_raw, args)
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
		onRecord(&rec)
	}
	if err = rows.Err(); err != nil {
		panic(err)
	}
}
