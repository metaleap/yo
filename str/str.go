package str

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	Has     = strings.Contains
	Begins  = strings.HasPrefix
	Ends    = strings.HasSuffix
	Trim    = strings.TrimSpace
	TrimL   = strings.TrimPrefix
	TrimR   = strings.TrimSuffix
	Idx     = strings.IndexByte
	Join    = strings.Join
	Split   = strings.Split
	Cut     = strings.Cut
	FromInt = strconv.Itoa
	ToInt   = strconv.Atoi
	FromI64 = strconv.FormatInt
	ToI64   = strconv.ParseInt
	Fmt     = fmt.Sprintf
	Q       = strconv.Quote
)

func Replace(s string, repl map[string]string) string {
	repl_old_new := make([]string, 0, len(repl)*2)
	for k, v := range repl {
		repl_old_new = append(repl_old_new, k, v)
	}
	return strings.NewReplacer(repl_old_new...).Replace(s)
}

func ReSuffix(s string, oldSuffix string, newSuffix string) string {
	return TrimR(s, oldSuffix) + newSuffix
}

func DurationMs(nanos int64) string {
	return strconv.FormatInt(time.Duration(nanos).Milliseconds(), 10) + "ms"
}

func IsLo(s string) bool {
	for _, r := range s {
		if !unicode.IsLower(r) {
			return false
		}
	}
	return true
}

func IsUp(s string) bool {
	for _, r := range s {
		if Begins(s, "Ä") {
			println(s, r, unicode.IsUpper(r))
		}
		if !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func IsPrtAscii(s string) bool {
	for i := 0; i < len(s); i++ {
		if (s[i] < 0x20) || (s[i] > 0x7e) {
			return false
		}
	}
	return true
}

func Sub(s string, runeIdx int, runesLen int) string {
	if s == "" || runesLen == 0 {
		return ""
	}
	n, idxStart, idxEnd := 0, -1, -1
	for i := range s { // iter by runes
		if n == runeIdx {
			if idxStart = i; runesLen < 0 {
				break
			}
		} else if (idxStart >= 0) && ((n - idxStart) == runesLen) {
			idxEnd = i
			break
		}
		n++
	}
	if idxStart < 0 {
		return ""
	} else if runesLen < 0 {
		return s[idxStart:]
	}
	return s[idxStart:idxEnd]
}
