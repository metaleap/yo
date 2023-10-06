package yo

var jsonNullTok = []byte("null")

type T0 struct {
}
type T1[T any] struct {
	Val1 T
}
type T2[T1 any, T2 any] struct {
	Val1 T1
	Val2 T2
}
type T3[T1 any, T2 any, T3 any] struct {
	Val1 T1
	Val2 T2
	Val3 T3
}
