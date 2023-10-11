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
		new(Stmt).Select(desc.cols...).From(desc.tableName).Where("id = @id").Limit(1),
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
			args[col_name] = field.Interface()
		}
	}

	result := doExec(ctx, new(Stmt).Insert(desc.tableName, desc.cols[2:]...), args)
	id, err := result.LastInsertId()
	if err != nil {
		panic(err)
	}
	return I64(id)
}

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

func doExec(ctx *Ctx, stmt *Stmt, args dbArgs) sql.Result {
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

func doSelect[T any](ctx *Ctx, stmt *Stmt, args dbArgs) (ret []*T) {
	doStream[T](ctx, stmt, func(rec *T) {
		ret = append(ret, rec)
	}, args)
	return
}

func doStream[T any](ctx *Ctx, stmt *Stmt, onRecord func(*T), args dbArgs) {
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
		rv, col_scanners := reflect.ValueOf(&rec).Elem(), make([]any, len(desc.cols))
		for i := range desc.cols {
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
