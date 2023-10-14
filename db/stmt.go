package yodb

import (
	"reflect"

	q "yo/db/query"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type sqlStmt str.Buf

func (me *sqlStmt) Sql(buf *str.Buf) { buf.WriteString(me.String()) }
func (me *sqlStmt) String() string   { return (*str.Buf)(me).String() }

func (me *sqlStmt) delete(from string) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	w("DELETE FROM ")
	w(from)
	return me
}

func (me *sqlStmt) update(tableName string, colNames ...string) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	if len(colNames) == 0 {
		panic("buggy update call: len(upd)==0, include the check at the call site")
	}
	w("UPDATE ")
	w(tableName)
	w(" SET ")
	for i, name := range colNames {
		if i > 0 {
			w(", ")
		}
		w(name)
		w(" = @")
		w(name)
	}
	return me
}

func (me *sqlStmt) sel(countColName q.C, countDistinct bool, cols ...q.C) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	w("SELECT ")
	if countColName != "" {
		w("COUNT")
		w("(")
		if !countDistinct {
			w(string(countColName))
		} else {
			w("DISTINCT")
			if countColName != "" {
				w(string(countColName))
			}
		}
		w(")")
	} else if len(cols) == 0 {
		w("*")
	} else {
		for i, col := range cols {
			if i > 0 {
				w(", ")
			}
			w(string(col))
		}
	}
	return me
}

func (me *sqlStmt) from(from string) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	if from != "" {
		w(" FROM ")
		w(from)
	}
	return me
}

func (me *sqlStmt) limit(max int) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	if max > 0 {
		w(" LIMIT (")
		w(str.FromInt(max))
		w(")")
	}
	return me
}

func (me *sqlStmt) where(where q.Query, f2c func(q.F) q.C, args pgx.NamedArgs) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	if where != nil {
		w(" WHERE (")
		where.Sql((*str.Buf)(me), f2c, args)
		w(")")
	}
	return me
}

func (me *sqlStmt) orderBy(f2c func(q.F) q.C, orderBy ...q.OrderBy) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	if len(orderBy) > 0 {
		w(" ORDER BY ")
		for i, o := range orderBy {
			if i > 0 {
				w(", ")
			}
			if fld := o.Field(); fld != "" {
				w(string(f2c(fld)))
			} else {
				w(string(o.Col()))
			}
			w(If(o.Desc(), " DESC", " ASC"))
		}
	}
	return me
}

func (me *sqlStmt) insert(into string, numRows int, cols ...q.C) *sqlStmt {
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
			w(string(col_name))
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
				w("@A")
				w(string(col_name))
				if numRows > 1 {
					w(str.FromInt(j))
				}
			}
			w(")")
		}
	}
	if numRows == 1 {
		w(" RETURNING ")
		w(string(ColID))
	}
	return me
}

func (me *sqlStmt) createTable(desc *structDesc) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	w("CREATE TABLE IF NOT EXISTS ")
	w(desc.tableName)
	w(" (\n\t")
	for i, col_name := range desc.cols {
		if i > 0 {
			w(",\n\t")
		}
		w(string(col_name))
		w(" ")
		switch col_name {
		case ColID:
			w(If(desc.idBigInt, "bigserial PRIMARY KEY", " serial PRIMARY KEY"))
		case ColCreated:
			w("timestamp without time zone NOT NULL DEFAULT (current_timestamp)")
		default:
			w(sqlColTypeDeclFrom(desc.ty.Field(i).Type))
		}
	}
	w("\n)")
	return me
}

func alterTable(desc *structDesc, curTable []*TableColumn, oldTableName string, renamesOldColToNewField map[q.C]q.F) (ret []*sqlStmt) {
	if oldTableName == desc.tableName {
		panic("invalid table rename: " + oldTableName)
	}

	cols_gone, fields_new, col_type_changes := []q.C{}, []q.F{}, []q.C{}
	for _, table_col := range curTable {
		col_name := q.C(table_col.ColumnName)
		if (col_name == ColID) || (col_name == ColCreated) {
			continue
		}
		if !sl.Has(desc.cols, col_name) {
			cols_gone = append(cols_gone, col_name)
		} else if field, ok := desc.ty.FieldByName(string(desc.fields[sl.IdxWhere(desc.cols, func(it q.C) bool { return (it == col_name) })])); !ok {
			panic("impossible")
		} else if sql_type_name := sqlColTypeFrom(field.Type); sql_type_name != string(table_col.DataType) {
			col_type_changes = append(col_type_changes, col_name)
			panic(string(col_name) + "<<<oldDT:" + string(table_col.DataType) + ">>>newDT:" + sql_type_name)
		}
	}
	for i, struct_col_name := range desc.cols {
		if !sl.Any(curTable, func(t *TableColumn) bool { return t.ColumnName == Text(struct_col_name) }) {
			fields_new = append(fields_new, desc.fields[i])
		}
	}
	if renamesOldColToNewField != nil {
		for old_col_name, new_field_name := range renamesOldColToNewField {
			new_col_name := q.C(NameFrom(string(new_field_name)))
			if new_col_name == old_col_name {
				panic("invalid column rename: " + old_col_name)
			}
			idx_old, idx_new := sl.IdxOf(cols_gone, old_col_name), sl.IdxOf(fields_new, new_field_name)
			if idx_old < 0 || idx_new < 0 {
				panic(str.Repl("outdated column rename: col '{col}' => field '{fld}'", str.Dict{"col": string(old_col_name), "fld": string(new_field_name)}))
			}
			cols_gone, fields_new = sl.WithoutIdx(cols_gone, idx_old, true), sl.WithoutIdx(fields_new, idx_new, true)
		}
	}

	if oldTableName != "" {
		stmt := new(sqlStmt)
		w := (*str.Buf)(stmt).WriteString
		w("ALTER TABLE ")
		w(oldTableName)
		w(" RENAME TO ")
		w(desc.tableName)

		ret = append(ret, stmt)
	}

	if (len(cols_gone) > 0) || (len(fields_new) > 0) || (len(col_type_changes) > 0) {
		stmt := new(sqlStmt)
		w := (*str.Buf)(stmt).WriteString
		w("ALTER TABLE ")
		w(desc.tableName)
		for _, field_name := range fields_new {
			col_name := desc.cols[sl.IdxOf(desc.fields, field_name)]
			w(" \n\tADD COLUMN IF NOT EXISTS ")
			w(string(col_name))
			w(" ")
			field, _ := desc.ty.FieldByName(string(field_name))
			w(sqlColTypeDeclFrom(field.Type))
			w(",")
		}
		for _, col_name := range cols_gone {
			w(" \n\tDROP COLUMN IF EXISTS ")
			w(string(col_name))
			w(",")
		}
		ret = append(ret, stmt)
	}

	if len(renamesOldColToNewField) > 0 {
		for old_col_name, new_field_name := range renamesOldColToNewField {
			stmt := new(sqlStmt)
			w := (*str.Buf)(stmt).WriteString
			w("ALTER TABLE ")
			w(desc.tableName)
			w(" RENAME COLUMN ")
			w(string(old_col_name))
			w(" TO ")
			w(NameFrom(string(new_field_name)))
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

func sqlColTypeDeclFrom(ty reflect.Type) string {
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
