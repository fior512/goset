package plugin

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
)

func SampleMetric(output []byte, tag, display string) []Metric {
	sampleRe := regexp.MustCompile(fmt.Sprintf(`(?m)^SAMPLE %s (\d+) ([0-9.]+)`, regexp.QuoteMeta(tag)))
	var samples []float64
	for _, m := range sampleRe.FindAllSubmatch(output, -1) {
		if v, err := strconv.ParseFloat(string(m[2]), 64); err == nil {
			samples = append(samples, v)
		}
	}
	if len(samples) == 0 {
		return nil
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	return []Metric{{Name: "iter", Display: display, Value: sorted[len(sorted)/2], Samples: samples}}
}
