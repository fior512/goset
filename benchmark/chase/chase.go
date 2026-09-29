package chase

import (
	_ "embed"
	"flag"
	"strconv"

	"goset/benchmark/plugin"
)

//go:embed chase.c.tmpl
var source string

type Benchmark struct {
	kib    int
	loads  int
	ntimes int
}

func (b *Benchmark) Name() string { return "chase" }

func (b *Benchmark) RegisterFlags(fs *flag.FlagSet) {
	fs.IntVar(&b.kib, "chase-kib", 192, "chase: ring size in KiB, keep it inside the L2 cache")
	fs.IntVar(&b.loads, "chase-loads", 2_000_000, "chase: dependent loads per timed iteration")
	fs.IntVar(&b.ntimes, "chase-ntimes", 300, "chase: timed iterations, per-iteration samples kept for tail analysis")
}

func (b *Benchmark) Build() (string, error) { return plugin.BuildC("chase", source) }

func (b *Benchmark) Argv(bin string) []string {
	return []string{bin, strconv.Itoa(b.kib), strconv.Itoa(b.loads), strconv.Itoa(b.ntimes)}
}

func (b *Benchmark) Metrics(output []byte) []plugin.Metric {
	return plugin.SampleMetric(output, "chase", "chase iter")
}
