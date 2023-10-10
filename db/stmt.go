package db

import (
	"yo/str"
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
