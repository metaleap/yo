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
	yoserve.Api["__/db/listTables"] = yoserve.Method(apiListTables)
}

func apiListTables(ctx *Ctx, args *struct {
	Name string
}, ret *Return[map[Text][]*TableColumn]) any {
	ret.Result = ListTables(ctx, args.Name)
	return ret
}

func registerApiHandlers[TObj any, TFld ~string](desc *structDesc) {
	type_name := desc.ty.Name()
	yoserve.Api.Add(yoserve.ApiMethods{
		"__/db/" + type_name + "/findById":   yoserve.Method(apiFindById[TObj, TFld]),
		"__/db/" + type_name + "/findOne":    yoserve.Method(apiFindOne[TObj, TFld]),
		"__/db/" + type_name + "/findMany":   yoserve.Method(apiFindMany[TObj, TFld]),
		"__/db/" + type_name + "/createOne":  yoserve.Method(apiCreateOne[TObj, TFld]),
		"__/db/" + type_name + "/createMany": yoserve.Method(apiCreateMany[TObj, TFld]),
		"__/db/" + type_name + "/deleteOne":  yoserve.Method(apiDeleteOne[TObj, TFld]),
		"__/db/" + type_name + "/deleteMany": yoserve.Method(apiDeleteMany[TObj, TFld]),
		"__/db/" + type_name + "/updateOne":  yoserve.Method(apiUpdateOne[TObj, TFld]),
		"__/db/" + type_name + "/updateMany": yoserve.Method(apiUpdateMany[TObj, TFld]),
		"__/db/" + type_name + "/count":      yoserve.Method(apiCount[TObj, TFld]),
	})
}

type retCount struct{ Count int64 }
type argId struct{ Id I64 }
type argQuery[TObj any, TFld ~string] struct {
	Query     *ApiQueryExpr[TObj, TFld]
	QueryFrom *TObj
	OrderBy   []*ApiOrderBy[TObj, TFld]
	Max       uint32
}

func (me *argQuery[TObj, TFld]) toDbQ() q.Query {
	if (me.QueryFrom == nil) && (me.Query == nil) {
		return nil
	}
	if (me.QueryFrom != nil) && (me.Query != nil) {
		panic(Err("ExpectedOnlyEitherQueryOrQueryFromButNotBoth"))
	}
	if me.QueryFrom != nil {
		return Q[TObj](me.QueryFrom)
	}
	me.Query.Validate()
	return me.Query.toDbQ()
}
func (me *argQuery[TObj, TFld]) toDbO() []q.OrderBy {
	return sl.Conv(me.OrderBy, func(it *ApiOrderBy[TObj, TFld]) q.OrderBy {
		fld := q.F(it.Fld)
		return If(it.Desc, fld.Desc(), fld.Asc())
	})
}

func apiFindById[TObj any, TFld ~string](ctx *Ctx, args *argId, ret *TObj) any {
	if it := ById[TObj](ctx, args.Id); it != nil {
		*ret = *it
		return ret
	}
	return nil
}

func apiFindOne[TObj any, TFld ~string](ctx *Ctx, args *argQuery[TObj, TFld], ret *Return[*TObj]) any {
	ret.Result = FindOne[TObj](ctx, args.toDbQ(), args.toDbO()...)
	return ret
}

func apiFindMany[TObj any, TFld ~string](ctx *Ctx, args *argQuery[TObj, TFld], ret *Return[[]*TObj]) any {
	ret.Result = FindMany[TObj](ctx, args.toDbQ(), int(args.Max), args.toDbO()...)
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
	ret.Count = Delete[TObj](ctx, ColID.Equal(args.Id))
	return ret
}

func apiDeleteMany[TObj any, TFld ~string](ctx *Ctx, args *argQuery[TObj, TFld], ret *retCount) any {
	ret.Count = Delete[TObj](ctx, args.toDbQ())
	return ret
}

func apiUpdateOne[TObj any, TFld ~string](ctx *Ctx, args *struct {
	argId
	Changes                       *TObj
	IncludingEmptyOrMissingFields bool
}, ret *retCount) any {
	if args.Id <= 0 {
		panic(Err("ExpectedIdGreater0ButGot" + str.FromInt(int(args.Id))))
	}
	ret.Count = Update[TObj](ctx, args.Changes, args.IncludingEmptyOrMissingFields, ColID.Equal(args.Id))
	return ret
}

func apiUpdateMany[TObj any, TFld ~string](ctx *Ctx, args *struct {
	argQuery[TObj, TFld]
	Changes                       *TObj
	IncludingEmptyOrMissingFields bool
}, ret *retCount) any {
	ret.Count = Update[TObj](ctx, args.Changes, args.IncludingEmptyOrMissingFields, args.toDbQ())
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
	Int  *int64
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
		return q.AllTrue(sl.Conv(me.AND, func(it ApiQueryExpr[TObj, TFld]) q.Query { return it.toDbQ() })...)
	case len(me.OR) >= 2:
		return q.EitherOr(sl.Conv(me.OR, func(it ApiQueryExpr[TObj, TFld]) q.Query { return it.toDbQ() })...)
	case me.NOT != nil:
		return q.Not(me.NOT.toDbQ())
	case len(me.IN) >= 2:
		return q.In(me.IN[0].val(), sl.Conv(me.IN[1:], func(it ApiQueryVal[TObj, TFld]) any { return it.val() }))
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
	for _, rv := range []reflect.Value{r_v(me.Fld), r_v(me.Str), r_v(me.Bool), r_v(me.Int)} {
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
	case me.Int != nil:
		return *me.Int
	}
	return nil
}
