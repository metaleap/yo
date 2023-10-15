package yodb

import (
	"reflect"

	q "yo/db/query"
	. "yo/util"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type sqlStmt str.Buf

func (me *sqlStmt) Sql(buf *str.Buf)                      { buf.WriteString(me.String()) }
func (me *sqlStmt) String() string                        { return (*str.Buf)(me).String() }
func (me *sqlStmt) Eval(any, func(q.C) q.F) reflect.Value { panic("not a `q.Operand`") }
func (me *sqlStmt) Equal(other any) q.Query               { panic("not a `q.Operand`") }
func (me *sqlStmt) NotEqual(other any) q.Query            { panic("not a `q.Operand`") }
func (me *sqlStmt) LessThan(other any) q.Query            { panic("not a `q.Operand`") }
func (me *sqlStmt) GreaterThan(other any) q.Query         { panic("not a `q.Operand`") }
func (me *sqlStmt) LessOrEqual(other any) q.Query         { panic("not a `q.Operand`") }
func (me *sqlStmt) GreaterOrEqual(other any) q.Query      { panic("not a `q.Operand`") }
func (me *sqlStmt) In(set ...any) q.Query                 { panic("not a `q.Operand`") }
func (me *sqlStmt) NotIn(set ...any) q.Query              { panic("not a `q.Operand`") }

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
