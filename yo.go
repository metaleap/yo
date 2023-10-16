package yo

import (
	"time"

	yodb "yo/db"
	_ "yo/feat_auth" // because feat_auth it has its own admin-only endpoints: so api-related codegen (using api-refl) must know about them regardless of actual app using feat_auth or not
	yolog "yo/log"
	yosrv "yo/srv"
)

func init() {
	time.Local = time.UTC
}

func Init() (listenAndServe func()) {
	time.Local = time.UTC // between above `init` and now, `time` might have its own `init`-time ideas about setting `time.Local`...
	yolog.Println("DB config...")
	db_structs := yodb.InitAndConnectAndMigrateAndMaybeCodegen()
	yolog.Println("API init...")
	listenAndServe = yosrv.InitAndMaybeCodegen(db_structs)
	yolog.Println("yo.Init done")
	return
}
