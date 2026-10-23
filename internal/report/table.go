package report

import (
	"fmt"
	"slices"
	"strconv"
	"syscall"
	"time"

	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
	"goset/internal/telemetry"
)

func SelectionTable(selection *generic.Selection) Table {
	header := []string{"cpu", "sel", "steer", "non-steer", "core", "sibl", "isol", "numa", "nohz", "rcu"}
	rows := make([][]string, 0, len(selection.Scores))
	for _, candidate := range selection.Scores {
		mark := ""
		switch {
		case candidate.CPU == selection.HouseKeeper:
			mark = "& "
		case selection.Task.GetBit(candidate.CPU):
			mark = "* "
		case selection.Fence.GetBit(candidate.CPU):
			mark = "! "
		}
		sibling := "-"
		if candidate.Sibling >= 0 {
			sibling = strconv.Itoa(candidate.Sibling)
		}
		rows = append(rows, []string{
			fmt.Sprintf("%3d", candidate.CPU),
			mark,
			FormatValue(float64(candidate.Steerable)),
			FormatValue(float64(candidate.NonSteerable)),
			FormatValue(float64(candidate.Noise + candidate.SiblingLoad)),
			sibling,
			yesMark(candidate.KernelIsol),
			fmt.Sprintf("%4d", candidate.Numa),
			yesMark(candidate.NohzFull),
			yesMark(candidate.RcuNocb),
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
	Errors   []error // distinct telemetry failures, the counters they left at 0
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
	{generic.SourceFreq, generic.FreqAvg, FormatFreq, generic.Avg},
	{generic.SourceFreq, generic.FreqMax, FormatFreq, slices.Max[[]float64]},
	{generic.SourceThrottle, generic.ThrottleCount, FormatValue, generic.Sum},
	{generic.SourceIRQ, generic.IRQSteerable, FormatValue, generic.Sum},
	{generic.SourceIRQ, generic.IRQNonSteerable, FormatValue, generic.Sum},
}

func TelemetryTables(report Report, width int) []Table {
	values := perCPUValues(report.Counters)

	/* layout */
	keys := 0
	table := Table{Title: "Telemetry", Align: AlignRight}
	var groups []string
	if report.Cpus.Count() > 1 {
		keys = 1
		table.Align = AlignLabel
		table.Header = append(table.Header, generic.TelemetryCPU)
		groups = append(groups, "")
	}
	for _, column := range telemetryColumns {
		table.Header = append(table.Header, column.label())
		groups = append(groups, column.source)
	}

	/* rows */
	for cpu := range report.Cpus.All() {
		table.Rows = append(table.Rows, telemetryRow(cpu, telemetryColumns, values, keys > 0))
	}
	if keys > 0 {
		table.Rows = append(table.Rows, telemetryFooter(values, report.Cpus))
	}
	return splitColumns(table, groups, keys, width)
}

// one counter per column, "-" where the cpu reports none
func telemetryRow(cpu int, columns []telemetryColumn, values map[string]map[int]float64, keys bool) []string {
	var row []string
	if keys {
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
	return row
}

// one all-cpu row per table, each column folded over the task cpus
func telemetryFooter(values map[string]map[int]float64, cpus generic.CPUSet) []string {
	footer := []string{generic.TelemetryAll}
	for _, column := range telemetryColumns {
		cell, ok := column.reduceAll(values[column.label()], cpus)
		if !ok {
			cell = "-"
		}
		footer = append(footer, cell)
	}
	return footer
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

func splitColumns(table Table, groups []string, keys int, width int) []Table {
	widths := columnWidths(table)
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
		part := Table{Title: table.Title, Header: pick(table.Header, columns), Align: table.Align}
		for _, row := range table.Rows {
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

type runPair struct {
	key   string
	value string
}

// the scope name heads its own counters
type runScope struct {
	name  string
	pairs []runPair
}

func RunTable(report Report) Table {
	scopes := runScopes(report)
	table := Table{Title: "Run", Align: AlignPairs}
	for _, scope := range scopes {
		table.Header = append(table.Header, scope.name, "")
	}
	lines := 0
	for _, scope := range scopes {
		lines = max(lines, len(scope.pairs))
	}
	for line := 0; line < lines; line++ {
		row := make([]string, 0, 2*len(scopes))
		for _, scope := range scopes {
			pair := runPair{}
			if line < len(scope.pairs) {
				pair = scope.pairs[line]
			}
			row = append(row, pair.key, pair.value)
		}
		table.Rows = append(table.Rows, row)
	}
	return table
}

// a scope with no counter is dropped
func runScopes(report Report) []runScope {
	var scopes []runScope
	for _, scope := range []runScope{
		{generic.ScopeTask, taskPairs(report)},
		{generic.ScopeSched, schedPairs(report)},
		{generic.ScopeSteer, steerPairs(report)},
	} {
		if len(scope.pairs) > 0 {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

func taskPairs(report Report) []runPair {
	pairs := []runPair{
		{generic.RunPoll, fmt.Sprintf("%d@%s", report.Polls, report.Interval)},
		{generic.RunWall, FormatTime(report.Wall)},
	}
	if report.Rusage != nil {
		cpuTime := report.Rusage.Utime.Nano() + report.Rusage.Stime.Nano()
		pairs = append(pairs, runPair{generic.RunUtil, fmt.Sprintf("%.0f%%", 100*float64(cpuTime)/float64(report.Wall))})
	}
	return append(pairs, runPair{generic.RunExit, strconv.Itoa(report.ExitCode)})
}

func schedPairs(report Report) []runPair {
	var pairs []runPair
	if report.Rusage != nil {
		pairs = append(pairs,
			runPair{generic.RunCtxswVol, fmt.Sprintf("%d", report.Rusage.Nvcsw)},
			runPair{generic.RunCtxswInvol, fmt.Sprintf("%d", report.Rusage.Nivcsw)},
		)
	}
	pairs = append(pairs,
		runPair{generic.RunMigrations, strconv.Itoa(telemetry.CountMigrations(report.Counters))},
		runPair{generic.SchedRunDelay, FormatTime(time.Duration(telemetry.CountRunqueue(report.Counters)))},
	)
	return pairs
}

func steerPairs(report Report) []runPair {
	if report.Steer == nil {
		return nil
	}
	return []runPair{
		{generic.RunIRQSteerApplied, strconv.Itoa(report.Steer.Applied)},
		{generic.RunIRQSteerRejected, strconv.Itoa(report.Steer.Rejected)},
		{generic.RunIRQSteerRemaining, strconv.Itoa(len(report.Steer.Remaining))},
		{generic.RunIRQSteerDrift, strconv.Itoa(telemetry.CountDriftedIRQs(report.Counters))},
		{generic.RunIRQBalance, report.Steer.Status()},
	}
}

// one row per distinct telemetry failure, naming the counters it left at 0
func ErrorsTable(report Report) Table {
	rows := make([][]string, 0, len(report.Errors))
	for _, failure := range report.Errors {
		rows = append(rows, []string{failure.Error()})
	}
	return Table{Title: "Errors", Align: AlignLeft, Rows: rows}
}

func TopologyTable(topology *cpu.Topology) Table {
	header := []string{"cpu", "core", "numa", "isol", "nohz", "rcu", "driver", "gov", "epp", "minf", "maxf"}
	var rows [][]string
	for id := range topology.Online.All() {
		rows = append(rows, []string{
			fmt.Sprintf("%3d", id),
			fmt.Sprintf("%4d", topology.Core[id]),
			fmt.Sprintf("%4d", topology.Numa[id]),
			yesMark(topology.KernelIsol.GetBit(id)),
			yesMark(topology.NohzFull.GetBit(id)),
			yesMark(topology.RcuNocb.GetBit(id)),
			topology.Driver[id],
			topology.Governor[id],
			topology.EPP[id],
			FormatFreq(float64(topology.MinFreq[id]) * 1000),
			FormatFreq(float64(topology.MaxFreq[id]) * 1000),
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

func yesMark(on bool) string {
	if on {
		return "y"
	}
	return ""
}
