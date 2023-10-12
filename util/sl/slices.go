package sl

import (
	"cmp"
	"slices"
)

func WithoutIdx[TSl ~[]TEl, TEl any](slice TSl, sansIdx int, noMake bool) (ret TSl) {
	if (sansIdx < 0) || (sansIdx >= len(slice)) {
		return slice
	}
	if noMake {
		return append(slice[:sansIdx], slice[sansIdx+1:]...)
	}
	ret = make(TSl, 0, len(slice)-1)
	for i := range slice {
		if i != sansIdx {
			ret = append(ret, slice[i])
		}
	}
	return
}

func WithoutIdxs[TSl ~[]TEl, TEl any](slice TSl, sansIdxs ...int) (ret TSl) {
	if len(sansIdxs) == 0 {
		return slice
	}
	ret = make(TSl, 0, len(slice)-len(sansIdxs))
	for i := range slice {
		if !Has(sansIdxs, i) {
			ret = append(ret, slice[i])
		}
	}
	return
}

func WithoutIdxRange[TSl ~[]TEl, TEl any](slice TSl, delFromIdx int, delUntilIdx int) TSl {
	if (delFromIdx <= 0) && ((delUntilIdx < 0) || (delUntilIdx >= len(slice))) {
		return TSl{}
	}
	return append(append(make(TSl, 0, len(slice)-(delUntilIdx-delFromIdx)), slice[:delFromIdx]...), slice[delUntilIdx:]...)
}

func Sorted[TSl ~[]TEl, TEl cmp.Ordered](slice TSl) TSl {
	slices.Sort(slice)
	return slice
}

func IdxOf[TSl ~[]TEl, TEl comparable](s TSl, v TEl) int {
	for i := range s {
		if v == s[i] {
			return i
		}
	}
	return -1
}

func IdxWhere[TSl ~[]TEl, TEl any](slice TSl, pred func(TEl) bool) int {
	for i := range slice {
		if pred(slice[i]) {
			return i
		}
	}
	return -1
}

func Has[TSl ~[]TEl, TEl comparable](slice TSl, needle TEl) bool {
	for i := range slice {
		if slice[i] == needle {
			return true
		}
	}
	return false
}

func HasAnyOf[TSl ~[]TEl, TEl comparable](slice TSl, of ...TEl) bool {
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

func HasAllOf[TSl ~[]TEl, TEl comparable](slice TSl, of ...TEl) bool {
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

func Map[TSl ~[]TEl, TEl any, TOut any](slice TSl, f func(TEl) TOut) (ret []TOut) {
	ret = make([]TOut, len(slice))
	for i := range slice {
		ret[i] = f(slice[i])
	}
	return
}

func All[TSl ~[]TEl, TEl any](slice TSl, pred func(TEl) bool) bool {
	for i := range slice {
		if !pred(slice[i]) {
			return false
		}
	}
	return true
}

func Any[TSl ~[]TEl, TEl any](slice TSl, pred func(TEl) bool) bool {
	for i := range slice {
		if pred(slice[i]) {
			return true
		}
	}
	return false
}

func Without[TSl ~[]TEl, TEl comparable](slice TSl, without ...TEl) TSl {
	if len(without) == 0 {
		return slice
	}
	return Where(slice, func(item TEl) bool {
		return !Has(without, item)
	})
}

func Where[TSl ~[]TEl, TEl any](slice TSl, pred func(TEl) bool) (ret TSl) {
	ret = make(TSl, 0, len(slice))
	for i := range slice {
		if pred(slice[i]) {
			ret = append(ret, slice[i])
		}
	}
	return
}
