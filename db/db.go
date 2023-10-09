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
