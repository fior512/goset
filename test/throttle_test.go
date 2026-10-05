package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"goset/internal/generic"
	"goset/internal/telemetry"
)

// writeThrottle writes cpu's core_throttle_count; a nil content makes the path a directory, an unreadable file.
func writeThrottle(t *testing.T, root string, cpu int, content []byte) {
	t.Helper()
	dir := filepath.Join(root, fmt.Sprintf("cpu%d", cpu), "thermal_throttle")
	path := filepath.Join(dir, "core_throttle_count")
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if content == nil {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestThrottleReportsEveryBookedCPU(t *testing.T) {
	root := t.TempDir()
	const absent, counted, unreadable, malformed, lostAtStop = 0, 1, 2, 3, 4
	writeThrottle(t, root, counted, []byte("7\n"))
	writeThrottle(t, root, unreadable, nil)
	writeThrottle(t, root, malformed, []byte("x\n"))
	writeThrottle(t, root, lostAtStop, []byte("5\n"))

	var cpus generic.CPUSet
	for _, cpu := range []int{absent, counted, unreadable, malformed, lostAtStop} {
		cpus.SetBit(cpu)
	}
	src := &telemetry.ThrottleSource{Root: root}
	if err := src.Baseline(cpus); err != nil {
		t.Fatal(err)
	}
	writeThrottle(t, root, counted, []byte("9\n"))
	writeThrottle(t, root, lostAtStop, nil)
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}

	got := map[int]float64{}
	for _, counter := range src.Summary() {
		got[counter.CPU] = counter.Value
	}
	want := map[int]float64{absent: 0, counted: 2, unreadable: 0, malformed: 0, lostAtStop: 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("throttle per cpu = %v, want %v: an absent counter and an unknown one both read 0", got, want)
	}
}
