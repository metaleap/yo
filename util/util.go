package util

import (
	"strings"
)

type Void struct{}

func If[T any](b bool, t T, f T) T {
	if b {
		return t
	}
	return f
}

func ToIdent(s string) string {
	s = strings.TrimSpace(s)
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
