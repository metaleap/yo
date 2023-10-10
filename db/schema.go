package db

import (
	. "yo/ctx"
)

type table_column struct {
	table_name       string
	column_name      string
	ordinal_position int64
	column_default   string
	is_nullable      bool
	data_type        string
}

func ListTables(ctx *Ctx) map[string][]table_column {
	ret := map[string][]table_column{}
	desc := desc[table_column]()
	desc.tableName = "information_schema.columns"
	stmt := new(Stmt).Select(desc.cols...).From(desc.tableName).
		Where("table_name IN (" +
			(new(Stmt).Select("table_name").From("information_schema.tables").
				Where("(table_type = 'BASE TABLE') AND (table_schema NOT IN ('pg_catalog', 'information_schema'))")).
				String() + ")").
		OrderBy("table_name, ordinal_position")
	flat_results := doSelect[table_column](ctx, stmt)
	for _, result := range flat_results {
		ret[result.table_name] = append(ret[result.table_name], result)
	}
	return ret
}
