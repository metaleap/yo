package yodb

import (
	"reflect"

	yoctx "yo/ctx"
	q "yo/db/query"
	. "yo/srv"
	. "yo/util"
	"yo/util/sl"
)

const (
	errBinOpPrefix = "ExpectedTwoOperandsFor"

	ErrSetQuery    = "Query"
	ErrSetDbUpdate = "DbUpdate"
	ErrSetDbDelete = "DbDelete"
)

func init() {
	KnownErrSets[ErrSetDbDelete] = []Err{"ExpectedQueryForDelete"}
	KnownErrSets[ErrSetDbUpdate] = []Err{"ExpectedChangesForUpdate", "ExpectedQueryForUpdate"}
	KnownErrSets[ErrSetQuery] = append([]Err{
		Err("ExpectedOnlyEitherQueryOrQueryFromButNotBoth"),
		Err("ExpectedSetOperandFor" + opIn),
		Err("ExpectedOneOrNoneButNotMultipleOfFldOrStrOrBoolOrInt"),
	}, sl.To([]string{opAnd, opOr, opNot, opIn, opEq, opNe, opGt, opGe, opLt, opLe}, func(it string) Err {
		return Err(errBinOpPrefix + it)
	})...)
	Apis(ApiMethods{
		"__/yo/db/getTable": api(apiGetTable),
	})
}

func apiGetTable(this *ApiCtx[struct {
	Name string
}, Return[[]*TableColumn]]) {
	this.Ret.Result = GetTable(this.Ctx, this.Args.Name)
}

func apiMethodPath(typeName string, relMethodPath string) string {
	return "__/yo/db/" + typeName + "/" + relMethodPath
}

func registerApiHandlers[TObj any, TFld q.Field](desc *structDesc) {
	type_name := desc.ty.Name()

	Apis(ApiMethods{
		apiMethodPath(type_name, "findById"): api(apiFindById[TObj, TFld]),
		apiMethodPath(type_name, "findOne"): api(apiFindOne[TObj, TFld]).
			CouldFailWith(":" + ErrSetQuery),
		apiMethodPath(type_name, "findMany"): api(apiFindMany[TObj, TFld]).
			CouldFailWith(":" + ErrSetQuery),
		apiMethodPath(type_name, "deleteOne"): api(apiDeleteOne[TObj, TFld]).
			CouldFailWith(":" + ErrSetDbDelete),
		apiMethodPath(type_name, "deleteMany"): api(apiDeleteMany[TObj, TFld]).
			CouldFailWith(":"+ErrSetQuery, ":"+ErrSetDbDelete),
		apiMethodPath(type_name, "updateOne"): api(apiUpdateOne[TObj, TFld]).
			CouldFailWith(":"+ErrSetDbUpdate, yoctx.ErrDbNotStored, yoctx.ErrDbUpdExpectedIdGt0),
		apiMethodPath(type_name, "updateMany"): api(apiUpdateMany[TObj, TFld]).
			CouldFailWith(":"+ErrSetQuery, ":"+ErrSetDbUpdate),
		apiMethodPath(type_name, "count"): api(apiCount[TObj, TFld]).
			CouldFailWith(":" + ErrSetQuery),
		apiMethodPath(type_name, "createOne"):  api(apiCreateOne[TObj, TFld]),
		apiMethodPath(type_name, "createMany"): api(apiCreateMany[TObj, TFld]),
	})
}

type retCount struct{ Count int64 }
type ApiArgId struct{ Id I64 }
type argQuery[TObj any, TFld q.Field] struct {
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
		panic(ErrQuery_ExpectedOnlyEitherQueryOrQueryFromButNotBoth)
	}
	if me.QueryFrom != nil {
		return Q[TObj](me.QueryFrom)
	}
	me.Query.Validate()
	return me.Query.toDbQ()
}
func (me *argQuery[TObj, TFld]) toDbO() []q.OrderBy {
	return sl.To(me.OrderBy, func(it *ApiOrderBy[TObj, TFld]) q.OrderBy {
		fld := q.F(it.Fld)
		return If(it.Desc, fld.Desc(), fld.Asc())
	})
}

func apiFindById[TObj any, TFld q.Field](this *ApiCtx[ApiArgId, TObj]) {
	this.Ret = ById[TObj](this.Ctx, this.Args.Id)
}

func apiFindOne[TObj any, TFld q.Field](this *ApiCtx[argQuery[TObj, TFld], Return[*TObj]]) {
	this.Ret.Result = FindOne[TObj](this.Ctx, this.Args.toDbQ(), this.Args.toDbO()...)
}

