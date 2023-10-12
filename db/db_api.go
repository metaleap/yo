package yodb

import (
	"reflect"

	. "yo/ctx"
	q "yo/db/query"
	yoserve "yo/server"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
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

func registerApiHandlers[TObj any, TFld ~string](desc *structDesc) {
	type_name := desc.ty.Name()
	yoserve.API["__/db/"+type_name+"/findById"] = yoserve.Method(apiFindById[TObj, TFld])
	yoserve.API["__/db/"+type_name+"/findOne"] = yoserve.Method(apiFindOne[TObj, TFld])
	yoserve.API["__/db/"+type_name+"/createOne"] = yoserve.Method(apiCreateOne[TObj, TFld])
	yoserve.API["__/db/"+type_name+"/createMany"] = yoserve.Method(apiCreateMany[TObj, TFld])
	yoserve.API["__/db/"+type_name+"/deleteOne"] = yoserve.Method(apiDeleteOne[TObj, TFld])
	yoserve.API["__/db/"+type_name+"/deleteMany"] = yoserve.Method(apiDeleteMany[TObj, TFld])
	yoserve.API["__/db/"+type_name+"/count"] = yoserve.Method(apiCount[TObj, TFld])
}

type retCount struct{ Count int64 }
type argId struct{ ID I64 }
type argQuery[TObj any, TFld ~string] struct {
	Query *ApiQueryExpr[TObj, TFld]
}
type argFind[TObj any, TFld ~string] struct {
	Query   *ApiQueryExpr[TObj, TFld]
	OrderBy []*ApiOrderBy[TObj, TFld]
}

func (me *argQuery[TObj, TFld]) toDbQ() q.Query {
	if me != nil && me.Query != nil {
		me.Query.Validate()
		return me.Query.toDbQ()
	}
	return nil
}

func apiFindById[TObj any, TFld ~string](ctx *Ctx, args *argId, ret *TObj) any {
	if it := ById[TObj](ctx, args.ID); it != nil {
		*ret = *it
		return ret
	}
	return nil
}

func apiFindOne[TObj any, TFld ~string](ctx *Ctx, args *argFind[TObj, TFld], ret *Return[*TObj]) any {
	ret.Result = FindOne[TObj](ctx, args.Query.toDbQ())
	return ret
}

func apiCount[TObj any, TFld ~string](ctx *Ctx, args *argQuery[TObj, TFld], ret *retCount) any {
	ret.Count = Count[TObj](ctx, args.toDbQ(), 0, "", nil)
	return ret
}

func apiCreateOne[TObj any, TFld ~string](ctx *Ctx, args *TObj, ret *struct {
	ID int64
}) any {
	id := CreateOne[TObj](ctx, args)
	ret.ID = int64(id)
	return ret
}

func apiCreateMany[TObj any, TFld ~string](ctx *Ctx, args *struct {
	Items []*TObj
}, ret *Void) any {
	CreateMany[TObj](ctx, args.Items...)
	return ret
}

func apiDeleteOne[TObj any, TFld ~string](ctx *Ctx, args *argId, ret *retCount) any {
	ret.Count = Delete[TObj](ctx, ColID.Equal(args.ID))
	return ret
}

func apiDeleteMany[TObj any, TFld ~string](ctx *Ctx, args *argQuery[TObj, TFld], ret *retCount) any {
	query := args.toDbQ()
	if query == nil {
		panic(Err("QueryRequiredButMissing"))
	}
	ret.Count = Delete[TObj](ctx, query)
	return ret
}

type ApiOrderBy[TObj any, TFld ~string] struct {
	Fld  TFld
	Desc bool
}
type ApiQueryVal[TObj any, TFld ~string] struct {
	Fld  *TFld
	Str  *string
	Bool *bool
	I64  *int64
	F64  *float64
}
type ApiQueryExpr[TObj any, TFld ~string] struct {
	AND []ApiQueryExpr[TObj, TFld]
	OR  []ApiQueryExpr[TObj, TFld]
	NOT *ApiQueryExpr[TObj, TFld]
	EQ  []ApiQueryVal[TObj, TFld]
	NE  []ApiQueryVal[TObj, TFld]
	LT  []ApiQueryVal[TObj, TFld]
	LE  []ApiQueryVal[TObj, TFld]
	GT  []ApiQueryVal[TObj, TFld]
	GE  []ApiQueryVal[TObj, TFld]
	IN  []ApiQueryVal[TObj, TFld]
}

func (me *ApiQueryExpr[TObj, TFld]) Validate() {
	num_found := 0
	// binary operators
	for name, bin_op := range map[string][]ApiQueryVal[TObj, TFld]{"EQ": me.EQ, "NE": me.NE, "LT": me.LT, "LE": me.LE, "GT": me.GT, "GE": me.GE} {
		if (len(bin_op) != 0) && (len(bin_op) != 2) {
			panic(Err(name + "_TwoOperandsRequired"))
		}
		for i := range bin_op {
			bin_op[i].Validate()
		}
		num_found++
	}
	// the others
	for _, l := range []int{len(me.IN), len(me.AND), len(me.OR)} {
		if l > 0 {
			num_found++
		}
	}
	if len(me.IN) == 1 {
		panic("IN_SetOperandRequired")
	}
	for i := range me.IN {
		me.IN[i].Validate()
	}
	for _, it := range [][]ApiQueryExpr[TObj, TFld]{me.AND, me.OR} {
		for i := range it {
			it[i].Validate()
		}
	}
	if me.NOT != nil {
		num_found++
		me.NOT.Validate()
	}
}

func (me *ApiQueryExpr[TObj, TFld]) toDbQ() q.Query {
	switch {
	case len(me.AND) >= 2:
		return q.AllTrue(sl.Map(me.AND, func(it ApiQueryExpr[TObj, TFld]) q.Query { return it.toDbQ() })...)
	case len(me.OR) >= 2:
		return q.EitherOr(sl.Map(me.OR, func(it ApiQueryExpr[TObj, TFld]) q.Query { return it.toDbQ() })...)
	case me.NOT != nil:
		return q.Not(me.NOT.toDbQ())
	case len(me.IN) >= 2:
		return q.In(me.IN[0].val(), sl.Map(me.IN[1:], func(it ApiQueryVal[TObj, TFld]) any { return it.val() }))
	}
	for bin_op, q_f := range map[*[]ApiQueryVal[TObj, TFld]]func(any, any) q.Query{&me.EQ: q.Equal, &me.NE: q.NotEqual, &me.LT: q.LessThan, &me.LE: q.LessOrEqual, &me.GT: q.GreaterThan, &me.GE: q.GreaterOrEqual} {
		if bin_op := *bin_op; len(bin_op) == 2 {
			return q_f(bin_op[0].val(), bin_op[1].val())
		}
	}
	return nil
}

func (me *ApiQueryVal[TObj, TFld]) Validate() {
	num_set, r_v := 0, reflect.ValueOf
	for _, rv := range []reflect.Value{r_v(me.Fld), r_v(me.Str), r_v(me.Bool), r_v(me.I64), r_v(me.F64)} {
		if !rv.IsNil() {
			num_set++
		}
	}
	if num_set > 1 {
		panic("ExpectedOneOrNoneOf" + str.Join([]string{"Fld", "Str", "Bool", "I64", "F64"}, "Or") + "ButGot" + str.FromInt(num_set))
	}
}

func (me *ApiQueryVal[TObj, TFld]) val() any {
	switch {
	case me.Fld != nil:
		return q.F(*me.Fld)
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
