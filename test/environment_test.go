package integration

import (
	"os"
	"path/filepath"
	"testing"

	"goset/internal/cpu"
)

func writeFixture(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadBoostUnavailable(t *testing.T) {
	dir := t.TempDir()

	if got, want := cpu.ReadBoost(dir), "n/a"; got != want {
		t.Errorf("ReadBoost(missing) = %q, want %q", got, want)
	}

	writeFixture(t, filepath.Join(dir, "cpufreq/boost"), "")
	if got, want := cpu.ReadBoost(dir), "n/a"; got != want {
		t.Errorf("ReadBoost(empty) = %q, want %q", got, want)
	}
}

func TestReadBoostValues(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "intel_pstate/no_turbo"), "0\n")
	if got, want := cpu.ReadBoost(dir), "on"; got != want {
		t.Errorf("ReadBoost(no_turbo=0) = %q, want %q", got, want)
	}

	writeFixture(t, filepath.Join(dir, "cpufreq/boost"), "1\n")
	if got, want := cpu.ReadBoost(dir), "on"; got != want {
		t.Errorf("ReadBoost(boost=1) = %q, want %q", got, want)
	}

	writeFixture(t, filepath.Join(dir, "cpufreq/boost"), "0\n")
	if got, want := cpu.ReadBoost(dir), "off"; got != want {
		t.Errorf("ReadBoost(boost=0) = %q, want %q", got, want)
	}
}

func TestCountMitigationsUnavailable(t *testing.T) {
	dir := t.TempDir()

	if count := cpu.CountMitigations(filepath.Join(dir, "missing")); count != nil {
		t.Errorf("CountMitigations(missing) = %d, want nil", *count)
	}

	if err := os.Mkdir(filepath.Join(dir, "unreadable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if count := cpu.CountMitigations(dir); count != nil {
		t.Errorf("CountMitigations(unreadable entry) = %d, want nil", *count)
	}
}

func TestCountMitigationsCounts(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "spectre_v1"), "Not affected\n")
	writeFixture(t, filepath.Join(dir, "spectre_v2"), "Mitigation: usercall/swap\n")
	writeFixture(t, filepath.Join(dir, "meltdown"), "Mitigation: PTI\n")

	count := cpu.CountMitigations(dir)
	if count == nil || *count != 2 {
		t.Errorf("CountMitigations(mixed) = %v, want 2", count)
	}
}
