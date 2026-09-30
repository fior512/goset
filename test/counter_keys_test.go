package integration

import (
	"reflect"
	"syscall"
	"testing"
	"time"

	"goset/internal/generic"
	"goset/internal/isolation"
	"goset/internal/report"
	"goset/internal/telemetry"
)

func cpuSet(cpus ...int) generic.CPUSet {
	var set generic.CPUSet
	for _, cpu := range cpus {
		set.SetBit(cpu)
	}
	return set
}

func perCPUCounters(cpus ...int) []telemetry.Counter {
	var counters []telemetry.Counter
	for idx, cpu := range cpus {
		base := float64(idx + 1)
		counters = append(counters,
			telemetry.Counter{Source: generic.SourceFreq, CPU: cpu, Name: generic.FreqMin, Value: base * 1e9},
			telemetry.Counter{Source: generic.SourceFreq, CPU: cpu, Name: generic.FreqAvg, Value: base * 2e9},
			telemetry.Counter{Source: generic.SourceFreq, CPU: cpu, Name: generic.FreqMax, Value: base * 3e9},
			telemetry.Counter{Source: generic.SourceIRQ, CPU: cpu, Name: generic.IRQSteerable, Value: base},
			telemetry.Counter{Source: generic.SourceIRQ, CPU: cpu, Name: generic.IRQNonSteerable, Value: base * 10},
		)
	}
	return counters
}

func TestRunTableRows(t *testing.T) {
	rep := report.Report{
		Counters: []telemetry.Counter{
			{Source: generic.SourceIRQSteer, CPU: -1, Name: generic.IRQSteerDrift, Value: 2},
			{Source: generic.SourceSched, CPU: -1, Name: generic.SchedMigrations, Value: 3},
			{Source: generic.SourceSched, CPU: -1, Name: generic.SchedRunDelay, Value: 4},
		},
		Steer:    &isolation.Steering{Applied: 1, Rejected: 2, Remaining: []string{"NMI"}},
		Rusage:   &syscall.Rusage{Nvcsw: 5, Nivcsw: 6},
		Wall:     time.Second,
		Polls:    19,
		Interval: 100 * time.Millisecond,
	}
	want := [][]string{
		{"task", "wall", "1.00s", "exit", "0", "samples", "19@100ms"},
		{"sched", "ctxsw voluntary", "5", "ctxsw involuntary", "6", "migrations", "3"},
		{"", "run_delay", "4ns"},
		{"steer", "applied", "1", "rejected", "2", "remaining", "1"},
		{"", "drift", "2", "irqbalance", "absent"},
	}
	if got := report.RunTable(rep).Rows; !reflect.DeepEqual(got, want) {
		t.Errorf("RunTable rows =\n%q\nwant\n%q", got, want)
	}
}

func TestRunTableOptionalScopes(t *testing.T) {
	want := [][]string{{"task", "wall", "1.00s", "exit", "0", "samples", "0@0s"}}
	if got := report.RunTable(report.Report{Wall: time.Second}).Rows; !reflect.DeepEqual(got, want) {
		t.Errorf("RunTable(no steer, no rusage) rows = %q, want %q", got, want)
	}
}

func TestTelemetryTablesSingleCPU(t *testing.T) {
	tables := report.TelemetryTables(report.Report{Cpus: cpuSet(9), Counters: perCPUCounters(9)}, 80)
	if len(tables) != 1 {
		t.Fatalf("TelemetryTables blocks = %d, want 1", len(tables))
	}
	wantHeader := []string{"freq min", "freq avg", "freq max", "irq steerable", "irq non-steerable"}
	if !reflect.DeepEqual(tables[0].Header, wantHeader) {
		t.Errorf("header = %q, want %q: no cpu column at -n 1", tables[0].Header, wantHeader)
	}
	wantRows := [][]string{{"1GHz", "2GHz", "3GHz", "1", "10"}}
	if !reflect.DeepEqual(tables[0].Rows, wantRows) {
		t.Errorf("rows = %q, want %q: no footer at -n 1", tables[0].Rows, wantRows)
	}
}

