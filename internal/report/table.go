package report

import (
	"fmt"
	"math"
	"slices"
	"syscall"
	"time"

	"goset/internal/cpu"
	"goset/internal/isolation"
	"goset/internal/telemetry"
)

func SelectionTable(sel *cpu.SelectionResult, pinned bool) Table {
	header := []string{"cpu", "sel", "soft", "hard", "sibl", "isol", "node", "nohz", "rcu"}
	rows := make([][]string, 0, len(sel.Scores))
	for _, sc := range sel.Scores {
		mark := ""
		switch {
		case sc.CPU == sel.HouseKeeper:
			mark = "& "
		case sel.Benchmark.GetBit(sc.CPU) && pinned:
			mark = "* "
		case sel.Benchmark.GetBit(sc.CPU):
			mark = ". "
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
	Counters []telemetry.Counter
	Steer    *isolation.SteerResult
	Rusage   *syscall.Rusage
	Wall     time.Duration
	ExitCode int
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
	rows := [][]string{}
	if rep.Steer != nil {
		rows = append(rows,
			[]string{"irq steer applied", fmt.Sprintf("%d", rep.Steer.Applied)},
			[]string{"irq steer rejected", fmt.Sprintf("%d", rep.Steer.Rejected)},
			[]string{"irq steer remaining", fmt.Sprintf("%d", len(rep.Steer.Remaining))},
		)
	}
	if rep.Rusage != nil {
		rows = append(rows,
			[]string{"ctxsw voluntary", fmt.Sprintf("%d", rep.Rusage.Nvcsw)},
			[]string{"ctxsw involuntary", fmt.Sprintf("%d", rep.Rusage.Nivcsw)},
		)
	}
	rows = append(rows,
		[]string{"wall", FormatTime(rep.Wall)},
		[]string{"exit", fmt.Sprintf("%d", rep.ExitCode)},
	)
	return Table{Title: "Global", Header: []string{"key", "value"}, Rows: rows}
}


func TopologyTable(topo *cpu.Topology) Table {
	header := []string{"cpu", "core", "node", "isol", "nohz", "rcu", "driver", "gov", "epp", "minf", "maxf"}
	flag := func(on bool) string {
		if on {
			return "y"
		}
		return ""
	}
	var rows [][]string
	for id := range topo.Online.All() {
		rows = append(rows, []string{
			fmt.Sprintf("%3d", id),
			fmt.Sprintf("%4d", topo.Core[id]),
			fmt.Sprintf("%4d", topo.NumaNode[id]),
			flag(topo.KernelIsol.GetBit(id)),
			flag(topo.NohzFull.GetBit(id)),
			flag(topo.RcuNocb.GetBit(id)),
			topo.Driver[id],
			topo.Governor[id],
			topo.EPP[id],
			FormatFreq(float64(topo.MinFreq[id]) * 1000),
			FormatFreq(float64(topo.MaxFreq[id]) * 1000),
		})
	}
	return Table{Title: "Topology", Header: header, Rows: rows}
}


func EnvironmentTable(env *cpu.Environment) Table {
	rows := [][]string{
		{"smt", env.SMT},
		{"boost", env.Boost},
		{"numa_balancing", env.NumaBalancing},
		{"nmi_watchdog", env.NmiWatchdog},
		{"thp", env.THP},
		{"mitigations", fmt.Sprintf("%d", env.Mitigations)},
	}
	return Table{Title: "Environment", Header: []string{"key", "value"}, Rows: rows}
}


func CgroupTable(infos []isolation.CgroupInfo) Table {
	header := []string{"name", "cpus", "mems", "partition", "procs", "usage_usec", "nr_throttled"}
	if len(infos) == 0 {
		return Table{Title: "Cgroups", Header: header, Rows: [][]string{{"No Cgroup found"}}}
	}
	rows := make([][]string, 0, len(infos))
	for _, info := range infos {
		rows = append(rows, []string{
			info.Name,
			info.Cpus,
			info.Mems,
			info.Partition,
			fmt.Sprintf("%d", info.Procs),
			FormatValue(float64(info.Stat["usage_usec"])),
			FormatValue(float64(info.Stat["nr_throttled"])),
		})
	}
	return Table{Title: "Cgroups", Header: header, Rows: rows}
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
