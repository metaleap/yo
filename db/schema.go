package yodb

import (
	"reflect"
	. "yo/ctx"
	q "yo/db/query"
	"yo/util/sl"
	"yo/util/str"
)

type TableColumn struct {
	tableName       Text
	ColumnName      Text
	OrdinalPosition U8
	ColumnDefault   Text
	IsNullable      Text
	DataType        Text
}

func GetTable(ctx *Ctx, tableName string) []*TableColumn {
	desc := desc[TableColumn]()
	desc.tableName = "information_schema.columns"
	for i, col_name := range desc.cols {
		desc.cols[i] = q.C(str.TrimR(string(col_name), "_"))
	}

	args := dbArgs{}
	stmt := new(sqlStmt).selCols(desc, desc.cols...).from(desc).
		where(desc, q.C("table_name").Equal(tableName), args).
		orderBy(desc, q.C("table_name").Asc(), q.C("ordinal_position").Asc())
	return doSelect[TableColumn](ctx, stmt, args, 0)
}

func schemaCreateTable(desc *structDesc) (ret []*sqlStmt) {
	stmt_create_table := new(sqlStmt)
	w := (*str.Buf)(stmt_create_table).WriteString
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
			w("bigserial PRIMARY KEY")
		case ColCreated:
			w("timestamp without time zone NOT NULL DEFAULT (current_timestamp)")
		default:
			w(sqlColTypeDeclFrom(desc.ty.Field(i).Type))
		}
	}
	w("\n)")
	ret = append(ret, stmt_create_table)

	var indexed_cols []q.C
	for i, field_name := range desc.fields { // always index foreign-key cols due to ON DELETE trigger perf
		field, _ := desc.ty.FieldByName(string(field_name))
		if "" != isDbRefType(field.Type) {
			indexed_cols = append(indexed_cols, desc.cols[i])
		}
	}

	for _, col_name := range indexed_cols {
		stmt_create_index := new(sqlStmt)
		w := (*str.Buf)(stmt_create_index).WriteString
		w("CREATE INDEX IF NOT EXISTS idx_")
		w(string(col_name))
		w(" ON ")
		w(desc.tableName)
		w(" (")
		w(string(col_name))
		w(")")
		ret = append(ret, stmt_create_index)
	}

	return
}

var sqlDtAltNames = map[string]Text{
	"int2": "smallint",
	"int4": "integer",
	"int8": "bigint",
}

func schemaAlterTable(desc *structDesc, curTable []*TableColumn, oldTableName string, renamesOldColToNewField map[q.C]q.F) (ret []*sqlStmt) {
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
		} else if sql_type_name := sqlColTypeFrom(field.Type); sql_type_name != string(table_col.DataType) && sqlDtAltNames[sql_type_name] != table_col.DataType {
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
	case tyDateTime:
		return "timestamp without time zone"
	default:
		if isDbJsonType(ty) {
			return "jsonb"
		} else if isDbRefType(ty) != "" {
			return "int8"
		}
		panic(ty)
	}
}

func sqlColTypeDeclFrom(ty reflect.Type) string {
	sql_data_type_name := sqlColTypeFrom(ty)
	switch ty {
	case tyBool:
		return sql_data_type_name + " NOT NULL DEFAULT (false)"
	case tyBytes, tyDateTime:
		return sql_data_type_name + " NULL DEFAULT (NULL)"
	case tyF32, tyF64, tyI16, tyI32, tyI64, tyI8, tyU16, tyU32, tyU8:
		return sql_data_type_name + " NOT NULL DEFAULT (0)"
	case tyText:
		return sql_data_type_name + " NOT NULL DEFAULT ('')"
	default:
		if is_db_json_dict_type, is_db_json_arr_type, is_db_json_obj_type := isWhatDbJsonType(ty); is_db_json_obj_type || is_db_json_dict_type || is_db_json_arr_type {
			return sql_data_type_name + " NULL DEFAULT (NULL)"
		} else if isDbRefType(ty) != "" {
			dummy := reflect.New(ty).Interface().(interface {
				structDesc() *structDesc
				refOnDel
			})
			desc := dummy.structDesc()
			return sql_data_type_name + " NULL DEFAULT (NULL) REFERENCES " + desc.tableName + " ON DELETE " + dummy.onDelSql()
		}
		panic(ty)
	}
}
