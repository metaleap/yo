package db

import (
	"reflect"

	. "yo/ctx"
)

func doSelect[T any](ctx *Ctx, stmt *Stmt, args ...any) (ret []*T) {
	doStream[T](ctx, stmt, func(rec *T) {
		ret = append(ret, rec)
	}, args...)
	return
}

func doStream[T any](ctx *Ctx, stmt *Stmt, onRecord func(*T), args ...any) {
	desc := desc[T]()
	sql_raw := stmt.String()
	ctx.Timings.Step("DB: " + sql_raw)
	rows, err := DB.QueryContext(ctx, sql_raw, args...)
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
