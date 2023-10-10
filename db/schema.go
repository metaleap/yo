package db

import (
	"time"
	. "yo/ctx"
)

type Bool bool
type Bytes []byte
type Int int64
type Float float64
type Str string
type Time time.Time

type TableColumn struct {
	tableName       Str
	ColumnName      Str
	OrdinalPosition Int
	ColumnDefault   Str
	IsNullable      Bool
	DataType        Str
}

func ListTables(ctx *Ctx) map[Str][]TableColumn {
	ret := map[Str][]TableColumn{}
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
