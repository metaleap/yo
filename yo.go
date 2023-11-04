package yo

import (
	"io/fs"
	"os/exec"
	"time"

	. "yo/cfg"
	yoctx "yo/ctx"
	yodb "yo/db"
	yoauth "yo/feat_auth"
	_ "yo/jobs"
	yojobs "yo/jobs"
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
	time.Local = time.UTC // repeat of init() above because who knows what happened in between (well, so far, we do. but still =)
	yosrv.StaticFileDirApp, yosrv.StaticFileDirYo =
		staticFileDirApp, staticFileDirYo

	yolog.PrintLnLn("DB init...")
	db_structs := yodb.InitAndConnectAndMigrateAndMaybeCodegen()
	{
		ctx := yoctx.NewCtxNonHttp(time.Minute, false, "")
		defer ctx.OnDone(nil)
		yodb.Upsert[yojobs.JobDef](ctx, &yoauth.UserPwdReqJobDef)
	}

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
