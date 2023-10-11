package yodb

import (
	. "yo/ctx"
	q "yo/db/query"
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

	args := dbArgs{}
	stmt := new(Stmt).Select(desc.cols...).From(desc.tableName).
		Where(If(tableName != "",
			q.Col("table_name").Equals(tableName),
			q.Col("table_name").In(
				new(Stmt).Select("table_name").From("information_schema.tables").
					Where(q.AllTrue(
						q.Col("table_type").Equals("BASE TABLE"),
						q.Col("table_schema").NotIn("pg_catalog", "information_schema"),
					), args),
			),
		), args).
		OrderBy("table_name, ordinal_position")
	flat_results := doSelect[TableColumn](ctx, stmt, args)
	for _, result := range flat_results {
		ret[result.tableName] = append(ret[result.tableName], result)
	}
	return ret
}
