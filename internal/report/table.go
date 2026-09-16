package report

import (
	"fmt"
	"math"
	"slices"
	"time"

	"goset/internal/cpu"
	"goset/internal/isolation"
	"goset/internal/telemetry"
)

func SelectionTable(sel *cpu.SelectionResult) Table {
	header := []string{"cpu", "sel", "soft", "hard", "sibl", "isol", "node", "nohz", "rcu"}
	rows := make([][]string, 0, len(sel.Scores))
	for _, sc := range sel.Scores {
		mark := ""
		switch {
		case sc.CPU == sel.HouseKeeper:
			mark = "& "
		case sel.Benchmark.GetBit(sc.CPU):
			mark = "* "
		}
		flag := func(on bool) string {
			if on {
				return "y"
			}
			return ""
		}
		rows = append(rows, []string{
			fmt.Sprintf("%3d", sc.CPU),
			mark,
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
	Steer    *isolation.SteerResult
}


func TelemetryTable(rep Report) Table {
	var labels []string
	seen := map[string]bool{}
	byLabel := map[string]map[int]float64{}
	cpuSeen := map[int]bool{}

	for _, counter := range rep.Counters {
		label := counter.Source + " " + counter.Name
		if !seen[label] {
			seen[label] = true
			labels = append(labels, label)
			byLabel[label] = map[int]float64{}
		}
		byLabel[label][counter.CPU] = counter.Value
		cpuSeen[counter.CPU] = true
	}

	cpus := make([]int, 0, len(cpuSeen))
	for cpu := range cpuSeen {
		cpus = append(cpus, cpu)
	}
	slices.Sort(cpus)

	header := []string{"counters"}
	for _, cpu := range cpus {
		header = append(header, fmt.Sprintf("cpu%d", cpu))
	}
	header = append(header, "avg", "sd", "sum")

	rows := make([][]string, 0, len(labels))
	for _, label := range labels {
		values := byLabel[label]
		row := []string{label}
		var series []float64
		for _, cpu := range cpus {
			if val, ok := values[cpu]; ok {
				row = append(row, FormatValue(val))
				series = append(series, val)
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


func GlobalTable(rep Report) Table {
	rows := [][]string{
		{"wall", FormatTime(rep.Wall)},
		{"exit", fmt.Sprintf("%d", rep.ExitCode)},
	}
	if r.Steer != nil {
		rows = append(rows,
			[]string{"irq steer applied", fmt.Sprintf("%d", r.Steer.Applied)},
			[]string{"irq steer rejected", fmt.Sprintf("%d", r.Steer.Rejected)},
			[]string{"irq steer remaining", fmt.Sprintf("%d", len(r.Steer.Remaining))},
		)
	}
	return Table{Title: "Global", Header: []string{"key", "value"}, Rows: rows}
}


func avg(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return sum(values) / float64(len(values))
}


func sum(values []float64) float64 {
	var total float64
	for _, val := range values {
		total += val
	}
	return total
}


func stddev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := avg(values)
	var acc float64
	for _, val := range values {
		diff := val - mean
		acc += diff * diff
	}
	return math.Sqrt(acc / float64(len(values)))
}
