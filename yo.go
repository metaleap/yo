package yo

import (
	"io/fs"
	"os/exec"
	"time"

	. "yo/cfg"
	yodb "yo/db"
	_ "yo/feat_auth"
	_ "yo/jobs"
	yolog "yo/log"
	yosrv "yo/srv"
	. "yo/util"
	"yo/util/str"
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
	if IsDevMode {
		go runBrowser()
	}
	return
}

func runBrowser() {
	port := str.FromInt(Cfg.YO_API_HTTP_PORT)
	cmd := exec.Command("wbd",
		// "about:"+port,
		"http://localhost:"+port+"?"+str.FromI64(time.Now().UnixNano(), 36),
		// "http://localhost:"+port+"/__yostatic/yo.html?"+str.FromI64(time.Now().UnixNano(), 36),
	)
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	if err := cmd.Wait(); err != nil {
		panic(err)
	}
}
