package utils

func MinOf(vars ...int) int {
	min := vars[0]

	for _, i := range vars {
		if min > i {
			min = i
		}
	}

	return min
}

func MaxOf(vars ...int) int {
	max := vars[0]

	for _, i := range vars {
		if max < i {
			max = i
		}
	}

	return max
}

// Abs return abs of the input integer
func Abs(v int64) int64 {
	if v >= 0 {
		return v
	}
	return -v
}
