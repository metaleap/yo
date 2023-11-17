package yo

import (
	"io/fs"
	"os"
	"os/exec"
	"time"

	yoauth "yo/auth"
	. "yo/cfg"
	yoctx "yo/ctx"
	yodb "yo/db"
	_ "yo/jobs"
	yojobs "yo/jobs"
	yolog "yo/log"
	yomail "yo/mail"
	yosrv "yo/srv"
	. "yo/util"
	"yo/util/str"
)

var ts2jsInAppSideStaticDir func()
var AppPkgPath = "yo/"
var buildDeployablyNow = func() {}

func init() {
	time.Local = time.UTC
}

func Init(staticFileDirYo fs.FS, staticFileDirApp fs.FS) (listenAndServe func()) {
	time.Local = time.UTC // repeat of init() above because who knows what happened in between (well, so far, we do. but still =)
	AppPkgPath = AppPkgPath[:str.Idx(AppPkgPath, '/')]
	yosrv.StaticFileDir_App, yosrv.StaticFileDir_Yo =
		staticFileDirApp, staticFileDirYo

	yolog.PrintLnLn("DB init...")
	if !IsDevMode {
		time.Sleep(4 * time.Second)
	}
	yodb.Ensure[ErrEntry, ErrEntryField]("", nil, false)

	db_structs := yodb.InitAndConnectAndMigrateAndMaybeCodegen()

	yolog.PrintLnLn("API init...")
	yoauth.Init()
	listenAndServe = yosrv.InitAndMaybeCodegen(db_structs)
	if ts2jsInAppSideStaticDir != nil { // set only in dev-mode
		ts2jsInAppSideStaticDir()
	}

	if os.Getenv("YO_BUILD") != "" {
		buildDeployablyNow()
		os.Exit(0)
	}

	yolog.PrintLnLn("Jobs init...")
	{
		ctx := yoctx.NewCtxNonHttp(yojobs.Timeout1Min, false, "")
		defer ctx.OnDone(nil)
		yodb.Upsert[yojobs.JobDef](ctx, &yoauth.UserPwdReqJobDef)
		yodb.Upsert[yojobs.JobDef](ctx, &yomail.MailReqJobDef)
		yodb.Upsert[yojobs.JobDef](ctx, &errJobDef)
		yojobs.Init(ctx) // some db clean-ups in there, doesn't `Engine.Resume` though, that's below

		listen_and_serve := listenAndServe
		listenAndServe = func() {
			go yojobs.Default.Resume()
			listen_and_serve()
		}
	}

	yolog.PrintLnLn("yo.Init done")
	go runBrowser()
	return
}

func runBrowser() {
	if IsDevMode && (os.Getenv("NO_WB") == "") {
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
}
