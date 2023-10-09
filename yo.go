package yo

import (
	"time"

	"yo/api"
	"yo/config"
	"yo/context"
	"yo/db"
	"yo/log"
)

type Ctx = context.Ctx

func init() {
	time.Local = time.UTC
}

func Init(apiMethods api.Methods) {
	if apiMethods != nil {
		for k, v := range apiMethods {
			api.API[k] = v
		}
	}
	time.Local = time.UTC // between above `init` and now, `time` might have its own `init`-time ideas about setting `time.Local`...
	log.Println("Load config...")
	config.Load()
	db.Connect()
	log.Println("API init...")
	var apiGenSdk func()
	apiGenSdk, apiHandle = api.Init()
	log.Println("API SDK gen...")
	if apiGenSdk != nil {
		apiGenSdk()
	}
	log.Println("`ListenAndServe`-ready!")
}
