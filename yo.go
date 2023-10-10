package yo

import (
	"embed"
	"time"

	"yo/config"
	"yo/ctx"
	"yo/db"
	"yo/log"
	"yo/server"
)

//go:embed __yostatic
var staticFileDir embed.FS

type APIMethods = server.APIMethods
type Ctx = ctx.Ctx

func init() {
	time.Local = time.UTC
}

func Init(apiMethods server.APIMethods) (listenAndServe func()) {
	time.Local = time.UTC // between above `init` and now, `time` might have its own `init`-time ideas about setting `time.Local`...
	if apiMethods != nil {
		for k, v := range apiMethods {
			server.API[k] = v
		}
	}
	log.Println("Load config...")
	config.Load()
	db.Init()
	log.Println("API init...")
	var apiGenSdk func()
	apiGenSdk, listenAndServe = server.Init(&staticFileDir)
	log.Println("API SDK gen...")
	if apiGenSdk != nil {
		apiGenSdk()
	}
	log.Println("`ListenAndServe`-ready!")
	return
}

func InOut[TIn any, TOut any](f func(*Ctx, *TIn, *TOut)) server.APIMethod {
	return server.Method(f)
}
