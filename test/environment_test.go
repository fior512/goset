package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goset/internal/cpu"
	"goset/internal/report"
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

func TestEnvironmentTableRendersUnavailable(t *testing.T) {
	smt := "on"
	mitigations := 3
	env := &cpu.Environment{Boost: "n/a", SMT: &smt, Mitigations: &mitigations}
	want := map[string]string{
		"smt":            "on",
		"boost":          "n/a",
		"numa_balancing": "unavailable",
		"nmi_watchdog":   "unavailable",
		"thp":            "unavailable",
		"mitigations":    "3",
	}
	for _, row := range report.EnvironmentTable(env).Rows {
		if got := row[1]; got != want[row[0]] {
			t.Errorf("row %q = %q, want %q", row[0], got, want[row[0]])
		}
	}
}

func environmentValue(t *testing.T, stdout string, key string) string {
	t.Helper()
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+" ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, key))
		}
	}
	t.Fatalf("environment row %q not found in output:\n%s", key, stdout)
	return ""
}

func TestDiagnoseEnvironmentHasNoBlankValues(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin)
	if res.exitCode != 0 {
		t.Fatalf("bare goset exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	for _, key := range []string{"smt", "boost", "numa_balancing", "nmi_watchdog", "thp", "mitigations"} {
		if value := environmentValue(t, res.stdout, key); value == "" {
			t.Errorf("environment row %q is blank, want a value or unavailable", key)
		}
	}
}
