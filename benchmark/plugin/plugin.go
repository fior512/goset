package plugin

import "flag"

type Metric struct {
	Name    string // alignment identity (masked label, column, unit)
	Display string // compact label for rendering (row name + unit)
	Value   float64
	Samples []float64
	Col     int // source column ordinal within the row (for ordering)
}


type Benchmark interface {
	Name() string
	RegisterFlags(fs *flag.FlagSet)
	Build() (binPath string, err error)
	Argv(binPath string) []string
	Metrics(output []byte) []Metric
}
