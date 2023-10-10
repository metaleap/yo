package db

import (
	. "yo/ctx"
	"yo/str"
)

func ListTables(ctx *Ctx) (ret []string) {
	rows, err := DB.QueryContext(ctx, `
		SELECT table_schema, table_name FROM information_schema.tables
			WHERE table_schema = 'public'
			ORDER BY table_schema, table_name`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var foo = make([]string, 2)
		rows.Scan(ptrs(foo)...)
		println(str.Fmt("%#v", foo))
		// ret = append(ret, s)
	}
	if err = rows.Err(); err != nil {
		panic(err)
	}
	return
}

func ptrs(slice []string) (ret []any) {
	ret = make([]any, len(slice))
	for i := range slice {
		ret[i] = &slice[i]
	}
	return
}

// var _ sql.Scanner=scanner{}

// type scanner struct {
// }
