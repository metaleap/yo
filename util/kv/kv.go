package kv

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

func FromKeys[K comparable, V any](keys []K, value func(K) V) map[K]V {
	ret := make(map[K]V, len(keys))
	for _, key := range keys {
		ret[key] = value(key)
	}
	return ret
}

func FromValues[K comparable, V any](values []V, key func(V) K) map[K]V {
	ret := make(map[K]V, len(values))
	for _, val := range values {
		ret[key(val)] = val
	}
	return ret
}
