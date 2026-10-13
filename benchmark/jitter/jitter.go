package jitter

import (
	_ "embed"
	"flag"
	"regexp"
	"sort"
	"strconv"

	"goset/benchmark/plugin"
)

//go:embed jitter.c.tmpl
var source string

type Benchmark struct {
	ops    int
	ntimes int
}

func (b *Benchmark) Name() string { return "jitter" }

func (b *Benchmark) RegisterFlags(flags *flag.FlagSet) {
	flags.IntVar(&b.ops, "jitter-ops", 3_000_000, "jitter: chain steps per timed iteration")
	flags.IntVar(&b.ntimes, "jitter-ntimes", 300, "jitter: timed iterations, per-iteration samples kept for tail analysis")
}

func (b *Benchmark) Build() (string, error) { return plugin.BuildC("jitter", source) }

func (b *Benchmark) Argv(bin string) []string {
	return []string{bin, strconv.Itoa(b.ops), strconv.Itoa(b.ntimes)}
}

var sampleRe = regexp.MustCompile(`(?m)^SAMPLE jitter (\d+) ([0-9.]+)`)

func (b *Benchmark) Metrics(output []byte) []plugin.Metric {
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
	med := sorted[len(sorted)/2]
	return []plugin.Metric{{Name: "iter", Display: "jitter iter", Value: med, Samples: samples}}
}
