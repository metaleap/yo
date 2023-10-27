package str

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

type Buf = strings.Builder
type Dict = map[string]string

var (
	ReflType = reflect.TypeOf("")
	Has      = strings.Contains
	Begins   = strings.HasPrefix
	Ends     = strings.HasSuffix
	Trim     = strings.TrimSpace
	TrimL    = strings.TrimPrefix
	TrimR    = strings.TrimSuffix
	Idx      = strings.IndexByte
	IdxSub   = strings.Index
	IdxLast  = strings.LastIndexByte
	IdxRune  = strings.IndexRune
	Join     = strings.Join
	Split    = strings.Split
	Cut      = strings.Cut
	FromInt  = strconv.Itoa
	FromBool = strconv.FormatBool
	FromI64  = strconv.FormatInt
	ToInt    = strconv.Atoi
	ToI64    = strconv.ParseInt
	Fmt      = fmt.Sprintf
	Q        = strconv.Quote
	Lo       = strings.ToLower
	Up       = strings.ToUpper
)

func From(v any) string                    { return Fmt("%#v", v) }
func FromFloat(f float64, prec int) string { return strconv.FormatFloat(f, 'f', prec, 64) }
func Base36(i int) string                  { return FromI64(int64(i), 36) }

func Replace(s string, repl Dict) string {
	if len(repl) == 0 {
		return s
	}
	repl_old_new := make([]string, 0, len(repl)*2)
	for k, v := range repl {
		repl_old_new = append(repl_old_new, k, v)
	}
	return strings.NewReplacer(repl_old_new...).Replace(s)
}

func RePrefix(s string, oldPrefix string, newPrefix string) string {
	return newPrefix + TrimL(s, oldPrefix)
}

func ReSuffix(s string, oldSuffix string, newSuffix string) string {
	return TrimR(s, oldSuffix) + newSuffix
}

func DurationMs(nanos int64) string {
	ms := float64(nanos) * 0.000001
	return FromFloat(ms, 2) + "ms"
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

// ascii only
func Lo0(s string) string {
	if (s == "") || !((s[0] >= 'A') && (s[0] <= 'Z')) {
		return s
	}
	return Lo(s[:1]) + s[1:]
}

// ascii only
func Up0(s string) string {
	if (s == "") || ((s[0] >= 'A') && (s[0] <= 'Z')) {
		return s
	}
	return Up(s[:1]) + s[1:]
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

// whether `str` matches at least _@_._
func IsEmailishEnough(str string) bool {
	iat, idot := Idx(str, '@'), IdxLast(str, '.')
	return (len(str) >= 5) && (iat > 0) && (iat < len(str)-1) && (iat == IdxLast(str, '@') && (idot > iat) && (idot < len(str)-1))
}

func In(str string, set ...string) bool {
	for i := range set {
		if set[i] == str {
			return true
		}
	}
	return false
}

func Repl(str string, namedReplacements Dict) string {
	if (len(namedReplacements) == 0) || (len(str) == 0) {
		return str
	}
	new_len := len(str)
	for k, v := range namedReplacements {
		new_len -= (len(k) + 2)
		new_len += len(v)
	}
	var buf Buf
	if new_len > 0 {
		buf.Grow(new_len)
	}

	var skip_until, accum_from int
	var accum string
	for i := 0; i < len(str); i++ {
		if skip_until > i {
			continue
		} else if str[i] == '{' {
			if idx := i + Idx(str[i:], '}'); idx > i {
				name := str[i+1 : idx]
				if repl, exists := namedReplacements[name]; exists {
					_, _ = buf.WriteString(accum)
					_, _ = buf.WriteString(repl)
					skip_until = idx + 1
					accum_from, accum = skip_until, ""
					continue
				}
			}
		}
		accum = str[accum_from : i+1]
	}
	_, _ = buf.WriteString(accum)
	return buf.String()
}