func apiFindMany[TObj any, TFld q.Field](this *ApiCtx[argQuery[TObj, TFld], Return[[]*TObj]]) {
	this.Ret.Result = FindMany[TObj](this.Ctx, this.Args.toDbQ(), int(this.Args.Max), this.Args.toDbO()...)
}

func apiCount[TObj any, TFld q.Field](this *ApiCtx[argQuery[TObj, TFld], retCount]) {
	this.Ret.Count = Count[TObj](this.Ctx, this.Args.toDbQ(), "", nil)
}

func apiCreateOne[TObj any, TFld q.Field](this *ApiCtx[TObj, struct {
	ID int64
}]) {
	id := CreateOne[TObj](this.Ctx, this.Args)
	this.Ret.ID = int64(id)
}

func apiCreateMany[TObj any, TFld q.Field](this *ApiCtx[struct {
	Items []TObj
}, Void]) {
	CreateMany[TObj](this.Ctx, sl.Ptrs(this.Args.Items)...)
}

func apiDeleteOne[TObj any, TFld q.Field](this *ApiCtx[ApiArgId, retCount]) {
	this.Ret.Count = Delete[TObj](this.Ctx, ColID.Equal(this.Args.Id))
}

func apiDeleteMany[TObj any, TFld q.Field](this *ApiCtx[argQuery[TObj, TFld], retCount]) {
	this.Ret.Count = Delete[TObj](this.Ctx, this.Args.toDbQ())
}

type ApiUpdateArgs[TObj any] struct {
	ApiArgId
	Changes                       TObj
	IncludingEmptyOrMissingFields bool
}

func apiUpdateOne[TObj any, TFld q.Field](this *ApiCtx[ApiUpdateArgs[TObj], retCount]) {
	if this.Args.Id <= 0 {
		panic(Err(yoctx.ErrDbUpdExpectedIdGt0))
	}
	this.Ret.Count = Update[TObj](this.Ctx, &this.Args.Changes, ColID.Equal(this.Args.Id), !this.Args.IncludingEmptyOrMissingFields)
}

func apiUpdateMany[TObj any, TFld q.Field](this *ApiCtx[struct {
	argQuery[TObj, TFld]
	Changes                       TObj
	IncludingEmptyOrMissingFields bool
}, retCount]) {
	this.Ret.Count = Update[TObj](this.Ctx, &this.Args.Changes, this.Args.toDbQ(), !this.Args.IncludingEmptyOrMissingFields)
}

type ApiOrderBy[TObj any, TFld q.Field] struct {
	Fld  TFld
	Desc bool
}
type ApiQueryVal[TObj any, TFld q.Field] struct {
	Fld  *TFld
	Str  *string
	Bool *bool
	Int  *int64
}
type ApiQueryExpr[TObj any, TFld q.Field] struct {
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

const ( // must be in sync with the `ApiQueryExpr` field names
	opAnd = "AND"
	opOr  = "OR"
	opNot = "NOT"
	opIn  = "IN"
	opEq  = "EQ"
	opNe  = "NE"
	opLt  = "LT"
	opLe  = "LE"
	opGt  = "GT"
	opGe  = "GE"
)

func (me *ApiQueryExpr[TObj, TFld]) Validate() {
	num_found := 0
	// binary operators
	for name, bin_op := range map[string][]ApiQueryVal[TObj, TFld]{opEq: me.EQ, opNe: me.NE, opLt: me.LT, opLe: me.LE, opGt: me.GT, opGe: me.GE} {
		if (len(bin_op) != 0) && (len(bin_op) != 2) {
			panic(Err(ErrSetQuery + "_" + errBinOpPrefix + name))
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
		panic(ErrQuery_ExpectedSetOperandForIN)
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
		return q.AllTrue(sl.To(me.AND, func(it ApiQueryExpr[TObj, TFld]) q.Query { return it.toDbQ() })...)
	case len(me.OR) >= 2:
		return q.EitherOr(sl.To(me.OR, func(it ApiQueryExpr[TObj, TFld]) q.Query { return it.toDbQ() })...)
	case me.NOT != nil:
		return q.Not(me.NOT.toDbQ())
	case len(me.IN) >= 2:
		return q.In(me.IN[0].val(), sl.To(me.IN[1:], func(it ApiQueryVal[TObj, TFld]) any { return it.val() })...)
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
		panic(ErrQuery_ExpectedOneOrNoneButNotMultipleOfFldOrStrOrBoolOrInt)
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
