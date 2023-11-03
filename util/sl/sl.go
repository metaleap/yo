package sl

import (
	"cmp"
	"reflect"
	"slices"
)

func Keys[K comparable, V any](m map[K]V) (ret []K) {
	ret = make([]K, 0, len(m))
	for k := range m {
		ret = append(ret, k)
	}
	return
}

func WithoutIdx[TSlice ~[]TItem, TItem any](slice TSlice, sansIdx int, noMake bool) (ret TSlice) {
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

func WithoutIdxs[TSlice ~[]TItem, TItem any](slice TSlice, sansIdxs ...int) (ret TSlice) {
	if len(sansIdxs) == 0 {
		return slice
	}
	ret = make(TSlice, 0, len(slice)-len(sansIdxs))
	for i := range slice {
		if !Has(sansIdxs, i) {
			ret = append(ret, slice[i])
		}
	}
	return
}

func WithoutIdxRange[TSlice ~[]TItem, TItem any](slice TSlice, delFromIdx int, delUntilIdx int) TSlice {
	if (delFromIdx <= 0) && ((delUntilIdx < 0) || (delUntilIdx >= len(slice))) {
		return TSlice{}
	}
	return append(append(make(TSlice, 0, len(slice)-(delUntilIdx-delFromIdx)), slice[:delFromIdx]...), slice[delUntilIdx:]...)
}

func Reversed[TSlice ~[]TItem, TItem any](slice TSlice) TSlice {
	for i := range slice[:len(slice)/2] {
		idx_opp := (len(slice) - 1) - i
		item_opp := slice[idx_opp]
		slice[idx_opp], slice[i] = slice[i], item_opp
	}
	return slice
}

func Sorted[TSlice ~[]TItem, TItem cmp.Ordered](slice TSlice) TSlice {
	slices.Sort(slice)
	return slice
}
func SortedPer[TSlice ~[]TItem, TItem any](slice TSlice, cmp func(TItem, TItem) int) TSlice {
	slices.SortStableFunc(slice, cmp)
	return slice
}

func IdxOf[TSlice ~[]TItem, TItem comparable](s TSlice, v TItem) int {
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

func IdxsWhere[TSlice ~[]TItem, TItem any](slice TSlice, pred func(TItem) bool) (ret []int) {
	for i := range slice {
		if pred(slice[i]) {
			ret = append(ret, i)
		}
	}
	return
}

func HasWhere[TSlice ~[]TItem, TItem any](slice TSlice, pred func(TItem) bool) bool {
	return (IdxWhere(slice, pred) >= 0)
}

func Has[TSlice ~[]TItem, TItem comparable](slice TSlice, needle TItem) bool {
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
		return Has(slice, of[0])
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
		return Has(slice, of[0])
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

func To[TSlice ~[]TItem, TItem any, TOut any](slice TSlice, f func(TItem) TOut) (ret Of[TOut]) {
	ret = make(Of[TOut], len(slice))
	for i := range slice {
		ret[i] = f(slice[i])
	}
	return
}

func ToAnys[TSlice ~[]TItem, TItem any](slice TSlice) []any {
	return To(slice, func(it TItem) any { return it })
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
		return !Has(without, item)
	})
}

func FirstWhere[TSlice ~[]TItem, TItem any](slice TSlice, pred func(TItem) bool) (ret TItem) {
	for i := range slice {
		if pred(slice[i]) {
			return slice[i]
		}
	}
	return
}

func FirstNonNil[T any](slice ...*T) *T {
	for i := range slice {
		if slice[i] != nil {
			return slice[i]
		}
	}
	return nil
}

func Grouped[TKey comparable, TItem any, TSlice ~[]TItem](slice TSlice, key func(TItem) TKey) (ret map[TKey]TSlice) {
	ret = make(map[TKey]TSlice, len(slice)/2)
	for i := range slice {
		key := key(slice[i])
		ret[key] = append(ret[key], slice[i])
	}
	return
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

// add only those `items` not yet in `slice`.
func With[TSlice ~[]TItem, TItem comparable](slice TSlice, items ...TItem) TSlice {
	append_from := 0
	for i := range items {
		if IdxOf(slice, items[i]) < 0 {
			slice = append(slice, items[append_from:i+1]...)
		}
		append_from = i + 1
	}
	return append(slice, items[append_from:]...)
}

func Uniq[TSlice ~[]TItem, TItem comparable](slice TSlice) TSlice {
	dupl_idxs := make([]int, 0, 2)
	for i := len(slice) - 1; i > 0; i-- {
		look_from := 0
		for idx := IdxOf(slice[look_from:i], slice[i]); (look_from < i) && (idx >= 0); idx = IdxOf(slice[look_from:i], slice[i]) {
			dupl_idxs = append(dupl_idxs, look_from+idx)
			look_from = look_from + idx + 1
		}
	}
	return WithoutIdxs(slice, dupl_idxs...)
}

func Repeat[TItem any](howMany int, item TItem) []TItem {
	ret := make([]TItem, howMany)
	for i := range ret {
		ret[i] = item
	}
	return ret
}

func ToPtrs[TSlice ~[]TItem, TItem any](slice TSlice) (ret []*TItem) {
	ret = make([]*TItem, len(slice))
	for i := range slice {
		ret[i] = &slice[i]
	}
	return
}

type Of[T any] []T

func New[T any](items ...T) Of[T] { return items }

func (me Of[T]) Any(pred func(T) bool) bool     { return Any(me, pred) }
func (me Of[T]) All(pred func(T) bool) bool     { return All(me, pred) }
func (me Of[T]) IdxWhere(pred func(T) bool) int { return IdxWhere(me, pred) }
func (me Of[T]) Where(pred func(T) bool) Of[T]  { return Where(me, pred) }
func (me Of[T]) Without(pred func(T) bool) Of[T] {
	return Where(me, func(it T) bool { return !pred(it) })
}

func (me Of[T]) ToAnys() (ret []any) {
	return ToAnys(me)
}

func (me *Of[T]) EnsureAllUnique(areEqual func(T, T) bool) {
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
	this = WithoutIdxs(this, idxs_to_remove...)
	*me = this
}
