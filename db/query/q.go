package q

import (
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type QFld string

type op string

const (
	opNone op = ""
	opEq   op = " == "
	opNeq  op = " != "
	opLt   op = " < "
	opLeq  op = " <= "
	opGt   op = " > "
	opGeq  op = " >= "
	opIn   op = " IN "
	opAnd  op = " AND "
	opOr   op = " OR "
	opNot  op = "NOT "
)

type Query interface {
	Sql() string
}

func All(conds ...Query) Query          { return &query{op: opAnd, conds: conds} }
func EitherOr(conds ...Query) Query     { return &query{op: opOr, conds: conds} }
func Not(cond Query) Query              { return &query{op: opNot, conds: []Query{cond}} }
func Equal(x any, y any) Query          { return &query{op: opEq, operands: []any{x, y}} }
func NotEqual(x any, y any) Query       { return &query{op: opNeq, operands: []any{x, y}} }
func Less(x any, y any) Query           { return &query{op: opLt, operands: []any{x, y}} }
func LessOrEqual(x any, y any) Query    { return &query{op: opLeq, operands: []any{x, y}} }
func Greater(x any, y any) Query        { return &query{op: opGt, operands: []any{x, y}} }
func GreaterOrEqual(x any, y any) Query { return &query{op: opGeq, operands: []any{x, y}} }
func In(x any, y any) Query             { return &query{op: opIn, operands: []any{x, y}} }

func q() *query {
	return &query{}
}

type query struct {
	op       op
	conds    []Query
	operands []any
}

func (me *query) sqlExpr(buf *str.Buf, expr any) {
	switch expr := expr.(type) {
	case bool:
		buf.WriteString(str.Bool(expr))
	case string:
		buf.WriteString(str.Q(expr))
	default:
		panic(expr)
	}
}

func (me *query) Sql() string {
	args := pgx.NamedArgs{}
	var buf str.Buf
	buf.Grow(16)
	me.sql(&buf, args)
	return buf.String()
}

func (me *query) sql(buf *str.Buf, args pgx.NamedArgs) {
	if (str.Trim(string(me.op)) == "") || ((len(me.conds) == 0) && (len(me.operands) == 0)) ||
		((len(me.conds) != 0) && (len(me.operands) != 0)) ||
		((len(me.operands) != 0) && (len(me.operands) != 2)) {
		panic(str.From(me))
	}
	buf.WriteByte('(')
	switch me.op {
	case opAnd, opOr, opNot:
		is_not := (me.op == opNot)
		if is_not && (len(me.conds) != 1) {
			panic(str.From(me))
		}
		for i, cond := range me.conds {
			if is_not || (i > 0) {
				buf.WriteString(string(me.op))
			}
			buf.WriteByte('(')
			cond.(*query).sql(buf, args)
			buf.WriteByte(')')
		}
	default:
		me.sqlExpr(buf, me.operands[0])
		buf.WriteString(string(me.op))
		me.sqlExpr(buf, me.operands[1])
	}
	buf.WriteByte(')')
}
