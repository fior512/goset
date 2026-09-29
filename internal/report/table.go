package report

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
	"goset/internal/telemetry"
)

func SelectionTable(selected *generic.Selection) Table {
	header := []string{"cpu", "sel", "steerable", "non-steerable", "sibl", "isol", "numa", "nohz", "rcu"}
	rows := make([][]string, 0, len(selected.Scores))
	for _, candidate := range selected.Scores {
		mark := ""
		switch {
		case candidate.CPU == selected.HouseKeeper:
			mark = "& "
		case selected.Task.GetBit(candidate.CPU):
			mark = "* "
		case selected.Fence.GetBit(candidate.CPU):
			mark = "+ "
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
			fmt.Sprintf("%4d", candidate.Numa),
			flag(candidate.NohzFull),
			flag(candidate.RcuNocb),
		})
	}
	return Table{Title: "Selection", Header: header, Rows: rows}
}

type Report struct {
	Cpus     generic.CPUSet // task cpus: one Telemetry row each
	Counters []telemetry.Counter
	Steer    *isolation.Steering
	Rusage   *syscall.Rusage
	Wall     time.Duration
	ExitCode int
	Polls    int
	Interval time.Duration
}

type telemetryColumn struct {
	source string // column group
	name   string
	format func(float64) string
	reduce func([]float64) float64 // footer over the task cpus
}

// label composes the counter identity, Counter.Label is its only owner.
func (column telemetryColumn) label() string {
	return telemetry.Counter{Source: column.source, Name: column.name}.Label()
}

var telemetryColumns = []telemetryColumn{
	{generic.SourceFreq, generic.FreqMin, FormatFreq, slices.Min[[]float64]},
	{generic.SourceFreq, generic.FreqAvg, FormatFreq, avg},
	{generic.SourceFreq, generic.FreqMax, FormatFreq, slices.Max[[]float64]},
	{generic.SourceThrottle, generic.ThrottleCount, FormatValue, sum},
	{generic.SourceIRQ, generic.IRQSteerable, FormatValue, sum},
	{generic.SourceIRQ, generic.IRQNonSteerable, FormatValue, sum},
}

func TelemetryTables(rep Report, width int) []Table {
	values := perCPUValues(rep.Counters)
	var columns []telemetryColumn
	for _, column := range telemetryColumns {
		if len(values[column.label()]) > 0 {
			columns = append(columns, column)
		}
	}
	if len(columns) == 0 {
		return nil
	}

	/* layout */
	keys := 0
	tab := Table{Title: "Telemetry", Align: AlignRight}
	var groups []string
	if rep.Cpus.Count() > 1 {
		keys = 1
		tab.Align = AlignLabel
		tab.Header = append(tab.Header, generic.TelemetryCPU)
		groups = append(groups, "")
	}
	for _, column := range columns {
		tab.Header = append(tab.Header, column.label())
		groups = append(groups, column.source)
	}

	/* rows */
	for cpu := range rep.Cpus.All() {
		var row []string
		if keys > 0 {
			row = append(row, strconv.Itoa(cpu))
		}
		for _, column := range columns {
			value, ok := values[column.label()][cpu]
			if !ok {
				row = append(row, "-")
				continue
			}
			row = append(row, column.format(value))
		}
		tab.Rows = append(tab.Rows, row)
	}
	if keys > 0 {
		footer := []string{generic.TelemetryAll}
		for _, column := range columns {
			cell, ok := column.reduceAll(values[column.label()], rep.Cpus)
			if !ok {
				cell = "-"
			}
			footer = append(footer, cell)
		}
		tab.Rows = append(tab.Rows, footer)
	}
	return splitColumns(tab, groups, keys, width)
}

// reduceAll folds the column over every task cpu, and reports false when one
// of them carries no value, so the footer never stands for a subset.
func (column telemetryColumn) reduceAll(values map[int]float64, cpus generic.CPUSet) (string, bool) {
	series := make([]float64, 0, cpus.Count())
	for cpu := range cpus.All() {
		value, ok := values[cpu]
		if !ok {
			return "", false
		}
		series = append(series, value)
	}
	return column.format(column.reduce(series)), true
}

func perCPUValues(counters []telemetry.Counter) map[string]map[int]float64 {
	values := map[string]map[int]float64{}
	for _, counter := range counters {
		if counter.CPU < 0 {
			continue
		}
		label := counter.Label()
		if values[label] == nil {
			values[label] = map[int]float64{}
		}
		values[label][counter.CPU] = counter.Value
	}
	return values
}

