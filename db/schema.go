package yodb

import (
	"reflect"
	. "yo/ctx"
	q "yo/db/query"
	. "yo/util"
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
	stmt := new(sqlStmt).selCols(desc).
		where(desc, false, q.C("table_name").Equal(tableName), args,
			q.C("table_name").Asc(), q.C("ordinal_position").Asc())
	return doSelect[TableColumn](ctx, stmt, args, 0)
}

func schemaReCreateIndices(desc *structDesc, renamesOldColToNewField map[q.C]q.F) (ret []*sqlStmt) {
	indexed_cols_and_order := map[q.C]string{ColCreatedAt: "DESC", ColModifiedAt: "DESC"}
	for i, field_name := range desc.fields { // always index foreign-key cols for ON DELETE trigger perf
		if sl.Has(desc.constraints.uniques, field_name) {
			continue // uniques always auto-indexed by default
		}
		field, _ := desc.ty.FieldByName(string(field_name))
		if isDbRefType(field.Type) {
			indexed_cols_and_order[desc.cols[i]] = ""
		}
	}
	for _, field_name := range desc.constraints.indexed { // indexes supplied to `Ensure`
		if sl.Has(desc.constraints.uniques, field_name) {
			continue // uniques always auto-indexed by default
		}
		field, _ := desc.ty.FieldByName(string(field_name))
		order_by := If(field.Type == tyDateTime, "DESC", "")
		col_name := desc.cols[sl.IdxOf(desc.fields, field_name)]
		indexed_cols_and_order[col_name] = order_by
	}

	index_names := make(map[q.C]string, len(indexed_cols_and_order)+len(renamesOldColToNewField))
	{
		stmt_drop_indices := new(sqlStmt)
		w := (*str.Buf)(stmt_drop_indices).WriteString
		w("DROP INDEX IF EXISTS ")
		for i, col_name := range append(Keys(indexed_cols_and_order), Keys(renamesOldColToNewField)...) {
			index_name := "idx_t_" + desc.tableName + "_c_" + string(col_name)
			index_names[col_name] = index_name
			if i > 0 {
				w(",")
			}
			w(index_name)
		}
		ret = append(ret, stmt_drop_indices)
	}
	for col_name, order_by := range indexed_cols_and_order {
		stmt_create_index := new(sqlStmt)
		w := (*str.Buf)(stmt_create_index).WriteString
		w("CREATE INDEX IF NOT EXISTS ")
		w(index_names[col_name])
		w(" ON ")
		w(desc.tableName)
		w(" (")
		w(string(col_name))
		if order_by != "" {
			w(" ")
			w(order_by)
			w(" NULLS LAST")
		}
		w(")")
		ret = append(ret, stmt_create_index)
	}
	return
}

func schemaCreateTable(desc *structDesc, didWriteUpdTriggerFuncYet *bool) (ret []*sqlStmt) {
	{ // create table
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
			case ColCreatedAt, ColModifiedAt:
				w("timestamp without time zone NOT NULL DEFAULT (current_timestamp)")
			default:
				field := desc.ty.Field(i)
				is_unique := sl.Has(desc.constraints.uniques, q.F(field.Name))
				w(sqlColTypeDeclFrom(field.Type, is_unique))
			}
		}
		w("\n)")
		ret = append(ret, stmt_create_table)
	}

	{ // modified-at timestamp column auto-update
		var upd_trigger_on_flds []q.F
		if len(desc.constraints.noUpdTrigger) > 0 {
			upd_trigger_on_flds = sl.Without(desc.fields, desc.constraints.noUpdTrigger...)
		}
		var stmt_make_trigger str.Buf
		stmt_make_trigger.WriteString(str.Repl(If(*didWriteUpdTriggerFuncYet, "", `
				CREATE OR REPLACE FUNCTION on_yo_db_obj_upd()
				RETURNS TRIGGER
				LANGUAGE plpgsql AS
				$func$
				BEGIN
					IF NEW IS DISTINCT FROM OLD THEN
						NEW.{col_name} = now();
						RETURN NEW;
					ELSE
						RETURN NULL;
					END IF;
				END
				$func$;
		`)+If((upd_trigger_on_flds != nil) && (len(upd_trigger_on_flds) == 0), "",
			`CREATE OR REPLACE TRIGGER {table_name}onUpdate BEFORE UPDATE {of_cols} ON {table_name} FOR EACH ROW EXECUTE FUNCTION on_yo_db_obj_upd();`),
			str.Dict{"col_name": string(ColModifiedAt), "table_name": desc.tableName,
				"of_cols": If(len(upd_trigger_on_flds) == 0, "",
					"OF "+str.Join(sl.To(upd_trigger_on_flds, func(it q.F) string { return string(desc.cols[sl.IdxOf(desc.fields, it)]) }), ", "),
				)}))
		*didWriteUpdTriggerFuncYet = true
		ret = append(ret, (*sqlStmt)(&stmt_make_trigger))
	}

	ret = append(ret, schemaReCreateIndices(desc, nil)...)
	return
}

