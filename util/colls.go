package util

import (
	"cmp"
	"slices"
)

type Dict map[string]any
type KVs map[string]string

func Keys[K comparable, V any](m map[K]V) (ret []K) {
	ret = make([]K, 0, len(m))
	for k := range m {
		ret = append(ret, k)
	}
	return
}

func Sorted[S ~[]E, E cmp.Ordered](slice S) S {
	slices.Sort(slice)
	return slice
}
