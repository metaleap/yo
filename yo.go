package yo

import (
	"os"
)

var TraceItAll = IsDebugMode
var IsDebugMode = strHas(os.Args[0], "__debug_bin") || strHas(os.Args[0], "/go-build")

func Init() {
	cfgLoad()
	ListenAndServe()
}
