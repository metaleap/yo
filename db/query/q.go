package q

import (
	. "yo/util"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type Col string

const (
	opNone  = ""
	opEq    = " = "
	opNeq   = " != "
	opLt    = " < "
	opLeq   = " <= "
	opGt    = " > "
	opGeq   = " >= "
	opIn    = " IN "
	opNotIn = " NOT IN "
	opAnd   = " AND "
	opOr    = " OR "
	opNot   = "NOT "
)

type Query interface {
	Sql(*str.Buf, pgx.NamedArgs)
	String(pgx.NamedArgs) string
}

func AllTrue(conds ...Query) Query      { return &query{op: opAnd, conds: conds} }
func EitherOr(conds ...Query) Query     { return &query{op: opOr, conds: conds} }
func Not(cond Query) Query              { return &query{op: opNot, conds: []Query{cond}} }
func Equal(x any, y any) Query          { return &query{op: opEq, operands: []any{x, y}} }
func NotEqual(x any, y any) Query       { return &query{op: opNeq, operands: []any{x, y}} }
func Less(x any, y any) Query           { return &query{op: opLt, operands: []any{x, y}} }
func LessOrEqual(x any, y any) Query    { return &query{op: opLeq, operands: []any{x, y}} }
func Greater(x any, y any) Query        { return &query{op: opGt, operands: []any{x, y}} }
func GreaterOrEqual(x any, y any) Query { return &query{op: opGeq, operands: []any{x, y}} }
func In(x any, y ...any) Query          { return inOrNotIn(opIn, x, y...) }
func NotIn(x any, y ...any) Query       { return inOrNotIn(opNotIn, x, y...) }
func inOrNotIn(op string, x any, y ...any) Query {
	if len(y) == 0 {
		panic(str.Trim(op + "+empty set"))
	}
	sub_stmt, _ := y[0].(interface{ Sql(*str.Buf) })
	return &query{op: If(((len(y) == 1) && (sub_stmt == nil)), opEq, op), operands: append([]any{x}, y...)}
}

func q() *query {
	return &query{}
}

type query struct {
	op       string
	conds    []Query
	operands []any
}

func (me *query) Sql(buf *str.Buf, args pgx.NamedArgs) {
	me.sql(buf, args)
}

func (me *query) String(args pgx.NamedArgs) string {
	var buf str.Buf
	me.Sql(&buf, args)
	return buf.String()
}

func (me *query) sql(buf *str.Buf, args pgx.NamedArgs) {
	do_arg := func(operand any) {
		if sub_stmt, _ := operand.(interface{ Sql(*str.Buf) }); sub_stmt != nil {
			sub_stmt.Sql(buf)
		} else {
			arg_name := "@a" + str.FromInt(len(args))
			args[arg_name[1:]] = operand
			buf.WriteString(arg_name)
		}
	}

	if (str.Trim(string(me.op)) == "") || ((len(me.conds) == 0) && (len(me.operands) == 0)) ||
		((len(me.conds) != 0) && (len(me.operands) != 0)) ||
		((len(me.operands) != 0) && (len(me.operands) != 2) && (me.op != opIn) && (me.op != opNotIn)) {
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
		is_in_or_notin := (me.op == opIn) || (me.op == opNotIn)
		for i, operand := range me.operands {
			if i > 0 {
				if buf.WriteString(string(me.op)); is_in_or_notin {
					buf.WriteByte('(')
					for j, operand := range me.operands[i:] {
						if j > 0 {
							buf.WriteString(", ")
						}
						do_arg(operand)
					}
					buf.WriteByte(')')
					break
				}
			}
			col_name, is := operand.(Col)
			if is {
				buf.WriteString(string(col_name))
			} else {
				do_arg(operand)
			}
		}
	}
	buf.WriteByte(')')
}
