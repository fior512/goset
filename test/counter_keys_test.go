package integration

import (
	"syscall"
	"testing"
	"time"

	"goset/internal/generic"
	"goset/internal/isolation"
	"goset/internal/report"
	"goset/internal/telemetry"
)

func TestGlobalTableKeys(t *testing.T) {
	rep := report.Report{
		Counters: []telemetry.Counter{
			{Source: telemetry.SourceIRQSteer, CPU: -1, Name: telemetry.IRQSteerDrift, Value: 2},
			{Source: telemetry.SourceSched, CPU: -1, Name: telemetry.SchedMigrations, Value: 3},
			{Source: telemetry.SourceSched, CPU: -1, Name: telemetry.SchedRunDelay, Value: 4},
		},
		Steer:    &isolation.Steering{Applied: 1, Rejected: 2, Remaining: []string{"NMI"}},
		Rusage:   &syscall.Rusage{Nvcsw: 5, Nivcsw: 6},
		Wall:     time.Second,
		ExitCode: 0,
	}
	want := []string{
		"irqbalance held",
		"irq steer applied",
		"irq steer rejected",
		"irq steer remaining",
		"irq steer drift",
		"ctxsw voluntary",
		"ctxsw involuntary",
		"migrations",
		"run_delay",
		"wall",
		"exit",
	}
	rows := report.GlobalTable(rep).Rows
	if len(rows) != len(want) {
		t.Fatalf("GlobalTable rows = %d, want %d: %v", len(rows), len(want), rows)
	}
	for idx, key := range want {
		if got := rows[idx][0]; got != key {
			t.Errorf("GlobalTable row %d key = %q, want %q", idx, got, key)
		}
	}
}

func TestGlobalTableOptionalRows(t *testing.T) {
	want := []string{generic.GlobalWall, generic.GlobalExit}
	rows := report.GlobalTable(report.Report{Wall: time.Second}).Rows
	if len(rows) != len(want) {
		t.Fatalf("GlobalTable(no steer, no rusage) rows = %d, want %d: %v", len(rows), len(want), rows)
	}
	for idx, key := range want {
		if got := rows[idx][0]; got != key {
			t.Errorf("GlobalTable row %d key = %q, want %q", idx, got, key)
		}
	}
}

func TestTelemetryTableLabels(t *testing.T) {
	rep := report.Report{Counters: []telemetry.Counter{
		{Source: telemetry.SourceIRQ, CPU: 0, Name: telemetry.IRQSoft, Value: 1},
		{Source: telemetry.SourceIRQ, CPU: 0, Name: telemetry.IRQHard, Value: 2},
		{Source: telemetry.SourceThrottle, CPU: 0, Name: telemetry.ThrottleCount, Value: 3},
		{Source: telemetry.SourceFreq, CPU: 0, Name: telemetry.FreqMin, Value: 4},
		{Source: telemetry.SourceFreq, CPU: 0, Name: telemetry.FreqMax, Value: 5},
		{Source: telemetry.SourceFreq, CPU: 0, Name: telemetry.FreqAvg, Value: 6},
		{Source: telemetry.SourceIRQSteer, CPU: -1, Name: telemetry.IRQSteerDrift, Value: 7},
	}}
	want := []string{"irq soft", "irq hard", "throttle count", "freq min MHz", "freq max MHz", "freq avg MHz"}
	rows := report.TelemetryTable(rep).Rows
	if len(rows) != len(want) {
		t.Fatalf("TelemetryTable rows = %d, want %d: %v", len(rows), len(want), rows)
	}
	for idx, label := range want {
		if got := rows[idx][0]; got != label {
			t.Errorf("TelemetryTable row %d label = %q, want %q", idx, got, label)
		}
	}
}

func TestCounterLabelRunGlobal(t *testing.T) {
	counters := []telemetry.Counter{
		{Source: telemetry.SourceSched, Name: telemetry.SchedRunDelay},
		{Source: telemetry.SourceSched, Name: telemetry.SchedMigrations},
		{Source: telemetry.SourceIRQSteer, Name: telemetry.IRQSteerDrift},
	}
	want := []string{"sched run_delay", "sched nr_migrations", "irq steer drift"}
	for idx, counter := range counters {
		if got := counter.Label(); got != want[idx] {
			t.Errorf("Counter.Label() = %q, want %q", got, want[idx])
		}
	}
}
