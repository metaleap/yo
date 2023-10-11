package yodb

import (
	. "yo/ctx"
	q "yo/db/query"
	. "yo/util"
	"yo/util/str"
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
	for i, col_name := range desc.cols {
		desc.cols[i] = q.C(str.TrimR(string(col_name), "_"))
	}

	args := dbArgs{}
	stmt := new(Stmt).sel("", false, desc.cols...).from(desc.tableName).
		where(If(tableName != "",
			q.C("table_name").Equal(tableName),
			q.C("table_name").In(
				new(Stmt).sel("", false, "table_name").from("information_schema.tables").
					where(q.C("table_type").Equal("BASE TABLE").And(
						q.C("table_schema").NotIn("pg_catalog", "information_schema"),
					), args),
			),
		), args).
		orderBy(q.C("table_name").Asc(), q.C("ordinal_position").Desc())
	flat_results := doSelect[TableColumn](ctx, stmt, args, If(tableName == "", 0, 1))
	for _, result := range flat_results {
		ret[result.tableName] = append(ret[result.tableName], result)
	}
	return ret
}
