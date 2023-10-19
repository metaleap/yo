package yodb

import (
	q "yo/db/query"
	. "yo/util"
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

func (me *sqlStmt) update(desc *structDesc, colNames ...string) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	if len(colNames) == 0 {
		panic("buggy update call: len(colNames)==0, include the check at the call site")
	}
	w("UPDATE ")
	w(desc.tableName)
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

func (me *sqlStmt) from(desc *structDesc) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	w(" FROM ")
	w(desc.tableName)
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

func (me *sqlStmt) where(desc *structDesc, where q.Query, args pgx.NamedArgs) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	if where != nil {
		w(" WHERE (")
		where.Sql((*str.Buf)(me), func(fld q.F) q.C {
			return q.C(desc.tableName) + "." + desc.fieldNameToColName(fld)
		}, args)
		w(")")
	}
	return me
}

func (me *sqlStmt) orderBy(desc *structDesc, orderBy ...q.OrderBy) *sqlStmt {
	w := (*str.Buf)(me).WriteString
	if len(orderBy) > 0 {
		w(" ORDER BY ")
		for i, o := range orderBy {
			if i > 0 {
				w(", ")
			}
			w(desc.tableName)
			w(".")
			if fld := o.Field(); fld != "" {
				w(string(desc.fieldNameToColName(fld)))
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
