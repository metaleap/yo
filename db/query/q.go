package q

import (
	. "yo/util"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type C string

func (me C) Equal(other any) Query          { return Equal(me, other) }
func (me C) NotEqual(other any) Query       { return NotEqual(me, other) }
func (me C) LessThan(other any) Query       { return LessThan(me, other) }
func (me C) GreaterThan(other any) Query    { return GreaterThan(me, other) }
func (me C) LessOrEqual(other any) Query    { return LessOrEqual(me, other) }
func (me C) GreaterOrEqual(other any) Query { return GreaterOrEqual(me, other) }
func (me C) In(set ...any) Query            { return In(me, set...) }
func (me C) NotIn(set ...any) Query         { return NotIn(me, set...) }
func (me C) Asc() OrderBy                   { return &orderBy[C]{col: me} }
func (me C) Desc() OrderBy                  { return &orderBy[C]{col: me, desc: true} }

type F string

func (me F) Equal(other any) Query          { return Equal(me, other) }
func (me F) NotEqual(other any) Query       { return NotEqual(me, other) }
func (me F) LessThan(other any) Query       { return LessThan(me, other) }
func (me F) GreaterThan(other any) Query    { return GreaterThan(me, other) }
func (me F) LessOrEqual(other any) Query    { return LessOrEqual(me, other) }
func (me F) GreaterOrEqual(other any) Query { return GreaterOrEqual(me, other) }
func (me F) In(set ...any) Query            { return In(me, set...) }
func (me F) NotIn(set ...any) Query         { return NotIn(me, set...) }
func (me F) Asc() OrderBy                   { return &orderBy[F]{fld: me} }
func (me F) Desc() OrderBy                  { return &orderBy[F]{fld: me, desc: true} }

type A[T any] struct{ It T }

func (me A[T]) Equals(x any) Query     { return Equal(me.It, x) }
func (me A[T]) In(set ...any) Query    { return In(me.It, set...) }
func (me A[T]) NotIn(set ...any) Query { return NotIn(me.It, set...) }

type OrderBy interface {
	Col() C
	Field() F
	Desc() bool
}

type orderBy[T ~string] struct {
	col  C
	fld  F
	desc bool
}

func (me *orderBy[T]) Desc() bool { return me.desc }
func (me *orderBy[T]) Col() C     { return me.col }
func (me *orderBy[T]) Field() F   { return me.fld }

const (
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
	And(...Query) Query
	Or(...Query) Query
	Not() Query
	Sql(*str.Buf, func(F) C, pgx.NamedArgs)
	String(func(F) C, pgx.NamedArgs) string
}

func Equal(x any, y any) Query          { return &query{op: opEq, operands: []any{x, y}} }
func NotEqual(x any, y any) Query       { return &query{op: opNeq, operands: []any{x, y}} }
func LessThan(x any, y any) Query       { return &query{op: opLt, operands: []any{x, y}} }
func LessOrEqual(x any, y any) Query    { return &query{op: opLeq, operands: []any{x, y}} }
func GreaterThan(x any, y any) Query    { return &query{op: opGt, operands: []any{x, y}} }
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
func AllTrue(conds ...Query) Query {
	return If(len(conds) == 1, conds[0], (Query)(&query{op: opAnd, conds: conds}))
}
func EitherOr(conds ...Query) Query {
	return If(len(conds) == 1, conds[0], (Query)(&query{op: opOr, conds: conds}))
}
func Not(cond Query) Query {
	switch q := cond.(*query); q.op {
	case opIn:
		return NotIn(q.operands[0], q.operands[1:]...)
	case opNotIn:
		return In(q.operands[0], q.operands[1:]...)
	case opEq:
		return NotEqual(q.operands[0], q.operands[1])
	case opNeq:
		return Equal(q.operands[0], q.operands[1])
	case opGt:
		return LessOrEqual(q.operands[0], q.operands[1])
	case opLt:
		return GreaterOrEqual(q.operands[0], q.operands[1])
	case opGeq:
		return LessThan(q.operands[0], q.operands[1])
	case opLeq:
		return GreaterThan(q.operands[0], q.operands[1])
	case opNot:
		return q.conds[0]
	}
	return &query{op: opNot, conds: []Query{cond}}
}

func q() *query {
	return &query{}
}

type query struct {
	op       string
	conds    []Query
	operands []any
}

func (me *query) And(conds ...Query) Query { return AllTrue(append([]Query{me}, conds...)...) }
func (me *query) Or(conds ...Query) Query  { return EitherOr(append([]Query{me}, conds...)...) }
func (me *query) Not() Query               { return Not(me) }

func (me *query) Sql(buf *str.Buf, fld2col func(F) C, args pgx.NamedArgs) {
	me.sql(buf, fld2col, args)
}

func (me *query) String(fld2col func(F) C, args pgx.NamedArgs) string {
	var buf str.Buf
	me.Sql(&buf, fld2col, args)
	return buf.String()
}

func (me *query) sql(buf *str.Buf, fld2col func(F) C, args pgx.NamedArgs) {
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
			cond.(*query).sql(buf, fld2col, args)
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

			if col_name, is := operand.(C); is {
				buf.WriteString(string(col_name))
			} else if fld_name, is := operand.(F); is {
				buf.WriteString(string(fld2col(fld_name)))
			} else {
				do_arg(operand)
			}
		}
	}
	buf.WriteByte(')')
}
