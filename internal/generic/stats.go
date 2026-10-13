package generic

import (
	"math"
	"sort"
)

func Sum(values []float64) float64 {
	var total float64
	for _, value := range values {
		total += value
	}
	return total
}

func Avg(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return Sum(values) / float64(len(values))
}

func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func StdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := Avg(values)
	var acc float64
	for _, value := range values {
		diff := value - mean
		acc += diff * diff
	}
	return math.Sqrt(acc / float64(len(values)))
}

func Percentile(values []float64, fraction float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	idx := int(fraction * float64(len(sorted)-1))
	return sorted[idx]
}

func MinOf(values []float64) float64 {
	smallest := values[0]
	for _, value := range values[1:] {
		if value < smallest {
			smallest = value
		}
	}
	return smallest
}

func MaxOf(values []float64) float64 {
	largest := values[0]
	for _, value := range values[1:] {
		if value > largest {
			largest = value
		}
	}
	return largest
}
