package yodb

import sl "yo/util/sl"

func (me JsonArr[T]) All(a1 func(T) bool) bool            { return ((sl.Slice[T])(me)).All(a1) }
func (me JsonArr[T]) Any(a1 func(T) bool) bool            { return ((sl.Slice[T])(me)).Any(a1) }
func (me JsonArr[T]) IdxWhere(a1 func(T) bool) int        { return ((sl.Slice[T])(me)).IdxWhere(a1) }
func (me JsonArr[T]) ToAnys() []interface{}               { return ((sl.Slice[T])(me)).ToAnys() }
func (me JsonArr[T]) Where(a1 func(T) bool) sl.Slice[T]   { return ((sl.Slice[T])(me)).Where(a1) }
func (me JsonArr[T]) Without(a1 func(T) bool) sl.Slice[T] { return ((sl.Slice[T])(me)).Without(a1) }
func (me *JsonArr[T]) EnsureAllUnique(a1 func(T, T) bool) { ((*sl.Slice[T])(me)).EnsureAllUnique(a1) }
func (me Arr[T]) All(a1 func(T) bool) bool                { return ((sl.Slice[T])(me)).All(a1) }
func (me Arr[T]) Any(a1 func(T) bool) bool                { return ((sl.Slice[T])(me)).Any(a1) }
func (me Arr[T]) IdxWhere(a1 func(T) bool) int            { return ((sl.Slice[T])(me)).IdxWhere(a1) }
func (me Arr[T]) ToAnys() []interface{}                   { return ((sl.Slice[T])(me)).ToAnys() }
func (me Arr[T]) Where(a1 func(T) bool) sl.Slice[T]       { return ((sl.Slice[T])(me)).Where(a1) }
func (me Arr[T]) Without(a1 func(T) bool) sl.Slice[T]     { return ((sl.Slice[T])(me)).Without(a1) }
func (me *Arr[T]) EnsureAllUnique(a1 func(T, T) bool)     { ((*sl.Slice[T])(me)).EnsureAllUnique(a1) }
