package db

import (
	. "yo/ctx"
	"yo/server"
)

func init() {
	server.API["__/db/listTables"] = server.Method(apiListTables)
}

func apiListTables(ctx *Ctx, in *struct {
	Name string
}, out *struct {
	Tables map[Text][]*TableColumn
}) {
	out.Tables = ListTables(ctx, in.Name)
}
