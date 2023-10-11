package yodb

import (
	. "yo/ctx"
	yoserve "yo/server"
	. "yo/util"
)

func init() {
	yoserve.API["__/db/listTables"] = yoserve.Method(apiListTables)
}

type Return[T any] struct {
	Result T
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
}

func apiGetById[T any](ctx *Ctx, args *struct {
	ID I64
}, ret *T) any {
	if it := Get[T](ctx, args.ID); it != nil {
		*ret = *it
		return ret
	}
	return nil
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
