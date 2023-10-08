package yo

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
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
