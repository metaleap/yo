package sl

import (
	"cmp"
	"reflect"
	"slices"
)

func WithoutIdx[TSlice ~[]TItem, TItem any](sansIdx int, slice TSlice, noMake bool) (ret TSlice) {
	if (sansIdx < 0) || (sansIdx >= len(slice)) {
		return slice
	}
	if noMake {
		return append(slice[:sansIdx], slice[sansIdx+1:]...)
	}
	ret = make(TSlice, 0, len(slice)-1)
	for i := range slice {
		if i != sansIdx {
			ret = append(ret, slice[i])
		}
	}
	return
}

func WithoutIdxs[TSlice ~[]TItem, TItem any](sansIdxs []int, slice TSlice) (ret TSlice) {
	if len(sansIdxs) == 0 {
		return slice
	}
	ret = make(TSlice, 0, len(slice)-len(sansIdxs))
	for i := range slice {
		if !Has(i, sansIdxs) {
			ret = append(ret, slice[i])
		}
	}
	return
}

func WithoutIdxRange[TSlice ~[]TItem, TItem any](delFromIdx int, delUntilIdx int, slice TSlice) TSlice {
	if (delFromIdx <= 0) && ((delUntilIdx < 0) || (delUntilIdx >= len(slice))) {
		return TSlice{}
	}
	return append(append(make(TSlice, 0, len(slice)-(delUntilIdx-delFromIdx)), slice[:delFromIdx]...), slice[delUntilIdx:]...)
}

func Sorted[TSlice ~[]TItem, TItem cmp.Ordered](slice TSlice) TSlice {
	slices.Sort(slice)
	return slice
}
func SortedPer[TSlice ~[]TItem, TItem any](slice TSlice, cmp func(TItem, TItem) int) TSlice {
	slices.SortStableFunc(slice, cmp)
	return slice
}

func IdxOf[TSlice ~[]TItem, TItem comparable](v TItem, s TSlice) int {
	for i := range s {
		if v == s[i] {
			return i
		}
	}
	return -1
}

func IdxWhere[TSlice ~[]TItem, TItem any](slice TSlice, pred func(TItem) bool) int {
	for i := range slice {
		if pred(slice[i]) {
			return i
		}
	}
	return -1
}

func HasWhere[TSlice ~[]TItem, TItem any](slice TSlice, pred func(TItem) bool) bool {
	return (IdxWhere(slice, pred) >= 0)
}

func Has[TSlice ~[]TItem, TItem comparable](needle TItem, slice TSlice) bool {
	for i := range slice {
		if slice[i] == needle {
			return true
		}
	}
	return false
}

func HasAnyOf[TSlice ~[]TItem, TItem comparable](slice TSlice, of ...TItem) bool {
	if len(of) == 0 {
		return true
	} else if len(of) == 1 {
		return Has(of[0], slice)
	}
	for i := range slice {
		for j := range of {
			if slice[i] == of[j] {
				return true
			}
		}
	}
	return false
}

func HasAllOf[TSlice ~[]TItem, TItem comparable](slice TSlice, of ...TItem) bool {
	if len(of) == 0 {
		return true
	} else if len(of) == 1 {
		return Has(of[0], slice)
	}
	have := make([]bool, len(of))
	for i := range slice {
		for j := range of {
			if (!have[j]) && slice[i] == of[j] {
				have[j] = true
				break
			}
		}
	}
	for i := range have {
		if !have[i] {
			return false
		}
	}
	return true
}

func To[TSlice ~[]TItem, TItem any, TOut any](slice TSlice, f func(TItem) TOut) (ret []TOut) {
	ret = make([]TOut, len(slice))
	for i := range slice {
		ret[i] = f(slice[i])
	}
	return
}

func All[TSlice ~[]TItem, TItem any](slice TSlice, pred func(TItem) bool) bool {
	for i := range slice {
		if !pred(slice[i]) {
			return false
		}
	}
	return true
}

func Any[TSlice ~[]TItem, TItem any](slice TSlice, pred func(TItem) bool) bool {
	for i := range slice {
		if pred(slice[i]) {
			return true
		}
	}
	return false
}

func WithoutDupls[TSlice ~[]TItem, TItem comparable](slice TSlice) TSlice {
	return With(make(TSlice, 0, len(slice)), slice...)
}

func Without[TSlice ~[]TItem, TItem comparable](slice TSlice, without ...TItem) TSlice {
	if len(without) == 0 {
		return slice
	}
	return Where(slice, func(item TItem) bool {
		return !Has(item, without)
	})
}

func Where[TSlice ~[]TItem, TItem any](slice TSlice, pred func(TItem) bool) (ret TSlice) {
	ret = make(TSlice, 0, len(slice))
	for i := range slice {
		if pred(slice[i]) {
			ret = append(ret, slice[i])
		}
	}
	return
}

// add only those `items` not yet in `slice`. avoids the need for a `Unique(TSlice)TSlice`
func With[TSlice ~[]TItem, TItem comparable](slice TSlice, items ...TItem) TSlice {
	append_from := 0
	for i := range items {
		if IdxOf(items[i], slice) < 0 {
			slice = append(slice, items[append_from:i+1]...)
		}
		append_from = i + 1
	}
	return append(slice, items[append_from:]...)
}

func Ptrs[TSlice ~[]TItem, TItem any](slice TSlice) (ret []*TItem) {
	ret = make([]*TItem, len(slice))
	for i := range slice {
		ret[i] = &slice[i]
	}
	return
}

type Slice[T any] []T

func Of[T any](items ...T) Slice[T] {
	return items
}

func (me Slice[T]) Any(pred func(T) bool) bool       { return Any(me, pred) }
func (me Slice[T]) All(pred func(T) bool) bool       { return All(me, pred) }
func (me Slice[T]) IdxWhere(pred func(T) bool) int   { return IdxWhere(me, pred) }
func (me Slice[T]) Where(pred func(T) bool) Slice[T] { return Where(me, pred) }
func (me Slice[T]) Without(pred func(T) bool) Slice[T] {
	return Where(me, func(it T) bool { return !pred(it) })
}

func (me Slice[T]) ToAnys() (ret []any) {
	ret = make([]any, len(me))
	for i := range me {
		ret[i] = me[i]
	}
	return
}

func (me *Slice[T]) EnsureAllUnique(areEqual func(T, T) bool) {
	if areEqual == nil {
		areEqual = func(lhs T, rhs T) bool { return reflect.DeepEqual(reflect.ValueOf(lhs), reflect.ValueOf(rhs)) }
	}

	this := *me
	var idxs_to_remove []int
	for i := len(this) - 1; i >= 0; i-- {
		for j := 0; j < i; j++ {
			if areEqual(this[i], this[j]) {
				idxs_to_remove = append(idxs_to_remove, j) // dont `break`, there might be more =)
			}
		}
	}
	this = WithoutIdxs(idxs_to_remove, this)
	*me = this
}
