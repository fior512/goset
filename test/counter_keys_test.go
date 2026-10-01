package integration

import (
	"reflect"
	"testing"

	"goset/internal/generic"
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
		t.Errorf("footer = %q, want %q: a column missing a cpu is not reduced", rows, footer)
	}
}
