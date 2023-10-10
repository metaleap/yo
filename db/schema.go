package db

import (
	. "yo/ctx"
)

type TableColumn struct {
	tableName       string
	ColumnName      string
	OrdinalPosition int64
	ColumnDefault   string
	IsNullable      bool
	DataType        string
}

func ListTables(ctx *Ctx) map[string][]TableColumn {
	ret := map[string][]TableColumn{}
	desc := desc[TableColumn]()
	desc.tableName = "information_schema.columns"
	stmt := new(Stmt).Select(desc.cols...).From(desc.tableName).
		Where("table_name IN (" +
			(new(Stmt).Select("table_name").From("information_schema.tables").
				Where("(table_type = 'BASE TABLE') AND (table_schema NOT IN ('pg_catalog', 'information_schema'))")).
				String() + ")").
		OrderBy("table_name, ordinal_position")
	flat_results := doSelect[TableColumn](ctx, stmt)
	for _, result := range flat_results {
		ret[result.tableName] = append(ret[result.tableName], result)
	}
	return ret
}
