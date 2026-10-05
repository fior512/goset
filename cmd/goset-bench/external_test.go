package main

import (
	"bytes"
	"flag"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"goset/benchmark/plugin"
	"goset/internal/generic"
	"goset/internal/isolation"
	rpt "goset/internal/report"
	"goset/internal/telemetry"
)

func TestParseLineBareFloat(t *testing.T) {
	parsed := parseLine("0.31")
	if got, want := joinMask(parsed.mask), "N"; got != want {
		t.Errorf("mask = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(parsed.values, []float64{0.31}) {
		t.Errorf("values = %v, want [0.31]", parsed.values)
	}
	if !reflect.DeepEqual(parsed.units, []string{""}) {
		t.Errorf("units = %q, want one empty", parsed.units)
	}
	if !reflect.DeepEqual(parsed.cols, []int{0}) {
		t.Errorf("cols = %v, want [0]", parsed.cols)
	}
	if parsed.displays[0] != "" {
		t.Errorf("display = %q, want empty so Metrics falls back to the name", parsed.displays[0])
	}
}

func TestParseLineKeyValue(t *testing.T) {
	parsed := parseLine("bw 10.5 MB/s")
	if got, want := joinMask(parsed.mask), "bw N"; got != want {
		t.Errorf("mask = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(parsed.values, []float64{10.5}) {
		t.Errorf("values = %v, want raw [10.5]", parsed.values)
	}
	if !reflect.DeepEqual(parsed.units, []string{"MB/s"}) {
		t.Errorf("units = %q, want [MB/s]", parsed.units)
	}
	if parsed.displays[0] != "bw MB/s" {
		t.Errorf("display = %q, want %q", parsed.displays[0], "bw MB/s")
	}
}

func TestParseLineKeyEqualsValue(t *testing.T) {
	parsed := parseLine("size = 1000")
	if got, want := joinMask(parsed.mask), "size = N"; got != want {
		t.Errorf("mask = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(parsed.values, []float64{1000}) {
		t.Errorf("values = %v, want [1000]", parsed.values)
	}
	if parsed.displays[0] != "size =" {
		t.Errorf("display = %q, want %q", parsed.displays[0], "size =")
	}
}

func TestParseLineAttachedUnit(t *testing.T) {
	parsed := parseLine("lat 831.4ns")
	if !reflect.DeepEqual(parsed.units, []string{"ns"}) {
		t.Errorf("units = %q, want [ns]", parsed.units)
	}
	if !reflect.DeepEqual(parsed.values, []float64{831.4}) {
		t.Errorf("values = %v, want raw [831.4], no ns scaling", parsed.values)
	}
	if parsed.displays[0] != "lat ns" {
		t.Errorf("display = %q, want %q", parsed.displays[0], "lat ns")
	}
}

func TestParseLineDetachedUnitBindsToPrecedingValue(t *testing.T) {
	parsed := parseLine("lat 0.83 us")
	if !reflect.DeepEqual(parsed.units, []string{"us"}) {
		t.Errorf("units = %q, want [us]", parsed.units)
	}
	if !reflect.DeepEqual(parsed.values, []float64{0.83}) {
		t.Errorf("values = %v, want raw [0.83], no ns scaling", parsed.values)
	}
	if got, want := joinMask(parsed.mask), "lat N"; got != want {
		t.Errorf("mask = %q, want %q: the bound unit leaves the mask", got, want)
	}
}

func TestParseLineUnicodeUnit(t *testing.T) {
	parsed := parseLine("lat 5 \u00b5s")
	if !reflect.DeepEqual(parsed.units, []string{"\u00b5s"}) {
		t.Errorf("units = %q, want the microsecond unit", parsed.units)
	}
	if parsed.displays[0] != "lat \u00b5s" {
		t.Errorf("display = %q, want %q", parsed.displays[0], "lat \u00b5s")
	}
}

func TestParseLineMultiColumnRow(t *testing.T) {
	parsed := parseLine("iter 12 831.4 ns")
	if got, want := joinMask(parsed.mask), "iter N N"; got != want {
		t.Errorf("mask = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(parsed.cols, []int{0, 1}) {
		t.Errorf("cols = %v, want [0 1]", parsed.cols)
	}
	if !reflect.DeepEqual(parsed.units, []string{"", "ns"}) {
		t.Errorf("units = %q, want the counter bare and ns bound to the value", parsed.units)
	}
	if parsed.displays[0] != "iter" {
		t.Errorf("display[0] = %q, want %q", parsed.displays[0], "iter")
	}
	if parsed.displays[1] != "iter ns" {
		t.Errorf("display[1] = %q, want %q", parsed.displays[1], "iter ns")
	}
}

func TestParseLineColonKeepsRowName(t *testing.T) {
	parsed := parseLine("foo : 1.5")
	if got, want := joinMask(parsed.mask), "foo : N"; got != want {
		t.Errorf("mask = %q, want %q", got, want)
	}
	if parsed.displays[0] != "foo :" {
		t.Errorf("display = %q, want %q", parsed.displays[0], "foo :")
	}
}

func TestParseLineSimdjson(t *testing.T) {
	parsed := parseLine("string_builder : 0.00 ns 1.15 GB/s 5.26 GHz 4.58 cycles/char 13.53 ins./char 2.96 i/c")
	if got, want := joinMask(parsed.mask), "string_builder : N N N N N N"; got != want {
		t.Errorf("mask = %q, want %q", got, want)
	}
	wantValues := []float64{0.00, 1.15, 5.26, 4.58, 13.53, 2.96}
	if !reflect.DeepEqual(parsed.values, wantValues) {
		t.Errorf("values = %v, want %v", parsed.values, wantValues)
	}
	wantUnits := []string{"ns", "GB/s", "GHz", "cycles/char", "ins./char", "i/c"}
	if !reflect.DeepEqual(parsed.units, wantUnits) {
		t.Errorf("units = %q, want %q", parsed.units, wantUnits)
	}
	if !reflect.DeepEqual(parsed.cols, []int{0, 1, 2, 3, 4, 5}) {
		t.Errorf("cols = %v, want [0 1 2 3 4 5]", parsed.cols)
	}
	wantDisplays := []string{
		"string_builder : ns",
		"string_builder : GB/s",
		"string_builder : GHz",
		"string_builder : cycles/char",
		"string_builder : ins./char",
		"string_builder : i/c",
	}
	if !reflect.DeepEqual(parsed.displays, wantDisplays) {
		t.Errorf("displays = %q, want %q", parsed.displays, wantDisplays)
	}
}

func TestParseLineLabelOnly(t *testing.T) {
	parsed := parseLine("building the document")
	if len(parsed.values) != 0 {
		t.Errorf("values = %v, want none", parsed.values)
	}
}

func TestParseLineProgressLineKeepsLastSegment(t *testing.T) {
	bench := &externalCmd{}
	metrics := bench.Metrics([]byte("copied 1 MiB\rcopied 2 MiB\rcopied 3 MiB\n"))
	if len(metrics) != 1 {
		t.Fatalf("metrics = %d, want 1: only the last segment counts", len(metrics))
	}
	if !reflect.DeepEqual(metrics[0].Samples, []float64{3}) {
		t.Errorf("samples = %v, want [3]", metrics[0].Samples)
	}
}

func TestMetricsUnitSplitSeries(t *testing.T) {
	bench := &externalCmd{}
	metrics := bench.Metrics([]byte("lat 831ns\nlat 0.83us\n"))
	if len(metrics) != 2 {
		t.Fatalf("metrics = %d, want 2: the unit belongs to the alignment identity", len(metrics))
	}
	byName := map[string]plugin.Metric{}
	for _, m := range metrics {
		byName[m.Name] = m
	}
	nsMetric, ok := byName["lat N #0 ns"]
	if !ok {
		t.Fatalf("names = %v, want an ns series", metricNames(metrics))
	}
	if !reflect.DeepEqual(nsMetric.Samples, []float64{831}) {
		t.Errorf("ns samples = %v, want raw [831]", nsMetric.Samples)
	}
	usMetric, ok := byName["lat N #0 us"]
	if !ok {
		t.Fatalf("names = %v, want a us series", metricNames(metrics))
	}
	if !reflect.DeepEqual(usMetric.Samples, []float64{0.83}) {
		t.Errorf("us samples = %v, want raw [0.83]", usMetric.Samples)
	}
}

func TestMetricsGroupsTableRows(t *testing.T) {
	bench := &externalCmd{}
	metrics := bench.Metrics([]byte("iter 0 100.5 ns\niter 1 101.2 ns\niter 2 100.8 ns\n"))
	byName := map[string]plugin.Metric{}
	for _, m := range metrics {
		byName[m.Name] = m
	}
	counter, ok := byName["iter N N #0"]
	if !ok {
		t.Fatalf("names = %v, want the counter series", metricNames(metrics))
	}
	if !reflect.DeepEqual(counter.Samples, []float64{0, 1, 2}) {
		t.Errorf("counter samples = %v, want [0 1 2]", counter.Samples)
	}
	values, ok := byName["iter N N #1 ns"]
	if !ok {
		t.Fatalf("names = %v, want the value series", metricNames(metrics))
	}
	if !reflect.DeepEqual(values.Samples, []float64{100.5, 101.2, 100.8}) {
		t.Errorf("value samples = %v", values.Samples)
	}
	if values.Value != 100.8 {
		t.Errorf("value = %v, want the median 100.8", values.Value)
	}
}

func TestMetricsFingerprintCountsShapes(t *testing.T) {
	bench := &externalCmd{}
	bench.Metrics([]byte("lat 831ns\nlat 0.83us\nlat 1.2ms\n"))
	fingerprint := bench.fingerprint()
	if fingerprint["lat N [ns]"] != 1 {
		t.Errorf("ns shape = %d, want 1: a unit flip is drift", fingerprint["lat N [ns]"])
	}
	if fingerprint["lat N [us]"] != 1 || fingerprint["lat N [ms]"] != 1 {
		t.Errorf("fingerprint = %v, want one line per unit", fingerprint)
	}
}

func TestMetricsSimdjson(t *testing.T) {
	bench := &externalCmd{}
	metrics := bench.Metrics([]byte("string_builder : 0.00 ns 1.15 GB/s 5.26 GHz 4.58 cycles/char 13.53 ins./char 2.96 i/c\n"))
	if len(metrics) != 6 {
		t.Fatalf("metrics = %d, want 6", len(metrics))
	}
	wantDisplays := []string{
		"string_builder : ns",
		"string_builder : GB/s",
		"string_builder : GHz",
		"string_builder : cycles/char",
		"string_builder : ins./char",
		"string_builder : i/c",
	}
	for i, m := range metrics {
		if m.Display != wantDisplays[i] {
			t.Errorf("display[%d] = %q, want %q", i, m.Display, wantDisplays[i])
		}
		if m.Col != i {
			t.Errorf("col[%d] = %d, want %d", i, m.Col, i)
		}
	}
}

func TestMetricsBareFloatFallsBackToName(t *testing.T) {
	bench := &externalCmd{}
	metrics := bench.Metrics([]byte("0.31\n0.62\n"))
	if len(metrics) != 1 {
		t.Fatalf("metrics = %d, want 1", len(metrics))
	}
	if metrics[0].Name != "N #0" || metrics[0].Display != "N #0" {
		t.Errorf("name/display = %q/%q, want N #0 for both", metrics[0].Name, metrics[0].Display)
	}
}

func TestMetricsCapKeepsBiggestSeries(t *testing.T) {
	var output []byte
	for line := 0; line < maxExtractedKeys+10; line++ {
		output = append(output, []byte("iter 0 5.0\n")...)
		output = append(output, []byte("row"+strconv.Itoa(line)+" value 1.0\n")...)
	}
	bench := &externalCmd{}
	metrics := bench.Metrics(output)
	if len(metrics) != maxExtractedKeys {
		t.Fatalf("metrics = %d, want the cap %d", len(metrics), maxExtractedKeys)
	}
	for _, m := range metrics {
		if m.Name == "iter N N #1" && len(m.Samples) == maxExtractedKeys+10 {
			return
		}
	}
	t.Fatalf("the largest series was evicted by the cap")
}

func TestParseNumFieldStrict(t *testing.T) {
	cases := []struct {
		field string
		ok    bool
		value float64
		unit  string
	}{
		{field: "42", ok: true, value: 42},
		{field: "-3.5", ok: true, value: -3.5},
		{field: "1.2e-05", ok: true, value: 1.2e-05},
		{field: "177MiB/s", ok: true, value: 177, unit: "MiB/s"},
		{field: "99.9%", ok: true, value: 99.9, unit: "%"},
		{field: "\u00b5s", ok: false},
		{field: "1,234", ok: false},
		{field: "1.2.3", ok: false},
		{field: "10-20", ok: false},
		{field: "", ok: false},
	}
	for _, c := range cases {
		value, unit, ok := parseNumField(c.field)
		if ok != c.ok {
			t.Errorf("parseNumField(%q) ok = %v, want %v", c.field, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if value != c.value || unit != c.unit {
			t.Errorf("parseNumField(%q) = %v,%q, want %v,%q", c.field, value, unit, c.value, c.unit)
		}
	}
}

func TestExternalCmdImplementsBenchmark(t *testing.T) {
	var bench plugin.Benchmark = &externalCmd{argv: []string{"./task", "arg"}}
	if bench.Name() != "external" {
		t.Errorf("name = %q, want external", bench.Name())
	}
	bin, err := bench.Build()
	if err != nil || bin != "./task" {
		t.Errorf("Build = %q,%v, want ./task,nil", bin, err)
	}
	if got := bench.Argv(bin); !reflect.DeepEqual(got, []string{"./task", "arg"}) {
		t.Errorf("Argv = %v", got)
	}
	bench.RegisterFlags(flag.NewFlagSet("test", flag.ContinueOnError))
}

func TestStructureWarnReportsCrossModeDivergence(t *testing.T) {
	baseline := []sample{{structure: map[string]int{"lat N [ns]": 1, "gone N []": 1}}}
	isolated := []sample{{structure: map[string]int{"lat N [us]": 1, "new N []": 2}}}

	restored := captureStderr(t, func() { structureWarn(baseline, isolated) })
	for _, want := range []string{`"gone N []"`, `"lat N [ns]"`, `"lat N [us]"`, `"new N []"`} {
		if !strings.Contains(restored, want) {
			t.Errorf("warning missing %s:\n%s", want, restored)
		}
	}
	lines := strings.Count(strings.TrimSpace(restored), "\n") + 1
	if lines != 4 {
		t.Errorf("warning lines = %d, want 4 (one per diverging shape):\n%s", lines, restored)
	}
}

func TestStructureWarnSilentWhenShapesMatch(t *testing.T) {
	same := map[string]int{"lat N [ns]": 1}
	baseline := []sample{{structure: same}}
	isolated := []sample{{structure: map[string]int{"lat N [ns]": 1}}}

	if got := captureStderr(t, func() { structureWarn(baseline, isolated) }); got != "" {
		t.Errorf("warning for matching shapes: %q", got)
	}
}

func TestStructureWarnSilentWithoutBothModes(t *testing.T) {
	one := []sample{{structure: map[string]int{"lat N [ns]": 1}}}
	if got := captureStderr(t, func() { structureWarn(one, nil) }); got != "" {
		t.Errorf("warning with no isolated runs: %q", got)
	}
	if got := captureStderr(t, func() { structureWarn(nil, one) }); got != "" {
		t.Errorf("warning with no baseline runs: %q", got)
	}
	if got := captureStderr(t, func() { structureWarn(one, one) }); got != "" {
		t.Errorf("warning for shared fingerprint pointer: %q", got)
	}
}

func captureStderr(t *testing.T, run func()) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = write
	run()
	os.Stderr = saved
	if err := write.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	out, err := io.ReadAll(read)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := read.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return string(out)
}

func joinMask(mask []string) string {
	out := ""
	for i, field := range mask {
		if i > 0 {
			out += " "
		}
		out += field
	}
	return out
}

func metricNames(metrics []plugin.Metric) []string {
	names := make([]string, 0, len(metrics))
	for _, m := range metrics {
		names = append(names, m.Name)
	}
	return names
}

func renderedReport(cpus []int, width int) []byte {
	var booked generic.CPUSet
	var counters []telemetry.Counter
	for idx, cpu := range cpus {
		booked.SetBit(cpu)
		counters = append(counters,
			telemetry.Counter{Source: generic.SourceFreq, CPU: cpu, Name: generic.FreqAvg, Value: 4.8e9},
			telemetry.Counter{Source: generic.SourceIRQ, CPU: cpu, Name: generic.IRQSteerable, Value: float64(idx + 1)},
			telemetry.Counter{Source: generic.SourceIRQ, CPU: cpu, Name: generic.IRQNonSteerable, Value: 1500},
		)
	}
	counters = append(counters, telemetry.Counter{Source: generic.SourceSched, CPU: -1, Name: generic.SchedRunDelay, Value: 41200})
	rep := rpt.Report{
		Cpus:     booked,
		Counters: counters,
		Steer:    &isolation.Steering{Applied: 40, Rejected: 26},
		Rusage:   &syscall.Rusage{Nvcsw: 812, Nivcsw: 9},
		Wall:     1990 * time.Millisecond,
		ExitCode: 3,
		Polls:    19,
		Interval: 100 * time.Millisecond,
	}
	var out bytes.Buffer
	out.WriteString("Run\ntask wall  9s\n" + gosetReportMarker + "\n")
	tables := rpt.TelemetryTables(rep, width)
	tables = append(tables, rpt.RunTable(rep))
	rpt.Render(&out, tables...)
	return out.Bytes()
}

func TestParseReportReadsSplitTelemetryAndRun(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cpus  []int
		width int
	}{
		{"one cpu", []int{9}, 200},
		{"two cpus split", []int{3, 7}, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed := parseReport(renderedReport(tc.cpus, tc.width))
			cpuCount := float64(len(tc.cpus))
			if parsed.wall != 1.99 {
				t.Errorf("wall = %v, want 1.99: the task output before the marker must be ignored", parsed.wall)
			}
			wantGlobal := map[string]float64{
				exitLabel: 3,
				runLabel(generic.ScopeSched, generic.RunCtxswVol):   812,
				runLabel(generic.ScopeSched, generic.RunCtxswInvol): 9,
				runDelayLabel: 41.2e-6,
				runLabel(generic.ScopeSteer, generic.RunIRQSteerApplied):  40,
				runLabel(generic.ScopeSteer, generic.RunIRQSteerRejected): 26,
			}
			for key, want := range wantGlobal {
				if got := parsed.global[key]; math.Abs(got-want) > 1e-12 {
					t.Errorf("global[%q] = %v, want %v", key, got, want)
				}
			}
			wantTelemetry := map[string]float64{
				"irq steerable":     cpuCount * (cpuCount + 1) / 2,
				"irq non-steerable": cpuCount * 1500,
			}
			for key, want := range wantTelemetry {
				if got := parsed.telemetry[key]; got != want {
					t.Errorf("telemetry[%q] = %v, want %v", key, got, want)
				}
			}
		})
	}
}
