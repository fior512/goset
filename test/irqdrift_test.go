package integration

import (
	"os"
	"path/filepath"
	"testing"

	"goset/internal/generic"
	"goset/internal/telemetry"
)

func writeIRQ(t *testing.T, root, label, affinity string) {
	t.Helper()
	dir := filepath.Join(root, label)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, generic.SmpAffinityList), []byte(affinity), 0o644); err != nil {
		t.Fatal(err)
	}
}

func driftCount(t *testing.T, src *telemetry.IRQDriftSource) int {
	t.Helper()
	var total float64
	for _, counter := range src.Summary() {
		total += counter.Value
	}
	return int(total)
}

func TestIRQDriftCountsChangedAffinity(t *testing.T) {
	root := t.TempDir()
	writeIRQ(t, root, "74", "0-3")
	src := &telemetry.IRQDriftSource{
		Root:     root,
		Expected: map[string]string{"74": "0-3"},
		Drifted:  map[string]bool{},
	}
	if err := src.Baseline(generic.CPUSet{}); err != nil {
		t.Fatal(err)
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if got := driftCount(t, src); got != 0 {
		t.Errorf("drift before change = %d, want 0", got)
	}

	writeIRQ(t, root, "74", "4-7")
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if got := driftCount(t, src); got != 1 {
		t.Errorf("drift after change = %d, want 1", got)
	}

	writeIRQ(t, root, "74", "0-3") // restored: drift stays sticky
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}
	if got := driftCount(t, src); got != 1 {
		t.Errorf("drift after restore = %d, want sticky 1", got)
	}
}

func TestIRQDriftCanonicalForm(t *testing.T) {
	root := t.TempDir()
	writeIRQ(t, root, "74", "0,1,2,3") // kernel may re-render "0-3"
	src := &telemetry.IRQDriftSource{
		Root:     root,
		Expected: map[string]string{"74": "0-3"},
		Drifted:  map[string]bool{},
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if got := driftCount(t, src); got != 0 {
		t.Errorf("drift for same set, different form = %d, want 0", got)
	}
}

func TestIRQDriftMissingFile(t *testing.T) {
	src := &telemetry.IRQDriftSource{
		Root:     t.TempDir(),
		Expected: map[string]string{"74": "0-3"},
		Drifted:  map[string]bool{},
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if got := driftCount(t, src); got != 1 {
		t.Errorf("drift for missing file = %d, want 1", got)
	}
}

func TestIRQDriftEmptyExpectedReportsZero(t *testing.T) {
	src := &telemetry.IRQDriftSource{
		Root:     t.TempDir(),
		Expected: nil,
		Drifted:  map[string]bool{},
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if got := driftCount(t, src); got != 0 {
		t.Errorf("drift with no IRQ steered = %d, want 0", got)
	}
}
