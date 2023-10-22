package yo

import (
	"io/fs"
	"time"

	yodb "yo/db"
	_ "yo/feat_auth" // because feat_auth it has its own admin-only endpoints: so api-related codegen (using api-refl) must know about them regardless of actual app using feat_auth or not
	yolog "yo/log"
	yosrv "yo/srv"
)

var ts2jsAppSideStaticDir func()

func init() {
	time.Local = time.UTC
}

func Init(staticFileDirApp fs.FS, staticFileDirYo fs.FS) (listenAndServe func()) {
	yosrv.StaticFileDirApp, yosrv.StaticFileDirYo =
		staticFileDirApp, staticFileDirYo
	time.Local = time.UTC // between above `init` and now, `time` might have its own `init`-time ideas about setting `time.Local`...
	yolog.PrintLnLn("DB init...")
	db_structs := yodb.InitAndConnectAndMigrateAndMaybeCodegen()
	yolog.PrintLnLn("API init...")
	listenAndServe = yosrv.InitAndMaybeCodegen(db_structs)
	if ts2jsAppSideStaticDir != nil { // set only in dev-mode
		ts2jsAppSideStaticDir()
	}
	yolog.PrintLnLn("yo.Init done")
	return
}
