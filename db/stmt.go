package db

import (
	"reflect"

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
		if col == "id" {
			w("id ")
			w(If(desc.idBig, "bigserial", "serial"))
			w(" PRIMARY KEY")
		} else {
			w(col)
			var default_value string
			it := reflect.New(desc.ty).Interface()
			switch it.(type) {
			case Bool:
				default_value = "0"
				w(" boolean NOT NULL")
			case Bytes:
				default_value = "NULL"
				w(" bytea NULL")
			case Float:
				default_value = "0"
				w(" double precision NOT NULL")
			case Int:
				default_value = "0"
				w(" bigint NOT NULL")
			case Str:
				default_value = `""`
				w(" text NOT NULL")
			case Time:
				default_value = "NULL"
				w(" timestamp without time zone NULL")
			}
			w(" DEFAULT (")
			w(default_value)
			w(")")
		}
	}
	w("\n)")
	return me
}
