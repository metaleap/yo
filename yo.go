package yo

import (
	"log"
	"os"
)

var TraceItAll = IsDebugMode
var IsDebugMode = strHas(os.Args[0], "__debug_bin") || strHas(os.Args[0], "/go-build")

func Init() {
	log.Println("Load config...")
	cfgLoad()
	log.Println("API init...")
	apiInit()
	log.Println("API SDK gen...")
	if IsDebugMode {
		apiGenSdk()
	}
	log.Println("`ListenAndServe`-ready!")
}
