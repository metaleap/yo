package db

import (
	. "yo/ctx"
	"yo/server"
	. "yo/util"
)

func init() {
	server.API["__/db/listTables"] = server.Method(apiListTables)
}

func apiListTables(ctx *Ctx, in *Void, out *struct {
	Tables map[string][]table_column
}) {
	out.Tables = ListTables(ctx)
}
