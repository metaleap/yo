package yodb

import (
	. "yo/ctx"
	q "yo/db/query"
	yoserve "yo/server"
	. "yo/util"
	"yo/util/sl"
)

func init() {
	yoserve.API["__/db/listTables"] = yoserve.Method(apiListTables)
}

func apiListTables(ctx *Ctx, args *struct {
	Name string
}, ret *Return[map[Text][]*TableColumn]) any {
	ret.Result = ListTables(ctx, args.Name)
	return ret
}

func registerApiHandlers[T any](desc *structDesc) {
	type_name := desc.ty.Name()
	yoserve.API["__/db/"+type_name+"/getById"] = yoserve.Method(apiGetById[T])
	yoserve.API["__/db/"+type_name+"/createOne"] = yoserve.Method(apiCreateOne[T])
	yoserve.API["__/db/"+type_name+"/createMany"] = yoserve.Method(apiCreateMany[T])
	yoserve.API["__/db/"+type_name+"/deleteOne"] = yoserve.Method(apiDeleteOne[T])
	yoserve.API["__/db/"+type_name+"/count"] = yoserve.Method(apiCount[T])
}

type argId struct{ ID I64 }
type retCount struct{ Count int64 }

func apiGetById[T any](ctx *Ctx, args *argId, ret *T) any {
	if it := Get[T](ctx, args.ID); it != nil {
		*ret = *it
		return ret
	}
	return nil
}

func apiCount[T any](ctx *Ctx, args *ApiQueryExpr[T], ret *retCount) any {
	if args != nil {
		args.validate()
	}
	ret.Count = Count[T](ctx, args.Query(), 0, "", nil)
	return ret
}

func apiCreateOne[T any](ctx *Ctx, args *T, ret *struct {
	ID int64
}) any {
	id := CreateOne[T](ctx, args)
	ret.ID = int64(id)
	return ret
}

func apiCreateMany[T any](ctx *Ctx, args *struct {
	Items []*T
}, ret *Void) any {
	CreateMany[T](ctx, args.Items...)
	return ret
}

func apiDeleteOne[T any](ctx *Ctx, args *argId, ret *retCount) any {
	ret.Count = Delete[T](ctx, ColID.Equal(args.ID))
	return ret
}

type ApiQueryVal[T any] struct {
	Fld  *q.F
	Str  *string
	Bool *bool
	I64  *int64
	F64  *float64
}
type ApiQueryExpr[T any] struct {
	AND []ApiQueryExpr[T]
	OR  []ApiQueryExpr[T]
	NOT *ApiQueryExpr[T]
	EQ  []ApiQueryVal[T]
	NEQ []ApiQueryVal[T]
	LT  []ApiQueryVal[T]
	LE  []ApiQueryVal[T]
	GT  []ApiQueryVal[T]
	GE  []ApiQueryVal[T]
	IN  []ApiQueryVal[T]
}

func (me *ApiQueryExpr[T]) validate() {
	// binary operators
	for name, bin_op := range map[string][]ApiQueryVal[T]{"EQ": me.EQ, "NEQ": me.NEQ, "LT": me.LT, "LE": me.LE, "GT": me.GT, "GE": me.GE} {
		if (len(bin_op) != 0) && (len(bin_op) != 2) {
			panic("expected 2 operands for operator '" + name + "'")
		}
		for i := range bin_op {
			bin_op[i].validate()
		}
	}
	// IN operator
	if len(me.IN) == 1 {
		panic("expected rhs operand for 'IN' operator")
	}
	for i := range me.IN {
		me.IN[i].validate()
	}
	for i := range me.AND {
		me.AND[i].validate()
	}
	for i := range me.OR {
		me.OR[i].validate()
	}
	if me.NOT != nil {
		me.NOT.validate()
	}
}

func (me *ApiQueryExpr[T]) Query() q.Query {
	switch {
	case len(me.AND) >= 2:
		return q.AllTrue(sl.Map(me.AND, func(it ApiQueryExpr[T]) q.Query { return it.Query() })...)
	case len(me.OR) >= 2:
		return q.EitherOr(sl.Map(me.OR, func(it ApiQueryExpr[T]) q.Query { return it.Query() })...)
	case me.NOT != nil:
		return q.Not(me.NOT.Query())
	case len(me.IN) >= 2:
		return q.In(me.IN[0].Val(), sl.Map(me.IN[1:], func(it ApiQueryVal[T]) any { return it.Val() }))
	}
	return nil
}

func (me *ApiQueryVal[T]) validate() {
	num_set := 0
	if me.Fld != nil {
		num_set++
	}
	if me.Str != nil {
		num_set++
	}
	if me.Bool != nil {
		num_set++
	}
	if me.I64 != nil {
		num_set++
	}
	if me.F64 != nil {
		num_set++
	}
	if num_set > 1 {
		panic("expected only one of '.Fld', '.Str', '.Bool', '.I64', '.F64' to be set, but found multiple")
	}
}

func (me *ApiQueryVal[T]) Val() any {
	switch {
	case me.Fld != nil:
		return *me.Fld
	case me.Str != nil:
		return *me.Str
	case me.Bool != nil:
		return *me.Bool
	case me.I64 != nil:
		return *me.I64
	case me.F64 != nil:
		return *me.F64
	}
	return nil
}