func TestTelemetryTablesFooterReducesPerColumn(t *testing.T) {
	tables := report.TelemetryTables(report.Report{Cpus: cpuSet(3, 7), Counters: perCPUCounters(3, 7)}, 200)
	if len(tables) != 1 {
		t.Fatalf("TelemetryTables blocks = %d, want 1", len(tables))
	}
	want := [][]string{
		{"3", "1GHz", "2GHz", "3GHz", "1", "10"},
		{"7", "2GHz", "4GHz", "6GHz", "2", "20"},
		{generic.TelemetryAll, "1GHz", "3GHz", "6GHz", "3", "30"}, // min, avg, max, sum, sum
	}
	if !reflect.DeepEqual(tables[0].Rows, want) {
		t.Errorf("rows =\n%q\nwant\n%q", tables[0].Rows, want)
	}
}

func TestTelemetryTablesSplitKeepsGroupsAndCPUColumn(t *testing.T) {
	tables := report.TelemetryTables(report.Report{Cpus: cpuSet(3, 7), Counters: perCPUCounters(3, 7)}, 40)
	want := [][]string{
		{generic.TelemetryCPU, "freq min", "freq avg", "freq max"},
		{generic.TelemetryCPU, "irq steerable", "irq non-steerable"},
	}
	if len(tables) != len(want) {
		t.Fatalf("TelemetryTables blocks = %d, want %d", len(tables), len(want))
	}
	for idx, header := range want {
		if !reflect.DeepEqual(tables[idx].Header, header) {
			t.Errorf("block %d header = %q, want %q", idx, tables[idx].Header, header)
		}
		if tables[idx].Title != "Telemetry" {
			t.Errorf("block %d title = %q, want %q: a split repeats the title", idx, tables[idx].Title, "Telemetry")
		}
	}
}

func TestTelemetryTablesRowPerBookedCPU(t *testing.T) {
	tables := report.TelemetryTables(report.Report{Cpus: cpuSet(3, 7), Counters: perCPUCounters(3)}, 200)
	if len(tables) != 1 {
		t.Fatalf("TelemetryTables blocks = %d, want 1", len(tables))
	}
	want := []string{"7", "-", "-", "-", "-", "-"}
	if rows := tables[0].Rows; len(rows) != 3 || !reflect.DeepEqual(rows[1], want) {
		t.Errorf("rows = %q, want cpu 7 as %q: a booked cpu without values keeps its row", rows, want)
	}
	footer := []string{generic.TelemetryAll, "-", "-", "-", "-", "-"}
	if rows := tables[0].Rows; !reflect.DeepEqual(rows[2], footer) {
		t.Errorf("footer = %q, want %q: a column missing a cpu is not reduced", rows[2], footer)
	}
}

func TestNotReportedTableNamesMissingSources(t *testing.T) {
	rows := report.NotReportedTable(report.Report{Cpus: cpuSet(3), Counters: perCPUCounters(3)}).Rows
	want := [][]string{{"not reported: " + generic.SourceThrottle}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("NotReportedTable rows = %q, want %q", rows, want)
	}
}

func TestCounterLabelRunGlobal(t *testing.T) {
	counters := []telemetry.Counter{
		{Source: generic.SourceSched, Name: generic.SchedRunDelay},
		{Source: generic.SourceSched, Name: generic.SchedMigrations},
		{Source: generic.SourceIRQSteer, Name: generic.IRQSteerDrift},
	}
	want := []string{"sched run_delay", "sched nr_migrations", "irq steer drift"}
	for idx, counter := range counters {
		if got := counter.Label(); got != want[idx] {
			t.Errorf("Counter.Label() = %q, want %q", got, want[idx])
		}
	}
}
