package log

import (
	"os"
	"sync"
	"time"

	"yo/str"
)

var mut sync.Mutex

func Println(msg string, args ...any) {
	now := time.Now()
	mut.Lock()
	defer mut.Unlock()
	if msg = now.Format("15:04:05") + msg + "\n"; len(args) == 0 {
		_, _ = os.Stderr.WriteString(msg)
	} else {
		_, _ = os.Stderr.WriteString(str.Fmt(msg, args...))
	}
}
