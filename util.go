package yo

import (
	"fmt"
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
