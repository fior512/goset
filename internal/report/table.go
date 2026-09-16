package report

import (
	"fmt"
	"math"
	"slices"
	"time"

	"goset/internal/cpu"
	"goset/internal/telemetry"
)

func SelectionTable(s *cpu.SelectionResult) Table {
	header := []string{"cpu", "sel", "soft", "hard", "sibl", "isol", "node", "nohz", "rcu"}
	rows := make([][]string, 0, len(s.Scores))
	for _, sc := range s.Scores {
		sel := ""
		switch {
		case sc.CPU == s.HouseKeeper:
			sel = "& "
		case s.Benchmark.GetBit(sc.CPU):
			sel = "* "
		}
		flag := func(b bool) string {
			if b {
				return "y"
			}
			return ""
		}
		rows = append(rows, []string{
			fmt.Sprintf("%3d", sc.CPU),
			sel,
			FormatValue(float64(sc.Steerable)),
			FormatValue(float64(sc.NonSteerable)),
			FormatValue(float64(sc.SiblingLoad)),
			flag(sc.KernelIsol),
			fmt.Sprintf("%4d", sc.Node),
			flag(sc.NohzFull),
			flag(sc.RcuNocb),
		})
	}
	return Table{Title: "Selection", Header: header, Rows: rows}
}


type Report struct {
	Wall     time.Duration
	Counters []telemetry.Counter
	ExitCode int
}


func TelemetryTable(r Report) Table {
	var labels []string
	seen := map[string]bool{}
	byLabel := map[string]map[int]float64{}
	cpuSeen := map[int]bool{}

	for _, m := range r.Counters {
		label := m.Source + " " + m.Name
		if !seen[label] {
			seen[label] = true
			labels = append(labels, label)
			byLabel[label] = map[int]float64{}
		}
		byLabel[label][m.CPU] = m.Value
		cpuSeen[m.CPU] = true
	}

	cpus := make([]int, 0, len(cpuSeen))
	for c := range cpuSeen {
		cpus = append(cpus, c)
	}
	slices.Sort(cpus)

	header := []string{"counters"}
	for _, c := range cpus {
		header = append(header, fmt.Sprintf("cpu%d", c))
	}
	header = append(header, "avg", "sd", "sum")

	rows := make([][]string, 0, len(labels))
	for _, label := range labels {
		values := byLabel[label]
		row := []string{label}
		var series []float64
		for _, c := range cpus {
			if v, ok := values[c]; ok {
				row = append(row, FormatValue(v))
				series = append(series, v)
			} else {
				row = append(row, "-")
			}
		}
		row = append(row,
			FormatValue(avg(series)),
			FormatValue(stddev(series)),
			FormatValue(sum(series)))
		rows = append(rows, row)
	}

	return Table{Title: "Telemetry", Header: header, Rows: rows}
}


func GlobalTable(r Report) Table {
	rows := [][]string{
		{"wall", FormatTime(r.Wall)},
		{"exit", fmt.Sprintf("%d", r.ExitCode)},
	}
	return Table{Title: "Global", Header: []string{"key", "value"}, Rows: rows}
}


func avg(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	return sum(v) / float64(len(v))
}


func sum(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x
	}
	return s
}


func stddev(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := avg(v)
	var acc float64
	for _, x := range v {
		d := x - m
		acc += d * d
	}
	return math.Sqrt(acc / float64(len(v)))
}
