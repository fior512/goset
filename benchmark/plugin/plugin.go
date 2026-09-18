// Package plugin defines the contract goset-bench uses to run a known
// HPC benchmark (STREAM, and later HPCC/IO500) under goset. It's a leaf
// package (no dependency on benchmark implementations) so both the
// registry (benchmark) and each implementation (benchmark/stream, ...)
// can import it without a cycle.
package plugin

import "flag"

// Metric is one named number a benchmark reports, in the order the
// benchmark wants it displayed (e.g. STREAM's -stream-metrics flag
// controls this order directly).
type Metric struct {
	Name  string
	Value float64
}

// Benchmark is a self-contained HPC benchmark: it builds its own binary
// and knows how to read its own headline metrics back out of a run's
// combined stdout/stderr. Benchmarks that self-pin (e.g. likwid-bench)
// don't belong here — goset-bench needs to own affinity itself.
type Benchmark interface {
	Name() string
	RegisterFlags(fs *flag.FlagSet)
	Build() (binPath string, err error)
	Argv(binPath string) []string
	// Metrics extracts the benchmark's own headline numbers from a run's
	// output, filtered/ordered by whatever selection flags the benchmark
	// registered (e.g. -stream-metrics), so the report stays terse.
	Metrics(output []byte) []Metric
}
