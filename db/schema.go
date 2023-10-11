package db

import (
	. "yo/ctx"
	. "yo/util"
)

type TableColumn struct {
	tableName       Text
	ColumnName      Text
	OrdinalPosition U8
	ColumnDefault   Text
	IsNullable      Bool
	DataType        Text
}

func GetTable(ctx *Ctx, tableName string) []*TableColumn {
	tables := ListTables(ctx, tableName)
	return tables[(Text)(tableName)]
}

func ListTables(ctx *Ctx, tableName string) map[Text][]*TableColumn {
	ret := map[Text][]*TableColumn{}
	desc := desc[TableColumn]()
	desc.tableName = "information_schema.columns"
	stmt := new(Stmt).Select(desc.cols...).From(desc.tableName).
		Where(If(tableName != "", "table_name = @table_name",
			"table_name IN ("+
				(new(Stmt).Select("table_name").From("information_schema.tables").
					Where("(table_type = 'BASE TABLE') AND (table_schema NOT IN ('pg_catalog', 'information_schema'))")).
					String()+
				")")).
		OrderBy("table_name, ordinal_position")
	flat_results := doSelect[TableColumn](ctx, stmt, dbArgs{"table_name": tableName})
	for _, result := range flat_results {
		ret[result.tableName] = append(ret[result.tableName], result)
	}
	return ret
}
