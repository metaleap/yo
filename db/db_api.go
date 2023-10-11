package yodb

import (
	. "yo/ctx"
	yoserve "yo/server"
	. "yo/util"
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

func apiCount[T any](ctx *Ctx, args *Void, ret *retCount) any {
	ret.Count = Count[T](ctx, nil, 0, "", nil)
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
	ret.Count = Delete[T](ctx, ColID.Equals(args.ID))
	return ret
}