var sqlDtAltNames = map[string]Text{
	"int2":   "smallint",
	"int4":   "integer",
	"int8":   "bigint",
	"text[]": "ARRAY",
}

func init() {
	for k := range sqlDtAltNames {
		sqlDtAltNames[k+"[]"] = "ARRAY"
	}
}

func schemaAlterTable(desc *structDesc, curTable []*TableColumn) (ret []*sqlStmt) {
	if desc.mig.oldTableName == desc.tableName {
		panic("invalid table rename: " + desc.mig.oldTableName)
	}

	cols_gone, fields_new, col_type_changes := []q.C{}, []q.F{}, []q.C{}
	for _, table_col := range curTable {
		col_name := q.C(table_col.ColumnName)
		if (col_name == ColID) || (col_name == ColCreatedAt) || (col_name == ColModifiedAt) {
			continue
		}
		if !sl.Has(desc.cols, col_name) {
			cols_gone = append(cols_gone, col_name)
		} else if field, ok := desc.ty.FieldByName(string(desc.fields[sl.IdxWhere(desc.cols, func(it q.C) bool { return (it == col_name) })])); !ok {
			panic("impossible")
		} else if sql_type_name := sqlColTypeFrom(field.Type); (sql_type_name != string(table_col.DataType)) && (sqlDtAltNames[sql_type_name] != table_col.DataType) {
			col_type_changes = append(col_type_changes, col_name)
			panic(string(col_name) + "<<<oldDT:" + string(table_col.DataType) + ">>>newDT:" + sql_type_name)
		}
	}
	for i, struct_col_name := range desc.cols {
		if !sl.Any(curTable, func(t *TableColumn) bool { return t.ColumnName == Text(struct_col_name) }) {
			fields_new = append(fields_new, desc.fields[i])
		}
	}
	if desc.mig.renamesOldColToNewField != nil {
		for old_col_name, new_field_name := range desc.mig.renamesOldColToNewField {
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

	if desc.mig.oldTableName != "" {
		stmt := new(sqlStmt)
		w := (*str.Buf)(stmt).WriteString
		w("ALTER TABLE ")
		w(desc.mig.oldTableName)
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
			is_unique := sl.Has(desc.constraints.uniques, q.F(field.Name))
			w(sqlColTypeDeclFrom(field.Type, is_unique))
			w(",")
		}
		for _, col_name := range cols_gone {
			w(" \n\tDROP COLUMN IF EXISTS ")
			w(string(col_name))
			w(",")
		}
		ret = append(ret, stmt)
	}

	if len(desc.mig.renamesOldColToNewField) > 0 {
		for old_col_name, new_field_name := range desc.mig.renamesOldColToNewField {
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

	if (len(ret) > 0) || desc.mig.constraintsChanged { // alterations pertinent, re-create all indices
		ret = append(ret, schemaReCreateIndices(desc, desc.mig.renamesOldColToNewField)...)
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
		} else if isDbRefType(ty) {
			return "int8"
		} else if arr_item_type := dbArrType(ty); arr_item_type != nil {
			return sqlColTypeFrom(arr_item_type) + "[]"
		}
		panic(ty)
	}
}

func sqlColTypeDeclFrom(ty reflect.Type, isUnique bool) string {
	sql_data_type_name := sqlColTypeFrom(ty)
	unique_maybe := If(isUnique, " UNIQUE", "")
	switch ty {
	case tyBool:
		return sql_data_type_name + " NOT NULL DEFAULT (false)" + unique_maybe
	case tyBytes, tyDateTime:
		return sql_data_type_name + " NULL DEFAULT (NULL)" + unique_maybe
	case tyF32, tyF64, tyI16, tyI32, tyI64, tyI8, tyU16, tyU32, tyU8:
		return sql_data_type_name + " NOT NULL DEFAULT (0)" + unique_maybe
	case tyText:
		return sql_data_type_name + " NOT NULL DEFAULT ('')" + unique_maybe
	default:
		if is_db_json_dict_type, is_db_json_arr_type, is_db_json_obj_type := isWhatDbJsonType(ty); is_db_json_obj_type || is_db_json_dict_type || is_db_json_arr_type {
			return sql_data_type_name + " NULL DEFAULT (NULL)" + unique_maybe
		} else if isDbRefType(ty) {
			dummy := reflect.New(ty).Interface().(interface {
				structDesc() *structDesc
				refOnDel
			})
			desc := dummy.structDesc()
			return sql_data_type_name + " NULL DEFAULT (NULL)" + unique_maybe + " REFERENCES " + desc.tableName + " ON DELETE " + dummy.onDelSql()
		} else if isDbArrType(ty) {
			if unique_maybe != "" {
				panic("unique constraint on '" + ty.String() + "'")
			}
			return sql_data_type_name + " NULL DEFAULT NULL"
		}
		panic(ty)
	}
}
