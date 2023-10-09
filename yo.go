package yo

import (
	"log"
	"time"

	"yo/context"
	"yo/json"
)

type Ctx = context.Ctx
type Void = json.Void

func init() {
	time.Local = time.UTC
}

func Init() {
	time.Local = time.UTC // between above `init` and now, `time` might have its own `init`-time ideas about setting `time.Local`...
	log.Println("Load config...")
	cfgLoad()
	log.Println("API init...")
	apiInit()
	log.Println("API SDK gen...")
	if IsDevMode {
		apiGenSdk()
	}
	log.Println("`ListenAndServe`-ready!")
}
