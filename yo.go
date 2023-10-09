package yo

import (
	"time"

	"yo/config"
	"yo/context"
	"yo/db"
	"yo/log"

	. "yo/util"
)

type Ctx = context.Ctx

func init() {
	time.Local = time.UTC
}

func Init() {
	time.Local = time.UTC // between above `init` and now, `time` might have its own `init`-time ideas about setting `time.Local`...
	log.Println("Load config...")
	config.Load()
	db.Connect()
	log.Println("API init...")
	apiInit()
	log.Println("API SDK gen...")
	if IsDevMode {
		apiGenSdk()
	}
	log.Println("`ListenAndServe`-ready!")
}
