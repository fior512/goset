package pingpong

import (
	_ "embed"
	"flag"
	"strconv"

	"goset/benchmark/plugin"
)

//go:embed pingpong.c.tmpl
var source string

type Benchmark struct {
	roundtrips int
	ntimes     int
}

func (b *Benchmark) Name() string { return "pingpong" }

func (b *Benchmark) RegisterFlags(fs *flag.FlagSet) {
	fs.IntVar(&b.roundtrips, "pingpong-roundtrips", 20_000, "pingpong: pipe round trips per timed iteration")
	fs.IntVar(&b.ntimes, "pingpong-ntimes", 300, "pingpong: timed iterations, per-iteration samples kept for tail analysis")
}

func (b *Benchmark) Build() (string, error) { return plugin.BuildC("pingpong", source, "-pthread") }

func (b *Benchmark) Argv(bin string) []string {
	return []string{bin, strconv.Itoa(b.roundtrips), strconv.Itoa(b.ntimes)}
}

func (b *Benchmark) Metrics(output []byte) []plugin.Metric {
	return plugin.SampleMetric(output, "pingpong", "pingpong iter")
}
