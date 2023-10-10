package db

import (
	"reflect"
	"slices"

	"yo/str"
	. "yo/util"
)

type Stmt str.Buf

func (me *Stmt) String() string { return (*str.Buf)(me).String() }

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

func (me *Stmt) Where(where string) *Stmt {
	w := (*str.Buf)(me).WriteString
	if where != "" {
		w(" WHERE (")
		w(where)
		w(")")
	}
	return me
}

func (me *Stmt) OrderBy(orderBy string) *Stmt {
	w := (*str.Buf)(me).WriteString
	if orderBy != "" {
		w(" ORDER BY ")
		w(orderBy)
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
	stmt := new(Stmt)
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
			idx_old, idx_new := slices.Index(cols_gone, old_col_name), slices.Index(fields_new, new_field_name)
			if idx_old < 0 || idx_new < 0 {
				panic(str.Fmt("outdated column rename: col '%s' => field '%s'", old_col_name, new_field_name))
			}
			cols_gone, fields_new = slices.Delete(cols_gone, idx_old, idx_old+1), slices.Delete(fields_new, idx_new, idx_new+1)
		}
	}
	if (len(cols_gone) == 0) && (len(fields_new) == 0) && (len(renamesOldColToNewField) == 0) {
		return nil
	}

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
	if len(renamesOldColToNewField) > 0 {
		stmt = new(Stmt)
		w = (*str.Buf)(stmt).WriteString
		w("ALTER TABLE ")
		w(desc.tableName)

		for old_col_name, new_field_name := range renamesOldColToNewField {

		}
	}

	return
}

func sqlColDeclFrom(ty reflect.Type) string {
	switch ty {
	case tyBool:
		return "boolean NOT NULL DEFAULT (0)"
	case tyBytes:
		return "bytea NULL DEFAULT (NULL)"
	case tyFloat:
		return "double precision NOT NULL DEFAULT (0)"
	case tyInt:
		return "bigint NOT NULL DEFAULT (0)"
	case tyText:
		return "text NOT NULL DEFAULT ('')"
	case tyTimestamp:
		return "timestamp without time zone NULL DEFAULT (NULL)"
	default:
		panic(ty)
	}
}
