package yodb

import (
	"context"
	"database/sql"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/tracelog"

	. "yo/cfg"
	yolog "yo/log"
	"yo/util/str"
)

var (
	inited = false
	DB     *sql.DB
)

func Init() (dbStructs []reflect.Type) {
	if inited {
		panic("db.Init called twice?")
	}
	conn_cfg, err := pgx.ParseConfig(Cfg.DATABASE_URL)
	if err != nil {
		panic(err)
	}
	conn_cfg.ConnectTimeout = Cfg.DB_REQ_TIMEOUT
	conn_cfg.Tracer = &tracelog.TraceLog{
		LogLevel: tracelog.LogLevelError,
		Logger:   dbLogger{},
	}
	str_conn := stdlib.RegisterConnConfig(conn_cfg)
	for DB, err = sql.Open("pgx", str_conn); err != nil; time.Sleep(time.Second) {
		yolog.Println("DB connect: " + err.Error())
	}
	doEnsureDbStructTables()
	for _, desc := range ensureDescs {
		dbStructs = append(dbStructs, desc.ty)
	}
	inited = true
	return
}

type dbLogger struct{}

func (dbLogger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	yolog.Println("dbPgx: %s %v", msg, data)
}

func NameFrom(s string) string {
	if s = str.Trim(s); s == "" || !str.IsPrtAscii(s) {
		panic("DB names should be ASCII, not '" + s + "'")
	}
	var buf str.Buf
	buf.Grow(len(s) + 2)
	if str.IsUp(s) {
		s = str.Lo(s)
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				buf.WriteByte('_')
			}
			buf.WriteByte(c + 32)
		} else if ok := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z'); !ok {
			buf.WriteByte('_')
		} else {
			buf.WriteByte(c)
		}
	}
	buf.WriteByte('_')
	return buf.String()
}
