package telemetry

import (
	"fmt"
	"io"
	"time"
)

type Report struct {
	Wall    time.Duration
	Metrics []string
}


func NewReport(wall time.Duration, metrics []string) Report {
	return Report{Wall: wall, Metrics: metrics}
}


func Print(w io.Writer, r Report) {
	fmt.Fprintf(w, "wall: %s\n", r.Wall)
	for _, m := range r.Metrics {
		fmt.Fprintln(w, m)
	}
}
