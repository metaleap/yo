package db

import (
	. "yo/ctx"
	yoserve "yo/server"
)

func init() {
	yoserve.API["__/db/listTables"] = yoserve.Method(apiListTables)
}

func apiListTables(ctx *Ctx, args *struct {
	Name string
}, ret *struct {
	Tables map[Text][]*TableColumn
}) any {
	ret.Tables = ListTables(ctx, args.Name)
	return ret
}

func registerApiHandlers[T any](desc *structDesc) {
	// type_name := desc.ty.Name()
	// yoserve.API["__/db/"+type_name+"/getById"] = nil
}

func apiGetById[T any](ctx *Ctx, args *struct{ ID int }, ret *T) {
	// desc := desc[T]()
	// stmt := new(Stmt).Select(desc.cols...).From(desc.tableName).
	// 	Where("id = @id").Limit(1)
	// _ = stmt
}
