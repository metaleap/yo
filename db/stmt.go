package yodb

import (
	q "yo/db/query"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type sqlStmt str.Buf

var _ q.Operand = &sqlStmt{}

func (me *sqlStmt) Sql(buf *str.Buf)                 { buf.WriteString(me.String()) }
func (me *sqlStmt) String() string                   { return (*str.Buf)(me).String() }
func (me *sqlStmt) Eval(any, func(q.C) q.F) any      { panic("*sqlStmt isn't a full `q.Operand`") }
func (me *sqlStmt) Equal(other any) q.Query          { panic("*sqlStmt isn't a full `q.Operand`") }
func (me *sqlStmt) NotEqual(other any) q.Query       { panic("*sqlStmt isn't a full `q.Operand`") }
func (me *sqlStmt) LessThan(other any) q.Query       { panic("*sqlStmt isn't a full `q.Operand`") }
func (me *sqlStmt) GreaterThan(other any) q.Query    { panic("*sqlStmt isn't a full `q.Operand`") }
func (me *sqlStmt) LessOrEqual(other any) q.Query    { panic("*sqlStmt isn't a full `q.Operand`") }
func (me *sqlStmt) GreaterOrEqual(other any) q.Query { panic("*sqlStmt isn't a full `q.Operand`") }
func (me *sqlStmt) Not() q.Query                     { panic("*sqlStmt isn't a full `q.Operand`") }
func (me *sqlStmt) In(set ...any) q.Query            { panic("*sqlStmt isn't a full `q.Operand`") }
func (me *sqlStmt) NotIn(set ...any) q.Query         { panic("*sqlStmt isn't a full `q.Operand`") }

func (me *sqlStmt) delete(from string) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	w("DELETE FROM ")
	w(from)
	return me
}

func (me *sqlStmt) insertViaUnnest(desc *structDesc, needRetIdsForInserts bool, cols ...q.C) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	if len(cols) == 0 {
		cols = desc.cols[numStdCols:]
	}

	w("INSERT INTO ")
	w(desc.tableName)
	w(" (")
	for i, col_name := range cols {
		if i > 0 {
			w(", ")
		}
		w(string(col_name))
	}
	w(")(SELECT * FROM unnest(")
	for i, col_name := range cols {
		if i > 0 {
			w(", ")
		}
		w("(@C")
		w(string(col_name))
		w(")::")
		w(sqlColTypeFrom(desc.fieldTypeOfCol(col_name)))
		w("[]")
	}
	w("))")

	if needRetIdsForInserts {
		w(" RETURNING ")
		w(string(ColID))
	}
	return me
}

func (me *sqlStmt) insert(desc *structDesc, numRows int, upsert bool, needRetIdsForInserts bool, cols ...q.C) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	w("INSERT INTO ")
	w(desc.tableName)
	if (numRows < 1) || (upsert && (numRows > 1)) {
		panic(numRows)
	}
	var non_unique_cols []string
	var non_unique_vals []string
	if len(cols) == 0 {
		cols = desc.cols[numStdCols:]
	}

	w(" (")
	for i, col_name := range cols {
		if i > 0 {
			w(", ")
		}
		w(string(col_name))
		if upsert && !sl.Has(desc.constraints.uniques, desc.fieldNameOfCol(col_name)) {
			non_unique_cols = append(non_unique_cols, string(col_name))
		}
	}
	w(") VALUES ")
	for j := 0; j < numRows; j++ {
		if j > 0 {
			w(", ")
		}
		w("(")
		for i, col_name := range cols {
			field_name := desc.fieldNameOfCol(col_name)
			field, _ := desc.ty.FieldByName(string(field_name))
			is_json_field := isDbJsonType(field.Type)

			if i > 0 {
				w(", ")
			}
			col_val := If(!is_json_field, "", "jsonb_strip_nulls(") +
				"@A" + string(col_name) + str.FromInt(j) +
				If(is_json_field, ")", "")
			w(col_val)
			if upsert && !sl.Has(desc.constraints.uniques, field_name) {
				non_unique_vals = append(non_unique_vals, col_val)
			}
		}
		w(")")
	}

	if upsert && (len(desc.constraints.uniques) > 0) {
		me.insertUpsertAppendum(desc, non_unique_cols, non_unique_vals)
	} else if needRetIdsForInserts {
		w(" RETURNING ")
		w(string(ColID))
	}
	return me
}

func (me *sqlStmt) insertUpsertAppendum(desc *structDesc, nonUniqueCols []string, nonUniqueColVals []string) {
	w := (*str.Buf)(me).WriteString
	w(" ON CONFLICT (")
	for i, unique_field_name := range desc.constraints.uniques {
		if i > 0 {
			w(", ")
		}
		w(string(desc.colNameOfField(unique_field_name)))
	}
	w(") DO ")
	if len(nonUniqueCols) == 0 {
		w(" NOTHING")
	} else {
		w("UPDATE SET ")
		if len(nonUniqueCols) > 1 {
			w("(")
		}
		w(str.Join(nonUniqueCols, ", "))
		if len(nonUniqueCols) > 1 {
			w(")")
		}
		w(" = ")
		if len(nonUniqueColVals) > 1 {
			w("(")
		}
		w(str.Join(nonUniqueColVals, ", "))
		if len(nonUniqueColVals) > 1 {
			w(")")
		}
	}
}

