package yolog

import (
	"os"
	"sync"
	"time"

	"yo/util/str"
)

var mut sync.Mutex

func Println(msg string, args ...any) {
	now, buf := time.Now(), str.Buf{}
	if msg != "" {
		buf.Grow(len(msg) + 10 + (8 * len(args)))
		buf.WriteString(now.Format("15:04:05"))
		buf.WriteString("  ")
	}
	if len(args) == 0 {
		buf.WriteString(msg)
	} else {
		buf.WriteString(str.Fmt(msg, args...))
	}
	buf.WriteString("\n")
	mut.Lock()
	defer mut.Unlock()
	_, _ = os.Stderr.WriteString(buf.String())
}

func PrintLnLn(msg string, args ...any) {
	Println("")
	Println(msg, args...)
}
