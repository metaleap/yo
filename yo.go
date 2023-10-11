package yo

import (
	"embed"
	"time"

	config "yo/cfg"
	yoctx "yo/ctx"
	"yo/db"
	yolog "yo/log"
	yoserve "yo/server"
)

//go:embed __yostatic
var staticFileDir embed.FS

type APIMethods = yoserve.APIMethods
type Ctx = yoctx.Ctx

func init() {
	time.Local = time.UTC
}

func Init(apiMethods yoserve.APIMethods) (listenAndServe func()) {
	time.Local = time.UTC // between above `init` and now, `time` might have its own `init`-time ideas about setting `time.Local`...
	if apiMethods != nil {
		for k, v := range apiMethods {
			yoserve.API[k] = v
		}
	}
	yolog.Println("Load config...")
	config.Load()
	db.Init()
	yolog.Println("API init...")
	var apiGenSdk func()
	apiGenSdk, listenAndServe = yoserve.Init(&staticFileDir)
	yolog.Println("API SDK gen...")
	if apiGenSdk != nil {
		apiGenSdk()
	}
	yolog.Println("yo.Init done")
	return
}

func InOut[TIn any, TOut any](f func(*Ctx, *TIn, *TOut) any) yoserve.APIMethod {
	return yoserve.Method(f)
}
