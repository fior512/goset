package telemetry

type Target struct {
	BenchCPUs   []int
	Housekeeper int
}


type Source interface {
	Baseline(t Target) error // starting point
	Poll() error             // sample
	Stop() error             // ending point
	Summary() string         // report
}
