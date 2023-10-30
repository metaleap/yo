package q

import (
	"reflect"

	. "yo/util"
	"yo/util/sl"
	"yo/util/str"

	"github.com/jackc/pgx/v5"
)

type Operator string

const (
	OpEq       Operator = " = "
	OpNeq      Operator = " != "
	OpLt       Operator = " < "
	OpLeq      Operator = " <= "
	OpGt       Operator = " > "
	OpGeq      Operator = " >= "
	OpIn       Operator = " IN "
	OpNotIn    Operator = " NOT IN "
	OpInArr    Operator = OpEq + opArrAny
	OpNotInArr Operator = OpNeq + opArrAll
	OpAnd      Operator = " AND "
	OpOr       Operator = " OR "
	OpNot      Operator = "NOT "
	opArrAll   Operator = " ALL" // note: must be same strlen as opArrAny
	opArrAny   Operator = " ANY" // note: must be same strlen as opArrAll
)

var opFlips = map[Operator]Operator{
	OpLt:  OpGeq,
	OpGt:  OpLeq,
	OpLeq: OpGt,
	OpGeq: OpLt,
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
func (me C) InArr(arr any) Query             { return InArr(me, arr) }
func (me C) NotInArr(arr any) Query          { return NotInArr(me, arr) }
func (me C) Asc() OrderBy                    { return &orderBy[C]{col: me} }
func (me C) Desc() OrderBy                   { return &orderBy[C]{col: me, desc: true} }
func (me C) Eval(obj any, c2f func(C) F) any { return c2f(me).Eval(obj, c2f) }

type F string

func (me F) F() F                           { return me }
func (me F) Equal(other any) Query          { return Equal(me, other) }
func (me F) NotEqual(other any) Query       { return NotEqual(me, other) }
func (me F) LessThan(other any) Query       { return LessThan(me, other) }
func (me F) GreaterThan(other any) Query    { return GreaterThan(me, other) }
func (me F) LessOrEqual(other any) Query    { return LessOrEqual(me, other) }
func (me F) GreaterOrEqual(other any) Query { return GreaterOrEqual(me, other) }
func (me F) Not() Query                     { return me.Equal(false) }
func (me F) In(set ...any) Query            { return In(me, set...) }
func (me F) NotIn(set ...any) Query         { return NotIn(me, set...) }
func (me F) InArr(arr any) Query            { return InArr(me, arr) }
func (me F) NotInArr(arr any) Query         { return NotInArr(me, arr) }
func (me F) Asc() OrderBy                   { return &orderBy[F]{fld: me} }
func (me F) Desc() OrderBy                  { return &orderBy[F]{fld: me, desc: true} }
func (me F) Eval(it any, c2f func(C) F) (ret any) {
	ret = ReflField(it, string(me)).Interface()
	if ref, is_ref := ret.(DbRef); is_ref {
		return ref.IdRaw()
	}
	return ret
}

type DbRef interface {
	IsDbRef() bool
	IdRaw() int64
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
func (me V) InArr(arr any) Query            { return InArr(me, arr) }
func (me V) NotInArr(arr any) Query         { return NotInArr(me, arr) }
func (me V) Eval(any, func(C) F) any        { return me.Value }

func That(value any) Operand { return V{value} }
func isNull(value any) bool {
	if v, is := value.(V); is {
		return (v.Value == nil)
	}
	return (value == nil)
}

type fn string

const (
	FnStrLen fn = "octet_length"
	FnArrLen fn = "array_length"
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
func (me *fun) InArr(arr any) Query            { return InArr(me, arr) }
func (me *fun) NotInArr(arr any) Query         { return NotInArr(me, arr) }
func (me *fun) Eval(obj any, c2f func(C) F) any {
	if me.Alt != nil {
		return me.Alt(sl.To(me.Args, func(it Operand) any { return it.Eval(obj, c2f) })...)
	}
	switch me.Fn {
	case FnArrLen:
		rv := reflect.ValueOf(me.Args[0].Eval(obj, c2f))
		return rv.Len()
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

func Equal(lhs any, rhs any) Query {
	return &query{op: OpEq, operands: operandsFrom(lhs, rhs)}
}
func NotEqual(lhs any, rhs any) Query {
	return &query{op: OpNeq, operands: operandsFrom(lhs, rhs)}
}
func LessThan(lhs any, rhs any) Query {
	return &query{op: OpLt, operands: operandsFrom(lhs, rhs)}
}
func LessOrEqual(lhs any, rhs any) Query {
	return &query{op: OpLeq, operands: operandsFrom(lhs, rhs)}
}
func GreaterThan(lhs any, rhs any) Query {
	return &query{op: OpGt, operands: operandsFrom(lhs, rhs)}
}
func GreaterOrEqual(lhs any, rhs any) Query {
	return &query{op: OpGeq, operands: operandsFrom(lhs, rhs)}
}
func In(lhs any, rhs ...any) Query { return inNotIn(OpIn, operandFrom(lhs), operandsFrom(rhs...)...) }
func NotIn(lhs any, rhs ...any) Query {
	return inNotIn(OpNotIn, operandFrom(lhs), operandsFrom(rhs...)...)
}
func inNotIn(op Operator, lhs Operand, rhs ...Operand) Query {
	if len(rhs) == 0 {
		panic(op + ": empty set")
	}
	sub_stmt, _ := rhs[0].(interface{ Sql(*str.Buf) })
	_, is_literal := rhs[0].(V)
	return &query{op: If(((len(rhs) == 1) && (sub_stmt == nil) && ((rhs[0] == nil) || is_literal)), OpEq, op), operands: append([]Operand{lhs}, rhs...)}
}
func InArr(lhs any, rhs any) Query {
	return &query{op: OpInArr, operands: operandsFrom(lhs, rhs)}
}
func NotInArr(lhs any, rhs any) Query {
	return &query{op: OpNotInArr, operands: operandsFrom(lhs, rhs)}
}
func ArrIsEmpty(arr any) Query {
	return operandFrom(arr).Equal(nil).Or(Fn(FnArrLen, arr).Equal(0))
}
func ArrAreAnyIn(arr any, operator Operator, arg any) Query {
	return &query{op: operator + opArrAny, operands: operandsFrom(arr, arg)}
}
func ArrAreAllIn(arr any, operator Operator, arg any) Query {
	return &query{op: operator + opArrAll, operands: operandsFrom(arr, arg)}
}
func AllTrue(conds ...Query) Query {
	if len(conds) == 0 {
		panic("q.AllTrue reached the no-conds situation, double-check call-site and prototyped q.AllTrue impl")
	}
	return If((len(conds) == 0), V{true}.Equal(true), If((len(conds) == 1), conds[0], (Query)(&query{op: OpAnd, conds: conds})))
}
func EitherOr(conds ...Query) Query {
	if len(conds) == 0 {
		panic("q.EitherOr reached the no-conds situation, double-check call-site and prototyped q.EitherOr impl")
	}
	return If((len(conds) == 0), V{true}.Equal(true), If(len(conds) == 1, conds[0], (Query)(&query{op: OpOr, conds: conds})))
}
func Not(cond Query) Query {
	switch q := cond.(*query); q.op {
	case OpIn:
		return inNotIn(OpNotIn, q.operands[0], q.operands[1:]...)
	case OpNotIn:
		return inNotIn(OpIn, q.operands[0], q.operands[1:]...)
	case OpEq:
		return NotEqual(q.operands[0], q.operands[1])
	case OpNeq:
		return Equal(q.operands[0], q.operands[1])
	case OpGt:
		return LessOrEqual(q.operands[0], q.operands[1])
	case OpLt:
		return GreaterOrEqual(q.operands[0], q.operands[1])
	case OpGeq:
		return LessThan(q.operands[0], q.operands[1])
	case OpLeq:
		return GreaterThan(q.operands[0], q.operands[1])
	case OpNot:
		return q.conds[0]
	}
	return &query{op: OpNot, conds: []Query{cond}}
}

func operandsFrom(it ...any) []Operand {
	return sl.To(it, operandFrom)
}

func operandFrom(it any) Operand {
	if operand, _ := it.(Operand); operand != nil {
		return operand
	}
	return V{Value: it}
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

type Field interface {
	~string
	Operand
	F() F
	Asc() OrderBy
	Desc() OrderBy
}

type field interface{ F() F }

type stmt interface {
	Sql(*str.Buf)
}

type query struct {
	op       Operator
	conds    []Query
	operands []Operand
}

func (me *query) AllDottedFs() map[F][]string {
	ret := map[F][]string{}
	for _, operand := range me.operands {
		if fld, is := operand.(field); is {
			if lhs, rhs, ok := str.Cut(string(fld.F()), "."); ok && (len(lhs) > 0) {
				ret[F(lhs)] = append(ret[F(lhs)], rhs)
			}
		}
	}
	for _, sub_query := range me.conds {
		for k, v := range sub_query.(*query).AllDottedFs() {
			ret[k] = append(ret[k], v...)
		}
	}
	return ret
}

func (me *query) And(conds ...Query) Query { return AllTrue(append([]Query{me}, conds...)...) }
func (me *query) Or(conds ...Query) Query  { return EitherOr(append([]Query{me}, conds...)...) }
func (me *query) Not() Query               { return Not(me) }

func (me *query) String(fld2col func(F) C, args pgx.NamedArgs) string {
	var buf str.Buf
	me.Sql(&buf, fld2col, args)
	return buf.String()
}

func (me *query) Sql(buf *str.Buf, fld2col func(F) C, args pgx.NamedArgs) {
	var do_arg func(operand Operand)
	do_arg = func(operand Operand) {
		if sub_stmt, _ := operand.(stmt); sub_stmt != nil {
			sub_stmt.Sql(buf)
		} else if col_name, is := operand.(C); is {
			buf.WriteString(string(col_name))
		} else if fld, is := operand.(field); is {
			buf.WriteString(string(fld2col(fld.F())))
		} else if v, is := operand.(V); is {
			if v.Value == true {
				buf.WriteString("(true::boolean)")
			} else if v.Value == false {
				buf.WriteString("(false::boolean)")
			} else {
				arg_name := "@A" + str.FromInt(len(args))
				args[arg_name[1:]] = v.Value
				buf.WriteString(arg_name)
				buf.WriteByte(' ')
			}
		} else if fn, _ := operand.(*fun); fn != nil {
			if fn.Fn == FnArrLen {
				fn.Args = append(fn.Args, operandFrom(1))
			}
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
			panic(str.Fmt("%T being %#v", operand, operand))
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
			cond.(*query).Sql(buf, fld2col, args)
			buf.WriteByte(')')
		}
	default:
		is_in_or_notin := (me.op == OpIn) || (me.op == OpNotIn)
		is_eq, is_ne, lnull, rnull := (me.op == OpEq), (me.op == OpNeq), isNull(me.operands[0]), isNull(me.operands[1])
		if (lnull || rnull) && (is_eq || is_ne) { // `IS NULL` not `= NULL`
			if lnull && rnull {
				buf.WriteString(If(is_eq, "true", "false"))
			} else if lnull {
				buf.WriteString(If(is_ne, " NULL IS NOT ", " NULL IS "))
				do_arg(me.operands[1])
			} else if rnull {
				do_arg(me.operands[0])
				buf.WriteString(If(is_ne, " IS NOT NULL ", " IS NULL "))
			}
		} else {
			is_arr_all, is_arr_any := str.Ends(string(me.op), string(opArrAll)), str.Ends(string(me.op), string(opArrAny))
			operator, is_arrish := me.op, (me.op == OpInArr) || (me.op == OpNotInArr) || is_arr_all || is_arr_any
			for i, operand := range me.operands {
				if i == 0 { // ensuring `IS NULL` instead of `= 0` even for non-NULL empty [] arrs, thanks sql...
					if fn, _ := operand.(*fun); (fn != nil) && (fn.Fn == FnArrLen) && (is_eq || is_ne) {
						if lit, is := me.operands[1].(V); is && lit.Value == 0 {
							do_arg(operand)
							buf.WriteString(If(is_ne, " IS NOT NULL", " IS NULL"))
							break
						}
					}
				}
				if i > 0 {
					if buf.WriteString(string(operator)); is_arrish {
						buf.WriteByte('(') // ANY and ALL rhs operand (the array expr) must be in parens
					} else if is_in_or_notin {
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
			if is_arrish {
				buf.WriteByte(')') // ANY and ALL rhs operand (the array expr) must be in parens
			}
		}
	}
	buf.WriteByte(')')
}

func SqlReprForDebugging(q Query) string {
	var buf str.Buf
	args := pgx.NamedArgs{}
	q.Sql(&buf, func(f F) C { return C(f) }, args)
	return buf.String() + "<<<<<<<<<" + str.From(args)
}

func (me *query) Eval(obj any, c2f func(C) F) (falseDueTo Query) {
	is_eq := func() bool {
		lhs := me.operands[0].Eval(obj, c2f)
		rhs := me.operands[1].Eval(obj, c2f)
		return reflect.DeepEqual(lhs, rhs) || // the below oddity catches convertible comparables such as string-vs-yodb.Text, int64-vs-yodb.I64 etc
			(ReflGe(reflect.ValueOf(lhs), reflect.ValueOf(rhs)) && ReflLe(reflect.ValueOf(lhs), reflect.ValueOf(rhs)))
	}
	switch me.op {
	case OpAnd:
		_ = sl.All(me.conds, func(it Query) bool {
			maybe_failed := it.Eval(obj, c2f)
			falseDueTo = If[Query]((maybe_failed == nil), falseDueTo, maybe_failed)
			return (maybe_failed == nil)
		})
		return
	case OpOr:
		any_true := sl.Any(me.conds, func(it Query) bool {
			maybe_failed := it.Eval(obj, c2f)
			falseDueTo = If[Query]((maybe_failed == nil), falseDueTo, maybe_failed)
			return (maybe_failed == nil)
		})
		if any_true {
			falseDueTo = nil
		}
		return
	case OpNot:
		return If[Query]((me.conds[0].Eval(obj, c2f) == nil), me, nil)
	case OpIn:
		in_set := sl.Has(me.operands[0].Eval(obj, c2f), sl.To(me.operands, func(it Operand) any { return it.Eval(obj, c2f) }))
		return If[Query](in_set, nil, me)
	case OpNotIn:
		in_set := sl.Has(me.operands[0].Eval(obj, c2f), sl.To(me.operands, func(it Operand) any { return it.Eval(obj, c2f) }))
		return If[Query](in_set, me, nil)
	case OpEq:
		return If[Query](is_eq(), nil, me)
	case OpNeq:
		return If[Query](is_eq(), me, nil)
	case OpGt:
		return If[Query](ReflGt(reflect.ValueOf(me.operands[0].Eval(obj, c2f)), reflect.ValueOf(me.operands[1].Eval(obj, c2f))), nil, me)
	case OpGeq:
		return If[Query](ReflGe(reflect.ValueOf(me.operands[0].Eval(obj, c2f)), reflect.ValueOf(me.operands[1].Eval(obj, c2f))), nil, me)
	case OpLt:
		return If[Query](ReflLt(reflect.ValueOf(me.operands[0].Eval(obj, c2f)), reflect.ValueOf(me.operands[1].Eval(obj, c2f))), nil, me)
	case OpLeq:
		return If[Query](ReflLe(reflect.ValueOf(me.operands[0].Eval(obj, c2f)), reflect.ValueOf(me.operands[1].Eval(obj, c2f))), nil, me)
	default:
		is_arr_all, is_arr_any := str.Ends(string(me.op), string(opArrAll)), str.Ends(string(me.op), string(opArrAny))
		if is_arr_all || is_arr_any {
			arr := me.operands[0].Eval(obj, c2f)
			refl_arr := reflect.ValueOf(arr)
			arg := me.operands[1].Eval(obj, c2f)
			operator := me.op[:len(me.op)-len(opArrAny)] // assumes same strlen for both opArrAny & opArrAll
			for i, arr_len := 0, refl_arr.Len(); i < arr_len; i++ {
				lhs, rhs := refl_arr.Index(i).Interface(), arg
				failed_with := (&query{op: operator, operands: []Operand{operandFrom(lhs), operandFrom(rhs)}}).Eval(obj, c2f)
				if (failed_with != nil) && is_arr_all {
					return failed_with
				} else if (failed_with == nil) && is_arr_any {
					return nil
				}
			}
			return If(is_arr_all, nil, me)
		}
		panic(me.op)
	}
}
