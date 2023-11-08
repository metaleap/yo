package dict

type Any = map[string]any
type Of[T any] map[string]T

func Keys[K comparable, V any](m map[K]V) (ret []K) {
	ret = make([]K, 0, len(m))
	for k := range m {
		ret = append(ret, k)
	}
	return
}

func Fill[K comparable, V any](dst map[K]V, from map[K]V) map[K]V {
	for k, v := range from {
		dst[k] = v
	}
	return dst
}
