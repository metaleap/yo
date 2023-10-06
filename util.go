package yo

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type Dict map[string]any

var (
	strRepl    = strings.NewReplacer
	strHas     = strings.Contains
	strBegins  = strings.HasPrefix
	strEnds    = strings.HasSuffix
	strTrim    = strings.TrimSpace
	strTrimL   = strings.TrimPrefix
	strTrimR   = strings.TrimSuffix
	iToA       = strconv.Itoa
	aToI       = strconv.Atoi
	i64ToStr   = strconv.FormatInt
	i64FromStr = strconv.ParseInt
	strFmt     = fmt.Sprintf
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
