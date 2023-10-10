package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/tracelog"

	. "yo/config"
	"yo/log"
	"yo/str"
)

var DB *sql.DB

func Init() {
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
		log.Println("DB connect: " + err.Error())
	}
}

type dbLogger struct{}

func (dbLogger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	log.Println("DB: %s %v", msg, data)
}

func NameFrom(s string) string {
	if s = str.Trim(s); s == "" || !str.IsPrtAscii(s) {
		panic("DB names should be ASCII, not '" + s + "'")
	}
	var buf str.Buf
	buf.Grow(len(s) + 2)
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
	return buf.String()
}
