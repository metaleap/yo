package db

import (
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

func (me *Stmt) CreateTable(desc *structDesc) *Stmt {
	w := (*str.Buf)(me).WriteString
	w("CREATE TABLE IF NOT EXISTS ")
	w(desc.tableName)
	w(" (\n\t")
	for i, col := range desc.cols {
		if i > 0 {
			w(",\n\t")
		}
		if col == ColNameID {
			w(ColNameID)
			w(" ")
			w(If(desc.idBig, "bigserial", "serial"))
			w(" PRIMARY KEY")
		} else if col == ColNameCreated {
			w(ColNameCreated)
			w(" timestamp without time zone NOT NULL DEFAULT (current_timestamp)")
		} else {
			w(col)
			switch field_type := desc.ty.Field(i).Type; field_type {
			case tyBool:
				w(" boolean NOT NULL DEFAULT (0)")
			case tyBytes:
				w(" bytea NULL DEFAULT (NULL)")
			case tyFloat:
				w(" double precision NOT NULL DEFAULT (0)")
			case tyInt:
				w(" bigint NOT NULL DEFAULT (0)")
			case tyText:
				w(" text NOT NULL DEFAULT ('')")
			case tyTimestamp:
				w(" timestamp without time zone NULL DEFAULT (NULL)")
			default:
				panic(field_type)
			}
		}
	}
	w("\n)")
	return me
}
