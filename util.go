package yo

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

type Dict map[string]any
type kv map[string]string

var (
	strHas     = strings.Contains
	strBegins  = strings.HasPrefix
	strEnds    = strings.HasSuffix
	strTrim    = strings.TrimSpace
	strTrimL   = strings.TrimPrefix
	strTrimR   = strings.TrimSuffix
	strIdx     = strings.IndexByte
	strJoin    = strings.Join
	strSplit   = strings.Split
	strCut     = strings.Cut
	iToA       = strconv.Itoa
	aToI       = strconv.Atoi
	i64ToStr   = strconv.FormatInt
	i64FromStr = strconv.ParseInt
	strFmt     = fmt.Sprintf
	strQ       = strconv.Quote
)

func If[T any](b bool, t T, f T) T {
	if b {
		return t
	}
	return f
}

func keys[K comparable, V any](m map[K]V) (ret []K) {
	ret = make([]K, 0, len(m))
	for k := range m {
		ret = append(ret, k)
	}
	return
}

func sorted[S ~[]E, E cmp.Ordered](slice S) S {
	slices.Sort(slice)
	return slice
}

func reflHasMethod(ty reflect.Type, name string) bool {
	for ty.Kind() == reflect.Pointer {
		ty = ty.Elem()
	}
	_, ok := ty.MethodByName(name)
	if !ok {
		_, ok = reflect.PointerTo(ty).MethodByName(name)
	}
	return ok
}

func strReplace(s string, repl map[string]string) string {
	repl_old_new := make([]string, 0, len(repl)*2)
	for k, v := range repl {
		repl_old_new = append(repl_old_new, k, v)
	}
	return strings.NewReplacer(repl_old_new...).Replace(s)
}

func strIsLo(s string) bool {
	for _, r := range s {
		if !unicode.IsLower(r) {
			return false
		}
	}
	return true
}

func strIsUp(s string) bool {
	for _, r := range s {
		if strBegins(s, "Ä") {
			println(s, r, unicode.IsUpper(r))
		}
		if !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func strIsPrtAscii(s string) bool {
	for i := 0; i < len(s); i++ {
		if (s[i] < 0x20) || (s[i] > 0x7e) {
			return false
		}
	}
	return true
}

func strSub(s string, runeIdx int, runesLen int) string {
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

func toIdent(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if is_ident := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'); !is_ident {
			buf.WriteByte('_')
		} else {
			buf.WriteByte(c)
		}
	}
	return buf.String()
}
