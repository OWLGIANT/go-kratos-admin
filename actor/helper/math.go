package helper

import "math"

func Min[T int64 | int32 | int16 | int8 | int |
	float64 | float32](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func Max[T int64 | int32 | int16 | int8 | int |
	float64 | float32](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func RoundToPrecision(v float64, precision int) float64 {
	shift := math.Pow(10, float64(precision))
	return math.Round(v*shift) / shift
}
