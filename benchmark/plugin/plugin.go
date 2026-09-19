package plugin

import "flag"

type Metric struct {
	Name  string
	Value float64
	Samples []float64
}


type Benchmark interface {
	Name() string
	RegisterFlags(fs *flag.FlagSet)
	Build() (binPath string, err error)
	Argv(binPath string) []string
	Metrics(output []byte) []Metric
}
