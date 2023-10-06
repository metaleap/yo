package yo

import (
	"os"
)

var IsDebugMode = strHas(os.Args[0], "__debug_bin") || strHas(os.Args[0], "/go-build")

func Init() {
	cfgLoad()
	panic(cfg)
}
