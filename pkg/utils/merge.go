package utils

// Merge merges the src map into the dst map.
func Merge[K comparable, V any](dst *map[K]V, src map[K]V) {
	if *dst == nil {
		*dst = make(map[K]V)
	}

	for k, v := range src {
		(*dst)[k] = v
	}
}
