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

func (me *sqlStmt) insert(desc *structDesc, numRows int, cols ...q.C) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	w("INSERT INTO ")
	w(desc.tableName)
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
				field_name := desc.fields[sl.IdxOf(desc.cols, q.C(col_name))]
				field, _ := desc.ty.FieldByName(string(field_name))
				is_json_field := isDbJsonType(field.Type)

				if i > 0 {
					w(", ")
				}
				if is_json_field {
					w("jsonb_strip_nulls(")
				}
				w("@A")
				w(string(col_name))
				if numRows > 1 {
					w(str.FromInt(j))
				}
				if is_json_field {
					w(")")
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

func (me *sqlStmt) update(desc *structDesc, colNames ...string) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	w("UPDATE ")
	w(desc.tableName)
	w(" SET ")
	var num_cols int
	for _, col_name := range colNames {
		field_name := desc.fields[sl.IdxOf(desc.cols, q.C(col_name))]
		if sl.Has(desc.constraints.readOnly, field_name) {
			continue
		}
		field, _ := desc.ty.FieldByName(string(field_name))

		if num_cols > 0 {
			w(", ")
		}
		w(col_name)
		w(" = ")
		if isDbJsonType(field.Type) {
			w("jsonb_strip_nulls(@")
			w(col_name)
			w(")")
		} else {
			w("@")
			w(col_name)
		}
		num_cols++
	}
	if num_cols == 0 {
		panic("buggy update call: len(colNames)==0, include the check at the call site>>>>" + me.String())
	}
	return me
}

func (me *sqlStmt) selCols(desc *structDesc, cols ...q.C) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	w("SELECT ")
	if len(cols) == 0 {
		w(desc.tableName)
		w(".*")
	} else {
		for i, col := range cols {
			if i > 0 {
				w(", ")
			}
			w(desc.tableName)
			w(".")
			w(string(col))
		}
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
	var f2c func(d *structDesc, fieldName q.F) q.C
	f2c = func(d *structDesc, fieldName q.F) q.C {
		if lhs, rhs, ok := str.Cut(string(fieldName), "."); ok {
			join := joins[q.F(lhs)]
			return q.C(join.Key) + "." + f2c(join.It, q.F(rhs))
		}
		return d.cols[sl.IdxOf(d.fields, fieldName)]
	}

	w := (*str.Buf)(me).WriteString
	if !isMut {
		w(" FROM ")
		w(desc.tableName)
	}

	// select user_.* from user_ join user_auth_  on user_.auth_ = user_auth_.id_
	//													where user_auth_.email_addr_ = 'foo321@bar.baz'

	// add JOINs if any
	dotteds := where.(interface{ AllDottedFs() map[q.F][]string }).AllDottedFs()
	var idx_join int
	if len(dotteds) > 0 {
		w(" JOIN ")
		for field_name := range dotteds {
			field, _ := desc.ty.FieldByName(string(field_name))
			join_name := "__j__" + str.FromInt(idx_join)
			sub_desc := descs[field.Type]
			joins[field_name] = Pair[string, *structDesc]{join_name, sub_desc}
			w(sub_desc.tableName)
			w(" ON ")
			w(sub_desc.tableName)
			w(".")
			w(string(ColID))
			w(" = ")
			w(desc.tableName)
			w(".")
			w(string(f2c(field_name)))
			w(" ")
			idx_join++
		}
	}

	if where != nil {
		w(" WHERE (")
		where.Sql((*str.Buf)(me), func(fld q.F) q.C {
			return q.C(desc.tableName) + "." + f2c(fld)
		}, args)
		w(")")
	}
	if len(orderBy) > 0 {
		w(" ORDER BY ")
		for i, o := range orderBy {
			if i > 0 {
				w(", ")
			}
			w(desc.tableName)
			w(".")
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
