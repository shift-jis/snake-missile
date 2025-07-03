package utilities

func Conditional[T any](condition bool, a, b T) T {
	if condition {
		return a
	}
	return b
}
