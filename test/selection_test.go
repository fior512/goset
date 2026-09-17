// Unprivileged include/exclude coverage. All cases use -n 1, so
// internal/runner/run.go never creates a cgroup (gated on n>1) and never
// needs root: SelectCPUs/rankCPUs are exercised the same way regardless of
// n. Requires >=4 online cpus; skips with a reason otherwise.
package integration

import (
	"strconv"
	"strings"
	"testing"

	"goset/internal/cpu"
)

// onlineIDs returns the online cpu ids in ascending order.
func onlineIDs(t *testing.T) []int {
	t.Helper()
	topo, err := cpu.GetTopology()
	if err != nil {
		t.Fatalf("cpu.GetTopology: %v", err)
	}
	var ids []int
	for id := range topo.Online.All() {
		ids = append(ids, id)
	}
	if len(ids) < 4 {
		t.Skipf("need >=4 online cpus for the selection matrix, have %d", len(ids))
	}
	return ids
}

func csv(ids ...int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func containsInt(list []int, want int) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func setup(t *testing.T) (gosetBin, probeBin string) {
	t.Helper()
	root := repoRoot(t)
	return buildBin(t, root, "goset", "./cmd/goset"), buildBin(t, root, "threadprobe", "./test/threadprobe")
}

// TestSelectionNoIncludeNoExclude is the baseline: neither flag set.
func TestSelectionNoIncludeNoExclude(t *testing.T) {
	onlineIDs(t)
	gosetBin, probeBin := setup(t)

	res := runGoset(t, gosetBin, "-n", "1", "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	rep := parseProbe(t, res.stdout)
	if got := cpuListCount(rep.Threads[0].Allowed); got != 1 {
		t.Errorf("allowed=%q has %d cpu(s), want 1", rep.Threads[0].Allowed, got)
	}
}

// TestSelectionIncludeExactlyN: include set size == n (1). rankCPUs sorts
// included cpus first, so with size == n the benchmark set must equal include.
func TestSelectionIncludeExactlyN(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	want := ids[0]

	res := runGoset(t, gosetBin, "-n", "1", "-include", itoa(want), "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	rep := parseProbe(t, res.stdout)
	if rep.Threads[0].Allowed != itoa(want) {
		t.Errorf("allowed=%q, want exactly %q (include size == n)", rep.Threads[0].Allowed, itoa(want))
	}
}

// TestSelectionIncludeGreaterThanN: include has more cpus than n (1). goset
// does not error; rankCPUs takes scores[:n], silently keeping only n of the
// included cpus. This test pins that behavior, not an assumption of it.
func TestSelectionIncludeGreaterThanN(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	include := csv(ids[0], ids[1], ids[2])

	res := runGoset(t, gosetBin, "-n", "1", "-include", include, "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	rep := parseProbe(t, res.stdout)
	got := parseCPUList(t, rep.Threads[0].Allowed)
	if len(got) != 1 {
		t.Fatalf("allowed=%q has %d cpu(s), want 1", rep.Threads[0].Allowed, len(got))
	}
	if !containsInt(ids[:3], got[0]) {
		t.Errorf("selected cpu %d is not one of the requested include cpus %v", got[0], ids[:3])
	}
}

// TestSelectionExcludeShrinksPool: exclude removes cpus but the pool stays
// above the n+1 minimum. Benchmark must avoid every excluded cpu.
func TestSelectionExcludeShrinksPool(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	excluded := ids[len(ids)-1]

	res := runGoset(t, gosetBin, "-n", "1", "-exclude", itoa(excluded), "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	rep := parseProbe(t, res.stdout)
	got := parseCPUList(t, rep.Threads[0].Allowed)
	if containsInt(got, excluded) {
		t.Errorf("allowed=%v includes excluded cpu %d", got, excluded)
	}
}

// TestSelectionExcludeToBoundary: exclude removes all but exactly n+1
// candidates (the minimum SelectCPUs allows). Must still succeed.
func TestSelectionExcludeToBoundary(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	// n=1 needs 2 candidates; exclude everything past the first two.
	excluded := csv(ids[2:]...)

	res := runGoset(t, gosetBin, "-n", "1", "-exclude", excluded, "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("boundary exclude (exactly n+1 left) exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	rep := parseProbe(t, res.stdout)
	got := parseCPUList(t, rep.Threads[0].Allowed)
	if len(got) != 1 || !containsInt(ids[:2], got[0]) {
		t.Errorf("allowed=%v, want exactly one of %v", got, ids[:2])
	}
}

// TestSelectionExcludeBelowMinimum: exclude removes so many candidates that
// fewer than n+1 remain. Must error, mentioning the housekeeper requirement.
func TestSelectionExcludeBelowMinimum(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	// Leave only 1 candidate for n=1 (need 2: n + housekeeper).
	excluded := csv(ids[1:]...)

	res := runGoset(t, gosetBin, "-n", "1", "-exclude", excluded, "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("expected error when excluding down to fewer than n+1 candidates, got exit 0")
	}
	if !strings.Contains(res.stderr, "housekeeper") {
		t.Errorf("stderr should explain the housekeeper+n requirement, got: %s", res.stderr)
	}
}

// TestSelectionIncludeOffline: include references a cpu id past the last
// online cpu. Must error, not silently ignore the bad id.
func TestSelectionIncludeOffline(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	bogus := ids[len(ids)-1] + 1000

	res := runGoset(t, gosetBin, "-n", "1", "-include", itoa(bogus), "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("expected error for out-of-range -include, got exit 0")
	}
	if !strings.Contains(res.stderr, "offline or excluded") {
		t.Errorf("stderr should mention 'offline or excluded', got: %s", res.stderr)
	}
}

// TestSelectionIncludeAndExcludeDisjoint: both flags set, disjoint, pool
// still valid. Benchmark must honor include and avoid exclude.
func TestSelectionIncludeAndExcludeDisjoint(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	want := ids[0]
	excluded := ids[1]

	res := runGoset(t, gosetBin, "-n", "1", "-include", itoa(want), "-exclude", itoa(excluded), "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	rep := parseProbe(t, res.stdout)
	if rep.Threads[0].Allowed != itoa(want) {
		t.Errorf("allowed=%q, want exactly %q", rep.Threads[0].Allowed, itoa(want))
	}
}
