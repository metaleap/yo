package q

import (
	. "yo/util"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type C string

func (me C) Equals(x any) Query     { return Equal(me, x) }
func (me C) In(set ...any) Query    { return In(me, set...) }
func (me C) NotIn(set ...any) Query { return NotIn(me, set...) }
func (me C) Asc() OrderBy           { return OrderBy(me + " ASC") }
func (me C) Desc() OrderBy          { return OrderBy(me + " DESC") }

type A[T any] struct{ It T }

func (me A[T]) Equals(x any) Query     { return Equal(me.It, x) }
func (me A[T]) In(set ...any) Query    { return In(me.It, set...) }
func (me A[T]) NotIn(set ...any) Query { return NotIn(me.It, set...) }

type OrderBy string

const (
	OpNone  = ""
	OpEq    = " = "
	OpNeq   = " != "
	OpLt    = " < "
	OpLeq   = " <= "
	OpGt    = " > "
	OpGeq   = " >= "
	OpIn    = " IN "
	OpNotIn = " NOT IN "
	OpAnd   = " AND "
	OpOr    = " OR "
	OpNot   = "NOT "
)

type Query interface {
	And(...Query) Query
	Or(...Query) Query
	Not() Query
	Sql(*str.Buf, pgx.NamedArgs)
	String(pgx.NamedArgs) string
}

func AllTrue(conds ...Query) Query      { return &query{op: OpAnd, conds: conds} }
func EitherOr(conds ...Query) Query     { return &query{op: OpOr, conds: conds} }
func Not(cond Query) Query              { return &query{op: OpNot, conds: []Query{cond}} }
func Equal(x any, y any) Query          { return &query{op: OpEq, operands: []any{x, y}} }
func NotEqual(x any, y any) Query       { return &query{op: OpNeq, operands: []any{x, y}} }
func Less(x any, y any) Query           { return &query{op: OpLt, operands: []any{x, y}} }
func LessOrEqual(x any, y any) Query    { return &query{op: OpLeq, operands: []any{x, y}} }
func Greater(x any, y any) Query        { return &query{op: OpGt, operands: []any{x, y}} }
func GreaterOrEqual(x any, y any) Query { return &query{op: OpGeq, operands: []any{x, y}} }
func In(x any, y ...any) Query          { return inOrNotIn(OpIn, x, y...) }
func NotIn(x any, y ...any) Query       { return inOrNotIn(OpNotIn, x, y...) }
func inOrNotIn(op string, x any, y ...any) Query {
	if len(y) == 0 {
		panic(str.Trim(op + "+empty set"))
	}
	sub_stmt, _ := y[0].(interface{ Sql(*str.Buf) })
	return &query{op: If(((len(y) == 1) && (sub_stmt == nil)), OpEq, op), operands: append([]any{x}, y...)}
}

func q() *query {
	return &query{}
}

type query struct {
	op       string
	conds    []Query
	operands []any
}

func (me *query) And(also ...Query) Query { return AllTrue(append([]Query{me}, also...)...) }
func (me *query) Or(also ...Query) Query  { return EitherOr(append([]Query{me}, also...)...) }
func (me *query) Not() Query              { return Not(me) }

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
			arg_name := "@A" + str.FromInt(len(args))
			args[arg_name[1:]] = operand
			buf.WriteString(arg_name)
		}
	}

	if (str.Trim(string(me.op)) == "") || ((len(me.conds) == 0) && (len(me.operands) == 0)) ||
		((len(me.conds) != 0) && (len(me.operands) != 0)) ||
		((len(me.operands) != 0) && (len(me.operands) != 2) && (me.op != OpIn) && (me.op != OpNotIn)) {
		panic(str.From(me))
	}
	buf.WriteByte('(')
	switch me.op {
	case OpAnd, OpOr, OpNot:
		is_not := (me.op == OpNot)
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
		is_in_or_notin := (me.op == OpIn) || (me.op == OpNotIn)
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
			col_name, is_col_name := operand.(C)
			if is_col_name {
				buf.WriteString(string(col_name))
			} else {
				do_arg(operand)
			}
		}
	}
	buf.WriteByte(')')
}
