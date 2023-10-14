package yodb

import (
	"reflect"

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
	KnownErrSets[ErrSetQuery] = append([]Err{
		Err("ExpectedOnlyEitherQueryOrQueryFromButNotBoth"),
		Err("ExpectedSetOperandFor" + opIn),
		Err("ExpectedOneOrNoneButNotMultipleOfFldOrStrOrBoolOrInt"),
	}, sl.Conv([]string{opAnd, opOr, opNot, opIn, opEq, opNe, opGt, opGe, opLt, opLe}, func(it string) Err {
		return Err(errBinOpPrefix + it)
	})...)
	KnownErrSets[ErrSetDbDelete] = []Err{"ExpectedQueryForDelete"}
	KnownErrSets[ErrSetDbUpdate] = []Err{"ExpectedChangesForUpdate", "ExpectedQueryForUpdate"}
	Apis(ApiMethods{
		"__/db/listTables": Api(apiListTables, PkgInfo),
	})
}

func apiListTables(this *ApiCtx[struct {
	Name string
}, Return[map[Text][]*TableColumn]]) {
	this.Ret.Result = ListTables(this.Ctx, this.Args.Name)
}

func registerApiHandlers[TObj any, TFld ~string](desc *structDesc) {
	type_name := desc.ty.Name()

	Apis(ApiMethods{
		"__/db/" + type_name + "/findById":   Api(apiFindById[TObj, TFld], PkgInfo),
		"__/db/" + type_name + "/findOne":    Api(apiFindOne[TObj, TFld], PkgInfo, ":"+ErrSetQuery),
		"__/db/" + type_name + "/findMany":   Api(apiFindMany[TObj, TFld], PkgInfo, ":"+ErrSetQuery),
		"__/db/" + type_name + "/createOne":  Api(apiCreateOne[TObj, TFld], PkgInfo),
		"__/db/" + type_name + "/createMany": Api(apiCreateMany[TObj, TFld], PkgInfo),
		"__/db/" + type_name + "/deleteOne":  Api(apiDeleteOne[TObj, TFld], PkgInfo, ":"+ErrSetDbDelete),
		"__/db/" + type_name + "/deleteMany": Api(apiDeleteMany[TObj, TFld], PkgInfo, ":"+ErrSetQuery, ":"+ErrSetDbDelete),
		"__/db/" + type_name + "/updateOne":  Api(apiUpdateOne[TObj, TFld], PkgInfo, ":"+ErrSetDbUpdate, "ExpectedIdGreater0"),
		"__/db/" + type_name + "/updateMany": Api(apiUpdateMany[TObj, TFld], PkgInfo, ":"+ErrSetQuery, ":"+ErrSetDbUpdate),
		"__/db/" + type_name + "/count":      Api(apiCount[TObj, TFld], PkgInfo, ":"+ErrSetQuery),
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
		panic(ErrQuery_ExpectedOnlyEitherQueryOrQueryFromButNotBoth)
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

func apiFindById[TObj any, TFld ~string](this *ApiCtx[argId, TObj]) {
	this.Ret = ById[TObj](this.Ctx, this.Args.Id)
}

func apiFindOne[TObj any, TFld ~string](this *ApiCtx[argQuery[TObj, TFld], Return[*TObj]]) {
	this.Ret.Result = FindOne[TObj](this.Ctx, this.Args.toDbQ(), this.Args.toDbO()...)
}

func apiFindMany[TObj any, TFld ~string](this *ApiCtx[argQuery[TObj, TFld], Return[[]*TObj]]) {
	this.Ret.Result = FindMany[TObj](this.Ctx, this.Args.toDbQ(), int(this.Args.Max), this.Args.toDbO()...)
}

func apiCount[TObj any, TFld ~string](this *ApiCtx[argQuery[TObj, TFld], retCount]) {
	this.Ret.Count = Count[TObj](this.Ctx, this.Args.toDbQ(), 0, "", nil)
}

func apiCreateOne[TObj any, TFld ~string](this *ApiCtx[TObj, struct {
	ID int64
}]) {
	id := CreateOne[TObj](this.Ctx, this.Args)
	this.Ret.ID = int64(id)
}

func apiCreateMany[TObj any, TFld ~string](this *ApiCtx[struct {
	Items []*TObj
}, Void]) {
	CreateMany[TObj](this.Ctx, this.Args.Items...)
}

func apiDeleteOne[TObj any, TFld ~string](this *ApiCtx[argId, retCount]) {
	this.Ret.Count = Delete[TObj](this.Ctx, ColID.Equal(q.Lit(this.Args.Id)))
}

func apiDeleteMany[TObj any, TFld ~string](this *ApiCtx[argQuery[TObj, TFld], retCount]) {
	this.Ret.Count = Delete[TObj](this.Ctx, this.Args.toDbQ())
}

func apiUpdateOne[TObj any, TFld ~string](this *ApiCtx[struct {
	argId
	Changes                       *TObj
	IncludingEmptyOrMissingFields bool
}, retCount]) {
	if this.Args.Id <= 0 {
		panic(Err___db_UserAccount_updateOne_ExpectedIdGreater0)
	}
	this.Ret.Count = Update[TObj](this.Ctx, this.Args.Changes, this.Args.IncludingEmptyOrMissingFields, ColID.Equal(q.Lit(this.Args.Id)))
}

func apiUpdateMany[TObj any, TFld ~string](this *ApiCtx[struct {
	argQuery[TObj, TFld]
	Changes                       *TObj
	IncludingEmptyOrMissingFields bool
}, retCount]) {
	this.Ret.Count = Update[TObj](this.Ctx, this.Args.Changes, this.Args.IncludingEmptyOrMissingFields, this.Args.toDbQ())
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
		return q.AllTrue(sl.Conv(me.AND, func(it ApiQueryExpr[TObj, TFld]) q.Query { return it.toDbQ() })...)
	case len(me.OR) >= 2:
		return q.EitherOr(sl.Conv(me.OR, func(it ApiQueryExpr[TObj, TFld]) q.Query { return it.toDbQ() })...)
	case me.NOT != nil:
		return q.Not(me.NOT.toDbQ())
	case len(me.IN) >= 2:
		return q.In(q.Lit(me.IN[0].val()), sl.Conv(me.IN[1:], func(it ApiQueryVal[TObj, TFld]) q.Operand { return q.Lit(it.val()) })...)
	}
	for bin_op, q_f := range map[*[]ApiQueryVal[TObj, TFld]]func(q.Operand, q.Operand) q.Query{&me.EQ: q.Equal, &me.NE: q.NotEqual, &me.LT: q.LessThan, &me.LE: q.LessOrEqual, &me.GT: q.GreaterThan, &me.GE: q.GreaterOrEqual} {
		if bin_op := *bin_op; len(bin_op) == 2 {
			return q_f(q.Lit(bin_op[0].val()), q.Lit(bin_op[1].val()))
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