func splitColumns(tab Table, groups []string, keys int, width int) []Table {
	widths := columnWidths(tab)
	var blocks [][]int
	block := indexRange(0, keys)
	for start := keys; start < len(groups); {
		end := start
		for end < len(groups) && groups[end] == groups[start] {
			end++
		}
		grown := slices.Concat(block, indexRange(start, end))
		if len(block) > keys && lineWidth(pick(widths, grown)) > width {
			blocks = append(blocks, block)
			grown = slices.Concat(indexRange(0, keys), indexRange(start, end))
		}
		block = grown
		start = end
	}
	blocks = append(blocks, block)

	tables := make([]Table, 0, len(blocks))
	for _, columns := range blocks {
		part := Table{Title: tab.Title, Header: pick(tab.Header, columns), Align: tab.Align}
		for _, row := range tab.Rows {
			part.Rows = append(part.Rows, pick(row, columns))
		}
		tables = append(tables, part)
	}
	return tables
}

func indexRange(start, end int) []int {
	out := make([]int, 0, end-start)
	for idx := start; idx < end; idx++ {
		out = append(out, idx)
	}
	return out
}

func pick[T any](values []T, indexes []int) []T {
	out := make([]T, 0, len(indexes))
	for _, idx := range indexes {
		out = append(out, values[idx])
	}
	return out
}

func NotReportedTable(rep Report) Table {
	values := perCPUValues(rep.Counters)
	var missing []string
	for _, column := range telemetryColumns {
		if len(values[column.label()]) == 0 && !slices.Contains(missing, column.source) {
			missing = append(missing, column.source)
		}
	}
	if len(missing) == 0 {
		return Table{}
	}
	return Table{Rows: [][]string{{"not reported: " + strings.Join(missing, ", ")}}}
}

const runPairsPerLine = 3

type runPair struct {
	key   string
	value string
}

func RunTable(rep Report) Table {
	scopes := []struct {
		name  string
		pairs []runPair
	}{
		{generic.ScopeTask, taskPairs(rep)},
		{generic.ScopeSched, schedPairs(rep)},
		{generic.ScopeSteer, steerPairs(rep)},
	}
	var rows [][]string
	for _, scope := range scopes {
		for start := 0; start < len(scope.pairs); start += runPairsPerLine {
			label := ""
			if start == 0 {
				label = scope.name
			}
			row := []string{label}
			for _, pair := range scope.pairs[start:min(start+runPairsPerLine, len(scope.pairs))] {
				row = append(row, pair.key, pair.value)
			}
			rows = append(rows, row)
		}
	}
	return Table{Title: "Run", Rows: rows, Align: AlignLeft}
}

func taskPairs(rep Report) []runPair {
	return []runPair{
		{generic.RunWall, FormatTime(rep.Wall)},
		{generic.RunExit, strconv.Itoa(rep.ExitCode)},
		{generic.RunSamples, fmt.Sprintf("%d@%s", rep.Polls, rep.Interval)},
	}
}

func schedPairs(rep Report) []runPair {
	var pairs []runPair
	if rep.Rusage != nil {
		pairs = append(pairs,
			runPair{generic.RunCtxswVoluntary, fmt.Sprintf("%d", rep.Rusage.Nvcsw)},
			runPair{generic.RunCtxswInvoluntary, fmt.Sprintf("%d", rep.Rusage.Nivcsw)},
		)
	}
	if migrations, ok := telemetry.CountMigrations(rep.Counters); ok {
		pairs = append(pairs, runPair{generic.RunMigrations, strconv.Itoa(migrations)})
	}
	if runDelay, ok := telemetry.CountRunqueue(rep.Counters); ok {
		pairs = append(pairs, runPair{generic.SchedRunDelay, FormatTime(time.Duration(runDelay))})
	}
	return pairs
}

func steerPairs(rep Report) []runPair {
	if rep.Steer == nil {
		return nil
	}
	return []runPair{
		{generic.RunIRQSteerApplied, strconv.Itoa(rep.Steer.Applied)},
		{generic.RunIRQSteerRejected, strconv.Itoa(rep.Steer.Rejected)},
		{generic.RunIRQSteerRemaining, strconv.Itoa(len(rep.Steer.Remaining))},
		{generic.RunIRQSteerDrift, strconv.Itoa(telemetry.CountDriftedIRQs(rep.Counters))},
		{generic.RunIRQBalanceHeld, rep.Steer.Status()},
	}
}

func TopologyTable(topo *cpu.Topology) Table {
	header := []string{"cpu", "core", "numa", "isol", "nohz", "rcu", "driver", "gov", "epp", "minf", "maxf"}
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
			fmt.Sprintf("%4d", topo.Numa[id]),
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
