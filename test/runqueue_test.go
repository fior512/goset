package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"goset/internal/generic"
	"goset/internal/telemetry"
)

func writeSchedstat(t *testing.T, root string, pid, tid int, runDelay float64) {
	t.Helper()
	dir := filepath.Join(root, strconv.Itoa(pid), "task", strconv.Itoa(tid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("%d %d %d\n", int64(runDelay), int64(runDelay), 42)
	if err := os.WriteFile(filepath.Join(dir, "schedstat"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runDelayValue(t *testing.T, src *telemetry.RunqueueSource) (float64, bool) {
	t.Helper()
	return telemetry.CountRunqueue(src.Summary())
}

func TestRunqueueSumsTaskThreads(t *testing.T) {
	root := t.TempDir()
	const child = 4242
	writeChildren(t, root, child)
	writeSchedstat(t, root, child, child, 1000)
	writeSchedstat(t, root, child, child+1, 2500)

	src := &telemetry.RunqueueSource{Root: root}
	if err := src.Baseline(generic.CPUSet{}); err != nil {
		t.Fatal(err)
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	got, ok := runDelayValue(t, src)
	if !ok {
		t.Fatal("Summary reported no run_delay counter after a live child sample")
	}
	if got != 3500 {
		t.Errorf("run_delay = %v, want 3500 (1000+2500 over task threads)", got)
	}
}

func TestRunqueueStickyAfterChildVanishes(t *testing.T) {
	root := t.TempDir()
	const child = 4242
	writeChildren(t, root, child)
	writeSchedstat(t, root, child, child, 700)

	src := &telemetry.RunqueueSource{Root: root}
	if err := src.Baseline(generic.CPUSet{}); err != nil {
		t.Fatal(err)
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if got, _ := runDelayValue(t, src); got != 700 {
		t.Fatalf("run_delay = %v, want 700", got)
	}

	// child reaped: schedstat gone, Stop keeps last value
	if err := os.RemoveAll(filepath.Join(root, strconv.Itoa(child))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, strconv.Itoa(os.Getpid()), "task", strconv.Itoa(os.Getpid()), "children"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}
	got, ok := runDelayValue(t, src)
	if !ok {
		t.Fatal("Summary dropped the run_delay counter after child vanished")
	}
	if got != 700 {
		t.Errorf("run_delay = %v after reap, want sticky 700", got)
	}
}

func TestRunqueueNoChildNoCounter(t *testing.T) {
	src := &telemetry.RunqueueSource{Root: t.TempDir()}
	if err := src.Baseline(generic.CPUSet{}); err != nil {
		t.Fatal(err)
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, ok := runDelayValue(t, src); ok {
		t.Error("Summary reported a run_delay counter with no child ever sampled")
	}
}

func TestRunqueueMissingSchedstatFile(t *testing.T) {
	root := t.TempDir()
	const child = 4242
	writeChildren(t, root, child) // child listed but schedstat unreadable (mid-exit race)

	src := &telemetry.RunqueueSource{Root: root}
	if err := src.Baseline(generic.CPUSet{}); err != nil {
		t.Fatal(err)
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if _, ok := runDelayValue(t, src); ok {
		t.Error("Summary reported a run_delay counter when schedstat was unreadable")
	}
}
