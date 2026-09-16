package telemetry

type Target struct {
	BenchCPUs   []int
	Housekeeper int
}


type Counter struct {
	Source string
	CPU    int
	Name   string
	Value  float64
}


type Source interface {
	Baseline(t Target) error // starting point
	Poll() error             // sample
	Stop() error             // ending point
	Summary() []Counter      // report
}
