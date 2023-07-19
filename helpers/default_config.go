package helpers

func IntConfigDefault(x int, def int) int {
	if x == 0 {
		return def
	}
	return x
}
