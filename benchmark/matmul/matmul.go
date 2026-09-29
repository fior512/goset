package matmul

import (
	_ "embed"
	"flag"
	"strconv"

	"goset/benchmark/plugin"
)

//go:embed matmul.c.tmpl
var source string

type Benchmark struct {
	reps   int
	ntimes int
}

func (b *Benchmark) Name() string { return "matmul" }

func (b *Benchmark) RegisterFlags(fs *flag.FlagSet) {
	fs.IntVar(&b.reps, "matmul-reps", 2000, "matmul: 32x32 multiplications per timed iteration")
	fs.IntVar(&b.ntimes, "matmul-ntimes", 300, "matmul: timed iterations, per-iteration samples kept for tail analysis")
}

func (b *Benchmark) Build() (string, error) { return plugin.BuildC("matmul", source) }

func (b *Benchmark) Argv(bin string) []string {
	return []string{bin, strconv.Itoa(b.reps), strconv.Itoa(b.ntimes)}
}

func (b *Benchmark) Metrics(output []byte) []plugin.Metric {
	return plugin.SampleMetric(output, "matmul", "matmul iter")
}
