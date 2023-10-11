package yodb

import (
	"reflect"
	"slices"

	q "yo/db/query"
	. "yo/util"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type Stmt str.Buf

func (me *Stmt) Sql(buf *str.Buf) { buf.WriteString(me.String()) }
func (me *Stmt) String() string   { return (*str.Buf)(me).String() }

func (me *Stmt) Select(cols ...string) *Stmt {
	w := (*str.Buf)(me).WriteString
	w("SELECT ")
	if len(cols) == 0 {
		w("*")
	} else {
		for i, col := range cols {
			if i > 0 {
				w(", ")
			}
			w(col)
		}
	}
	return me
}

func (me *Stmt) From(from string) *Stmt {
	w := (*str.Buf)(me).WriteString
	if from != "" {
		w(" FROM ")
		w(from)
	}
	return me
}

func (me *Stmt) Limit(max int) *Stmt {
	w := (*str.Buf)(me).WriteString
	if max > 0 {
		w(" LIMIT (")
		w(str.FromInt(max))
		w(")")
	}
	return me
}

func (me *Stmt) Where(where q.Query, args pgx.NamedArgs) *Stmt {
	w := (*str.Buf)(me).WriteString
	if where != nil {
		w(" WHERE (")
		where.Sql((*str.Buf)(me), args)
		w(")")
	}
	return me
}

func (me *Stmt) OrderBy(orderBy ...q.O) *Stmt {
	w := (*str.Buf)(me).WriteString
	if len(orderBy) > 0 {
		w(" ORDER BY ")
		for i, o := range orderBy {
			if i > 0 {
				w(", ")
			}
			w(string(o))
		}
	}
	return me
}

func (me *Stmt) Insert(into string, numRows int, cols ...string) *Stmt {
	w := (*str.Buf)(me).WriteString
	w("INSERT INTO ")
	w(into)
	if numRows < 1 {
		panic(numRows)
	}
	if len(cols) == 0 {
		if numRows != 1 {
			panic("invalid INSERT: multiple rows but no cols specified")
		}
		w(" DEFAULT VALUES")
	} else {
		w(" (")
		for i, col_name := range cols {
			if i > 0 {
				w(", ")
			}
			w(col_name)
		}
		w(")")
		w(" VALUES ")
		for j := 0; j < numRows; j++ {
			if j > 0 {
				w(", ")
			}
			w("(")
			for i, col_name := range cols {
				if i > 0 {
					w(", ")
				}
				w("@")
				w(col_name)
				if numRows > 1 {
					w(str.FromInt(j))
				}
			}
			w(")")
		}
	}
	if numRows == 1 {
		w(" RETURNING ")
		w(ColNameID)
	}
	return me
}

func (me *Stmt) createTable(desc *structDesc) *Stmt {
	w := (*str.Buf)(me).WriteString
	w("CREATE TABLE IF NOT EXISTS ")
	w(desc.tableName)
	w(" (\n\t")
	for i, col_name := range desc.cols {
		if i > 0 {
			w(",\n\t")
		}
		w(col_name)
		switch col_name {
		case ColNameID:
			w(" ")
			w(If(desc.idBig, "bigserial", "serial"))
			w(" PRIMARY KEY")
		case ColNameCreated:
			w(" timestamp without time zone NOT NULL DEFAULT (current_timestamp)")
		default:
			w(" ")
			w(sqlColDeclFrom(desc.ty.Field(i).Type))
		}
	}
	w("\n)")
	return me
}

func alterTable(desc *structDesc, curTable []*TableColumn, oldTableName string, renamesOldColToNewField map[string]string) (ret []*Stmt) {
	if oldTableName == desc.tableName {
		panic("invalid table rename: " + oldTableName)
	}

	cols_gone, fields_new := []string{}, []string{}
	for _, table_col := range curTable {
		if !slices.Contains(desc.cols, string(table_col.ColumnName)) {
			cols_gone = append(cols_gone, string(table_col.ColumnName))
		}
	}
	for i, struct_col_name := range desc.cols {
		if !slices.ContainsFunc(curTable, func(t *TableColumn) bool { return t.ColumnName == Text(struct_col_name) }) {
			fields_new = append(fields_new, desc.fields[i])
		}
	}
	if renamesOldColToNewField != nil {
		for old_col_name, new_field_name := range renamesOldColToNewField {
			new_col_name := NameFrom(new_field_name)
			if new_col_name == old_col_name {
				panic("invalid column rename: " + old_col_name)
			}
			idx_old, idx_new := slices.Index(cols_gone, old_col_name), slices.Index(fields_new, new_field_name)
			if idx_old < 0 || idx_new < 0 {
				panic(str.Fmt("outdated column rename: col '%s' => field '%s'", old_col_name, new_field_name))
			}
			cols_gone, fields_new = slices.Delete(cols_gone, idx_old, idx_old+1), slices.Delete(fields_new, idx_new, idx_new+1)
		}
	}

	if oldTableName != "" {
		stmt := new(Stmt)
		w := (*str.Buf)(stmt).WriteString
		w("ALTER TABLE ")
		w(oldTableName)
		w(" RENAME TO ")
		w(desc.tableName)

		ret = append(ret, stmt)
	}

	if (len(cols_gone) > 0) || (len(fields_new) > 0) {
		stmt := new(Stmt)
		w := (*str.Buf)(stmt).WriteString
		w("ALTER TABLE ")
		w(desc.tableName)
		for _, field_name := range fields_new {
			col_name := desc.cols[slices.Index(desc.fields, field_name)]
			w(" \n\tADD COLUMN IF NOT EXISTS ")
			w(col_name)
			w(" ")
			field, _ := desc.ty.FieldByName(field_name)
			w(sqlColDeclFrom(field.Type))
			w(",")
		}
		for _, col_name := range cols_gone {
			w(" \n\tDROP COLUMN IF EXISTS ")
			w(col_name)
			w(",")
		}
		ret = append(ret, stmt)
	}

	if len(renamesOldColToNewField) > 0 {
		for old_col_name, new_field_name := range renamesOldColToNewField {
			stmt := new(Stmt)
			w := (*str.Buf)(stmt).WriteString
			w("ALTER TABLE ")
			w(desc.tableName)
			w(" RENAME COLUMN ")
			w(old_col_name)
			w(" TO ")
			w(NameFrom(new_field_name))
			ret = append(ret, stmt)
		}
	}

	return
}

func sqlColTypeFrom(ty reflect.Type) string {
	switch ty {
	case tyBool:
		return "boolean"
	case tyBytes:
		return "bytea"
	case tyF32:
		return "float4"
	case tyF64:
		return "float8"
	case tyI8, tyI16, tyU8:
		return "int2"
	case tyI32, tyU16:
		return "int4"
	case tyI64, tyU32:
		return "int8"
	case tyText:
		return "text"
	case tyTimestamp:
		return "timestamp without time zone"
	default:
		panic(ty)
	}
}

func sqlColDeclFrom(ty reflect.Type) string {
	sql_data_type_name := sqlColTypeFrom(ty)
	switch ty {
	case tyBool:
		return sql_data_type_name + " NOT NULL DEFAULT (false)"
	case tyBytes, tyTimestamp:
		return sql_data_type_name + " NULL DEFAULT (NULL)"
	case tyF32, tyF64, tyI16, tyI32, tyI64, tyI8, tyU16, tyU32, tyU8:
		return sql_data_type_name + " NOT NULL DEFAULT (0)"
	case tyText:
		return sql_data_type_name + " NOT NULL DEFAULT ('')"
	default:
		panic(ty)
	}
}
