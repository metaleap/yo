package yo

import (
	"time"

	config "yo/cfg"
	yoctx "yo/ctx"
	yodb "yo/db"
	yolog "yo/log"
	yoserve "yo/server"
)

type ApiMethods = yoserve.ApiMethods
type Ctx = yoctx.Ctx

func init() {
	time.Local = time.UTC
}

func Init(apiMethods yoserve.ApiMethods) (listenAndServe func()) {
	time.Local = time.UTC // between above `init` and now, `time` might have its own `init`-time ideas about setting `time.Local`...
	yoserve.Api.Add(apiMethods)
	yolog.Println("Load config...")
	config.Load()
	db_structs := yodb.Init()
	yolog.Println("API init...")
	var apiGenSdk func()
	apiGenSdk, listenAndServe = yoserve.Init(db_structs)
	yolog.Println("API SDK gen...")
	if apiGenSdk != nil {
		apiGenSdk()
	}
	yolog.Println("yo.Init done")
	return
}

func ApiOf[TIn any, TOut any](f func(*Ctx, *TIn, *TOut) any) yoserve.ApiMethod {
	return yoserve.Method(f)
}
