package util

import (
	"cmp"
	"strings"
	"yo/util/str"
)

type (
	None                     struct{}
	Return[T any]            struct{ Result T }
	Pair[TLhs any, TRhs any] struct {
		Key TLhs
		It  TRhs
	}
)

func Assert(alwaysTrue bool, show any) {
	if IsDevMode {
		if !alwaysTrue {
			var err any = "unreachable"
			if show != nil {
				if show_fn, _ := show.(func() any); show_fn != nil {
					err = show_fn()
				} else {
					err = show
				}
			}
			panic(str.FmtV(err))
		}
	}
}

func Never[T any](alwaysFalse bool, show func() any) (ret T) {
	Assert(!alwaysFalse, show)
	return
}

func If[T any](b bool, t T, f T) T {
	if b {
		return t
	}
	return f
}

func IfF[T any](b bool, t func() T, f func() T) T {
	if b {
		return t()
	}
	return f()
}

func Clamp[T cmp.Ordered](min T, max T, v T) T {
	return If(v < min, min, If(v > max, max, v))
}

func Min[T cmp.Ordered](values ...T) (ret T) {
	ret = values[0]
	for _, value := range values[1:] {
		if value < ret {
			ret = value
		}
	}
	return
}

func Max[T cmp.Ordered](values ...T) (ret T) {
	ret = values[0]
	for _, value := range values[1:] {
		if value > ret {
			ret = value
		}
	}
	return
}

func ToPtr[T any](v T) *T { return &v }

func Either[T comparable](try ...T) T {
	var none T
	for i := range try {
		if try[i] != none {
			return try[i]
		}
	}
	return none
}

func ToIdent(s string) string { return ToIdentWith(s, '_') }

func ToIdentWith(s string, replaceChar byte) string {
	s = strings.TrimSpace(s)
	var buf strings.Builder
	buf.Grow(len(s))
	next_up := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if is_ident := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'); is_ident {
			if next_up && (c >= 'a' && c <= 'z') {
				c, next_up = c-32, false
			}
			buf.WriteByte(c)
		} else if replaceChar != 0 {
			buf.WriteByte(replaceChar)
		} else {
			next_up = true
		}
	}
	return buf.String()
}