func (me *sqlStmt) update(desc *structDesc, colNames ...q.C) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	w("UPDATE ")
	w(desc.tableName)
	w(" SET ")
	var num_cols int
	for _, col_name := range colNames {
		field_name := desc.fieldNameOfCol(col_name)
		if sl.Has(desc.constraints.readOnly, field_name) {
			continue
		}
		field, _ := desc.ty.FieldByName(string(field_name))

		if num_cols > 0 {
			w(", ")
		}
		w(string(col_name))
		w(" = ")
		if isDbJsonType(field.Type) {
			w("jsonb_strip_nulls(@")
			w(string(col_name))
			w(" )")
		} else {
			w("@")
			w(string(col_name))
			w(" ")
		}
		num_cols++
	}
	if num_cols == 0 {
		panic("buggy update call: len(colNames)==0, include the check at the call site>>>>" + me.String())
	}
	return me
}

func (me *sqlStmt) selCols(desc *structDesc, colsPtr *[]q.C, ignoreAlwaysFetchFields bool) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	w("SELECT ")
	var cols []q.C
	if colsPtr != nil {
		cols = *colsPtr
	}
	if len(cols) == 0 {
		cols = desc.cols
	} else if !ignoreAlwaysFetchFields {
		cols = sl.With(cols, sl.As(desc.constraints.alwaysFetch,
			func(it q.F) q.C { return desc.colNameOfField(it) })...)
	}
	for i, col := range cols {
		if i > 0 {
			w(", ")
		}
		field, _ := desc.ty.FieldByName(string(desc.fields[i]))
		is_arr := isDbArrType(field.Type)
		if is_arr {
			w("array_to_json(")
		}
		w(desc.tableName)
		w(".")
		w(string(col))
		if is_arr {
			w(") AS ")
			w(string(col))
		}
	}
	if colsPtr != nil {
		*colsPtr = cols
	}
	return me
}

func (me *sqlStmt) selCount(desc *structDesc, colName q.C, distinct bool) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	if colName == "" && !distinct {
		colName = "*"
	}
	w("SELECT COUNT(")
	w(desc.tableName)
	w(".")
	w(string(colName))
	w(")")
	if distinct {
		w(" DISTINCT")
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

func (me *sqlStmt) where(desc *structDesc, isMut bool, where q.Query, args pgx.NamedArgs, orderBy ...q.OrderBy) *sqlStmt {
	joins := map[q.F]Pair[string, *structDesc]{}
	var f2c func(*structDesc, q.F, bool) q.C
	f2c = func(d *structDesc, fieldName q.F, noTableName bool) q.C {
		if lhs, rhs, ok := str.Cut(string(fieldName), "."); ok {
			join := joins[q.F(lhs)]
			return q.C(join.Key) + "." + f2c(join.It, q.F(rhs), true)
		}
		return If(noTableName, "", q.C(desc.tableName)+".") + d.colNameOfField(fieldName)
	}

	w := (*str.Buf)(me).WriteString
	if !isMut {
		w(" FROM ")
		w(desc.tableName)
	}
	// add JOINs if any
	var dotteds map[q.F][]string
	if where, _ := where.(interface{ AllDottedFs() map[q.F][]string }); where != nil {
		dotteds = where.AllDottedFs()
	}
	var idx_join int
	if len(dotteds) > 0 {
		w(" LEFT OUTER JOIN ")
		for field_name := range dotteds {
			if idx_join > 0 {
				w(" , ")
			}
			field, _ := desc.ty.FieldByName(string(field_name))
			join_name := "_j_" + str.FromInt(idx_join)
			type_refd := dbRefType(field.Type)
			var sub_desc *structDesc
			for ty, sd := range descs {
				if (ty.PkgPath() + "." + ty.Name()) == type_refd {
					sub_desc = sd
					break
				}
			}
			if sub_desc == nil {
				panic("bad join: " + desc.ty.String() + "." + string(field_name) + " due to no desc for '" + type_refd + "'")
			}
			joins[field_name] = Pair[string, *structDesc]{join_name, sub_desc}
			w(sub_desc.tableName)
			w(" AS ")
			w(join_name)
			w(" ON ")
			w(join_name)
			w(".")
			w(string(ColID))
			w(" = ")
			w(string(f2c(desc, field_name, false)))
			w(" ")
			idx_join++
		}
	}

	if where != nil {
		w(" WHERE (")
		where.Sql((*str.Buf)(me), func(fld q.F) q.C {
			return f2c(desc, fld, false)
		}, args)
		w(")")
	}
	if len(orderBy) > 0 {
		w(" ORDER BY ")
		for i, o := range orderBy {
			if i > 0 {
				w(", ")
			}
			if fld := o.Field(); fld != "" {
				w(string(f2c(desc, fld, false)))
			} else {
				w(desc.tableName)
				w(".")
				w(string(o.Col()))
			}
			w(If(o.Desc(), " DESC", " ASC"))
		}
	}
	return me
}
