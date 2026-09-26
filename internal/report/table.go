package report

import (
	"fmt"
	"math"
	"slices"
	"syscall"
	"time"

	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
	"goset/internal/telemetry"
)

func SelectionTable(selected *generic.Selection) Table {
	header := []string{"cpu", "sel", "steerable", "non-steerable", "sibl", "isol", "node", "nohz", "rcu"}
	rows := make([][]string, 0, len(selected.Scores))
	for _, candidate := range selected.Scores {
		mark := ""
		switch {
		case candidate.CPU == selected.HouseKeeper:
			mark = "& "
		case selected.Task.GetBit(candidate.CPU):
			mark = "* "
		}
		flag := func(on bool) string {
			if on {
				return "y"
			}
			return ""
		}
		rows = append(rows, []string{
			fmt.Sprintf("%3d", candidate.CPU),
			mark,
			FormatValue(float64(candidate.Steerable)),
			FormatValue(float64(candidate.NonSteerable)),
			FormatValue(float64(candidate.SiblingLoad)),
			flag(candidate.KernelIsol),
			fmt.Sprintf("%4d", candidate.NumaNode),
			flag(candidate.NohzFull),
			flag(candidate.RcuNocb),
		})
	}
	return Table{Title: "Selection", Header: header, Rows: rows}
}


type Report struct {
	Counters []telemetry.Counter
	Steer    *isolation.Steering
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
		if counter.CPU < 0 { // run-global: reported in Global
			continue
		}
		label := counter.Label()
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
			[]string{generic.GlobalIRQBalanceHeld, rep.Steer.Status()},
			[]string{generic.GlobalIRQSteerApplied, fmt.Sprintf("%d", rep.Steer.Applied)},
			[]string{generic.GlobalIRQSteerRejected, fmt.Sprintf("%d", rep.Steer.Rejected)},
			[]string{generic.GlobalIRQSteerRemaining, fmt.Sprintf("%d", len(rep.Steer.Remaining))},
			[]string{generic.GlobalIRQSteerDrift, fmt.Sprintf("%d", telemetry.CountDriftedIRQs(rep.Counters))},
		)
	}
	if rep.Rusage != nil {
		rows = append(rows,
			[]string{generic.GlobalCtxswVoluntary, fmt.Sprintf("%d", rep.Rusage.Nvcsw)},
			[]string{generic.GlobalCtxswInvoluntary, fmt.Sprintf("%d", rep.Rusage.Nivcsw)},
		)
	}
	if migrations, ok := telemetry.CountMigrations(rep.Counters); ok {
		rows = append(rows, []string{generic.GlobalMigrations, fmt.Sprintf("%d", migrations)})
	}
	if runDelay, ok := telemetry.CountRunqueue(rep.Counters); ok {
		rows = append(rows, []string{generic.GlobalRunDelay, FormatTime(time.Duration(runDelay))})
	}
	rows = append(rows,
		[]string{generic.GlobalWall, FormatTime(rep.Wall)},
		[]string{generic.GlobalExit, fmt.Sprintf("%d", rep.ExitCode)},
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
		{"smt", optionalValue(env.SMT)},
		{"boost", env.Boost},
		{"numa_balancing", optionalValue(env.NumaBalancing)},
		{"nmi_watchdog", optionalValue(env.NmiWatchdog)},
		{"thp", optionalValue(env.THP)},
		{"mitigations", optionalValue(env.Mitigations)},
	}
	return Table{Title: "Environment", Header: []string{"key", "value"}, Rows: rows}
}

func optionalValue[T any](value *T) string {
	if value == nil {
		return "unavailable"
	}
	return fmt.Sprint(*value)
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
