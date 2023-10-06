package yo

import (
	"strconv"
	"strings"
)

type Dict map[string]any

var (
	strRepl   = strings.NewReplacer
	strHas    = strings.Contains
	strBegins = strings.HasPrefix
	strEnds   = strings.HasSuffix
	strTrim   = strings.TrimSpace
	strTrimL  = strings.TrimPrefix
	strTrimR  = strings.TrimSuffix
	iToA      = strconv.Itoa
	aToI      = strconv.Atoi
)

func If[T any](b bool, t T, f T) T {
	if b {
		return t
	}
	return f
}
