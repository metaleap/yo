package q

import (
	"reflect"

	. "yo/util"
	"yo/util/sl"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

func operandsFrom(it ...any) []Operand {
	return sl.To(it, operandFrom)
}

func operandFrom(it any) Operand {
	if operand, _ := it.(Operand); operand != nil {
		return operand
	}
	return V{Value: it}
}

type C string

func (me C) Equal(other any) Query           { return Equal(me, other) }
func (me C) NotEqual(other any) Query        { return NotEqual(me, other) }
func (me C) LessThan(other any) Query        { return LessThan(me, other) }
func (me C) GreaterThan(other any) Query     { return GreaterThan(me, other) }
func (me C) LessOrEqual(other any) Query     { return LessOrEqual(me, other) }
func (me C) GreaterOrEqual(other any) Query  { return GreaterOrEqual(me, other) }
func (me C) Not() Query                      { return me.Equal(false) }
func (me C) In(set ...any) Query             { return In(me, set...) }
func (me C) NotIn(set ...any) Query          { return NotIn(me, set...) }
func (me C) Asc() OrderBy                    { return &orderBy[C]{col: me} }
func (me C) Desc() OrderBy                   { return &orderBy[C]{col: me, desc: true} }
func (me C) Eval(obj any, c2f func(C) F) any { return c2f(me).Eval(obj, c2f) }

type F string

func (me F) Equal(other any) Query          { return Equal(me, other) }
func (me F) NotEqual(other any) Query       { return NotEqual(me, other) }
func (me F) LessThan(other any) Query       { return LessThan(me, other) }
func (me F) GreaterThan(other any) Query    { return GreaterThan(me, other) }
func (me F) LessOrEqual(other any) Query    { return LessOrEqual(me, other) }
func (me F) GreaterOrEqual(other any) Query { return GreaterOrEqual(me, other) }
func (me F) Not() Query                     { return me.Equal(false) }
func (me F) In(set ...any) Query            { return In(me, set...) }
func (me F) NotIn(set ...any) Query         { return NotIn(me, set...) }
func (me F) Asc() OrderBy                   { return &orderBy[F]{fld: me} }
func (me F) Desc() OrderBy                  { return &orderBy[F]{fld: me, desc: true} }
func (me F) Eval(obj any, _ func(C) F) any {
	return reflect.ValueOf(obj).Elem().FieldByName(string(me)).Interface()
}

type V struct{ Value any }

func (me V) Equal(other any) Query          { return Equal(me, other) }
func (me V) NotEqual(other any) Query       { return NotEqual(me, other) }
func (me V) LessThan(other any) Query       { return LessThan(me, other) }
func (me V) GreaterThan(other any) Query    { return GreaterThan(me, other) }
func (me V) LessOrEqual(other any) Query    { return LessOrEqual(me, other) }
func (me V) GreaterOrEqual(other any) Query { return GreaterOrEqual(me, other) }
func (me V) Not() Query                     { return me.Equal(false) }
func (me V) In(set ...any) Query            { return In(me, set...) }
func (me V) NotIn(set ...any) Query         { return NotIn(me, set...) }
func (me V) Eval(any, func(C) F) any        { return me.Value }

type fn string

const (
	FnStrLen fn = "octet_length"
)

type fun struct {
	Fn   fn
	Args []Operand
	Alt  func(...any) any
}

func Fn(f fn, args ...any) Operand {
	return &fun{Fn: f, Args: operandsFrom(args...)}
}
func Via[TArg any, TRet any](fn func(TArg) TRet) func(any) Operand {
	return func(arg any) Operand {
		ret := Fn("", arg).(*fun)
		ret.Alt = func(args ...any) any { return fn(args[0].(TArg)) }
		return ret
	}
}
func (me *fun) Equal(other any) Query          { return Equal(me, other) }
func (me *fun) NotEqual(other any) Query       { return NotEqual(me, other) }
func (me *fun) LessThan(other any) Query       { return LessThan(me, other) }
func (me *fun) GreaterThan(other any) Query    { return GreaterThan(me, other) }
func (me *fun) LessOrEqual(other any) Query    { return LessOrEqual(me, other) }
func (me *fun) GreaterOrEqual(other any) Query { return GreaterOrEqual(me, other) }
func (me *fun) Not() Query                     { return me.Equal(false) }
func (me *fun) In(set ...any) Query            { return In(me, set...) }
func (me *fun) NotIn(set ...any) Query         { return NotIn(me, set...) }
func (me *fun) Eval(obj any, c2f func(C) F) any {
	if me.Alt != nil {
		return me.Alt(sl.To(me.Args, func(it Operand) any { return it.Eval(obj, c2f) })...)
	}
	switch me.Fn {
	case FnStrLen:
		str := me.Args[0].Eval(obj, c2f).(string)
		return len(str)
	default:
		panic(me.Fn)
	}
}

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

type Query interface {
	And(...Query) Query
	Or(...Query) Query
	Not() Query
	Sql(*str.Buf, func(F) C, pgx.NamedArgs)
	String(func(F) C, pgx.NamedArgs) string
	Eval(any, func(C) F) Query
}

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

func Equal(lhs any, rhs any) Query {
	return &query{op: opEq, operands: operandsFrom(lhs, rhs)}
}
func NotEqual(lhs any, rhs any) Query {
	return &query{op: opNeq, operands: operandsFrom(lhs, rhs)}
}
func LessThan(lhs any, rhs any) Query {
	return &query{op: opLt, operands: operandsFrom(lhs, rhs)}
}
func LessOrEqual(lhs any, rhs any) Query {
	return &query{op: opLeq, operands: operandsFrom(lhs, rhs)}
}
func GreaterThan(lhs any, rhs any) Query {
	return &query{op: opGt, operands: operandsFrom(lhs, rhs)}
}
func GreaterOrEqual(lhs any, rhs any) Query {
	return &query{op: opGeq, operands: operandsFrom(lhs, rhs)}
}
func In(lhs any, rhs ...any) Query { return inNotIn(opIn, operandFrom(lhs), operandsFrom(rhs...)...) }
func NotIn(lhs any, rhs ...any) Query {
	return inNotIn(opNotIn, operandFrom(lhs), operandsFrom(rhs...)...)
}
func inNotIn(op string, lhs Operand, rhs ...Operand) Query {
	if len(rhs) == 0 {
		panic(str.Trim(str.Trim(op) + ": empty set"))
	}
	sub_stmt, _ := rhs[0].(interface{ Sql(*str.Buf) })
	return &query{op: If(((len(rhs) == 1) && (sub_stmt == nil)), opEq, op), operands: append([]Operand{lhs}, rhs...)}
}
func AllTrue(conds ...Query) Query {
	return If((len(conds) == 0), nil, If((len(conds) == 1), conds[0], (Query)(&query{op: opAnd, conds: conds})))
}
func EitherOr(conds ...Query) Query {
	return If(len(conds) == 1, conds[0], (Query)(&query{op: opOr, conds: conds}))
}
func Not(cond Query) Query {
	switch q := cond.(*query); q.op {
	case opIn:
		return inNotIn(opNotIn, q.operands[0], q.operands[1:]...)
	case opNotIn:
		return inNotIn(opIn, q.operands[0], q.operands[1:]...)
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

type Operand interface {
	Eval(any, func(C) F) any
	Equal(other any) Query
	NotEqual(other any) Query
	LessThan(other any) Query
	GreaterThan(other any) Query
	LessOrEqual(other any) Query
	GreaterOrEqual(other any) Query
	Not() Query
	In(set ...any) Query
	NotIn(set ...any) Query
}

type query struct {
	op       string
	conds    []Query
	operands []Operand
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
	var do_arg func(operand Operand)
	do_arg = func(operand Operand) {
		if sub_stmt, _ := operand.(interface{ Sql(*str.Buf) }); sub_stmt != nil {
			sub_stmt.Sql(buf)
		} else if col_name, is := operand.(C); is {
			buf.WriteString(string(col_name))
		} else if fld_name, is := operand.(F); is {
			buf.WriteString(string(fld2col(fld_name)))
		} else if v, is := operand.(V); is {
			arg_name := "@A" + str.FromInt(len(args))
			args[arg_name[1:]] = v.Value
			buf.WriteString(arg_name)
		} else if fn, _ := operand.(*fun); fn != nil {
			buf.WriteString(string(fn.Fn))
			buf.WriteByte('(')
			for i, arg := range fn.Args {
				if i > 0 {
					buf.WriteByte(',')
				}
				do_arg(arg)
			}
			buf.WriteByte(')')
		} else {
			panic(operand)
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
			do_arg(operand)
		}
	}
	buf.WriteByte(')')
}

func (me *query) Eval(obj any, c2f func(C) F) Query {
	var failed Query
	switch me.op {
	case opAnd:
		_ = sl.All(me.conds, func(it Query) bool {
			maybe_failed := it.Eval(obj, c2f)
			failed = If[Query]((maybe_failed == nil), failed, maybe_failed)
			return (maybe_failed == nil)
		})
		return failed
	case opOr:
		any_true := sl.Any(me.conds, func(it Query) bool {
			maybe_failed := it.Eval(obj, c2f)
			failed = If[Query]((maybe_failed == nil), failed, maybe_failed)
			return (maybe_failed == nil)
		})
		if any_true {
			failed = nil
		}
		return failed
	case opNot:
		return If[Query]((me.conds[0].Eval(obj, c2f) == nil), me, nil)
	case opIn:
		in_set := sl.Has(sl.To(me.operands, func(it Operand) any { return it.Eval(obj, c2f) }), me.operands[0].Eval(obj, c2f))
		return If[Query](in_set, nil, me)
	case opNotIn:
		in_set := sl.Has(sl.To(me.operands, func(it Operand) any { return it.Eval(obj, c2f) }), me.operands[0].Eval(obj, c2f))
		return If[Query](in_set, me, nil)
	case opEq:
		eq := reflect.DeepEqual(me.operands[0].Eval(obj, c2f), me.operands[1].Eval(obj, c2f))
		return If[Query](eq, nil, me)
	case opNeq:
		eq := reflect.DeepEqual(me.operands[0].Eval(obj, c2f), me.operands[1].Eval(obj, c2f))
		return If[Query](eq, me, nil)
	case opGt:
		return If[Query](ReflGt(reflect.ValueOf(me.operands[0].Eval(obj, c2f)), reflect.ValueOf(me.operands[1].Eval(obj, c2f))), nil, me)
	case opGeq:
		return If[Query](ReflGe(reflect.ValueOf(me.operands[0].Eval(obj, c2f)), reflect.ValueOf(me.operands[1].Eval(obj, c2f))), nil, me)
	case opLt:
		return If[Query](ReflLt(reflect.ValueOf(me.operands[0].Eval(obj, c2f)), reflect.ValueOf(me.operands[1].Eval(obj, c2f))), nil, me)
	case opLeq:
		return If[Query](ReflLe(reflect.ValueOf(me.operands[0].Eval(obj, c2f)), reflect.ValueOf(me.operands[1].Eval(obj, c2f))), nil, me)
	default:
		panic(me.op)
	}
}
