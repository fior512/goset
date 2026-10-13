package stream

import (
	_ "embed"
	"flag"
	"regexp"
	"strconv"
	"strings"

	"goset/benchmark/plugin"
)

//go:embed stream.c.tmpl
var source string

type Benchmark struct {
	size    int
	ntimes  int
	metrics string
}


func (b *Benchmark) Name() string { return "stream" }

func (b *Benchmark) RegisterFlags(fs *flag.FlagSet) {
	fs.IntVar(&b.size, "stream-size", 20_000_000, "STREAM: array elements per array")
	fs.IntVar(&b.ntimes, "stream-ntimes", 300, "STREAM: timed iterations, per-iteration samples kept for tail analysis")
	fs.StringVar(&b.metrics, "stream-metrics", "Triad",
		"STREAM: comma-separated kernels to report, in order (Copy,Scale,Add,Triad)")
}


func (b *Benchmark) Build() (string, error) { return plugin.BuildC("stream", source) }


func (b *Benchmark) Argv(bin string) []string {
	return []string{bin, strconv.Itoa(b.size), strconv.Itoa(b.ntimes)}
}


var kernelRe = regexp.MustCompile(`(?m)^(Copy|Scale|Add|Triad)\s+([0-9.]+)`)
var sampleRe = regexp.MustCompile(`(?m)^SAMPLE (Copy|Scale|Add|Triad) (\d+) ([0-9.]+)`)

func (b *Benchmark) Metrics(output []byte) []plugin.Metric {
	best := map[string]float64{}
	for _, m := range kernelRe.FindAllSubmatch(output, -1) {
		if v, err := strconv.ParseFloat(string(m[2]), 64); err == nil {
			best[string(m[1])] = v
		}
	}

	samples := map[string][]float64{}
	for _, m := range sampleRe.FindAllSubmatch(output, -1) {
		kernel := string(m[1])
		if v, err := strconv.ParseFloat(string(m[3]), 64); err == nil {
			samples[kernel] = append(samples[kernel], v)
		}
	}

	var out []plugin.Metric
	for _, name := range strings.Split(b.metrics, ",") {
		name = strings.TrimSpace(name)
		if v, ok := best[name]; ok {
			out = append(out, plugin.Metric{Name: name + " MB/s", Value: v, Samples: samples[name]})
		}
	}
	return out
}
