package db

import (
	"slices"

	. "yo/ctx"
)

func ListTables(ctx *Ctx) (ret []string) {
	rows, err := DB.QueryContext(ctx, `
		SELECT table_name, column_name, column_default, is_nullable, data_type FROM information_schema.columns
			WHERE table_name IN
	  			(SELECT table_name FROM information_schema.tables
			  		WHERE (table_type = 'BASE TABLE') AND (table_schema NOT IN ('pg_catalog', 'information_schema')))
	  		ORDER BY table_name, ordinal_position
	`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cols = make([]string, 5)
		rows.Scan(ptrs(cols)...)
		if !slices.Contains(ret, cols[0]) {
			ret = append(ret, cols[0])
		}
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
