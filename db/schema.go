package db

import (
	. "yo/ctx"
	. "yo/util"

	"github.com/jackc/pgx/v5"
)

type TableColumn struct {
	tableName       Str
	ColumnName      Str
	OrdinalPosition Int
	ColumnDefault   Str
	IsNullable      Bool
	DataType        Str
}

func GetTable(ctx *Ctx, name string) []*TableColumn {

	return nil
}

func ListTables(ctx *Ctx, tableName string) map[Str][]*TableColumn {
	ret := map[Str][]*TableColumn{}
	desc := desc[TableColumn]()
	desc.tableName = "information_schema.columns"
	args, stmt := pgx.NamedArgs{"table_name": tableName}, new(Stmt).Select(desc.cols...).From(desc.tableName).
		Where(If(tableName != "", "table_name = @table_name",
			"table_name IN ("+
				(new(Stmt).Select("table_name").From("information_schema.tables").
					Where("(table_type = 'BASE TABLE') AND (table_schema NOT IN ('pg_catalog', 'information_schema'))")).
					String()+
				")")).
		OrderBy("table_name, ordinal_position")
	if tableName != "" {
		args["table_name"] = tableName
	}
	flat_results := doSelect[TableColumn](ctx, stmt, args)
	for _, result := range flat_results {
		ret[result.tableName] = append(ret[result.tableName], result)
	}
	return ret
}
