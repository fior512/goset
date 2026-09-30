package integration

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
)

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

func containsInt(list []int, want int) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func assertAllowed(t *testing.T, rep probeReport, want int, note string) {
	t.Helper()
	if got := rep.Threads[0].Allowed; got != strconv.Itoa(want) {
		t.Errorf("allowed=%q, want exactly %q (%s)", got, strconv.Itoa(want), note)
	}
}

func setup(t *testing.T) (gosetBin, probeBin string) {
	t.Helper()
	root := repoRoot(t)
	return buildBin(t, root, "goset", "./cmd/goset"), buildBin(t, root, "threadprobe", "./test/threadprobe")
}

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

func TestSelectionIncludeExactlyN(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	want := ids[0]

	res := runGoset(t, gosetBin, "-n", "1", "-include", strconv.Itoa(want), "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	rep := parseProbe(t, res.stdout)
	assertAllowed(t, rep, want, "include size == n")
}

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

func TestSelectionExcludeShrinksPool(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	excluded := ids[len(ids)-1]

	res := runGoset(t, gosetBin, "-n", "1", "-exclude", strconv.Itoa(excluded), "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	rep := parseProbe(t, res.stdout)
	got := parseCPUList(t, rep.Threads[0].Allowed)
	if containsInt(got, excluded) {
		t.Errorf("allowed=%v includes excluded cpu %d", got, excluded)
	}
}

func TestSelectionExcludeToBoundary(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	// n=1 needs 2 candidates; exclude everything past the first two
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

func TestSelectionExcludeBelowMinimum(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	// Leave only 1 candidate for n=1 (need 2: n + housekeeper)
	excluded := csv(ids[1:]...)

	res := runGoset(t, gosetBin, "-n", "1", "-exclude", excluded, "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("expected error when excluding down to fewer than n+1 candidates, got exit 0")
	}
	if !strings.Contains(res.stderr, "housekeeper") {
		t.Errorf("stderr should explain the housekeeper+n requirement, got: %s", res.stderr)
	}
}

func TestSelectionIncludeOffline(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	bogus := ids[len(ids)-1] + 1000

	res := runGoset(t, gosetBin, "-n", "1", "-include", strconv.Itoa(bogus), "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("expected error for out-of-range -include, got exit 0")
	}
	if !strings.Contains(res.stderr, "offline or excluded") {
		t.Errorf("stderr should mention 'offline or excluded', got: %s", res.stderr)
	}
}

func TestSelectionIncludeAndExcludeDisjoint(t *testing.T) {
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	want := ids[0]
	excluded := ids[1]

	res := runGoset(t, gosetBin, "-n", "1", "-include", strconv.Itoa(want), "-exclude", strconv.Itoa(excluded), "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	rep := parseProbe(t, res.stdout)
	assertAllowed(t, rep, want, "include minus exclude")
}

func hostNuma(t *testing.T) (topo *cpu.Topology, nodes []int) {
	t.Helper()
	topo, err := cpu.GetTopology()
	if err != nil {
		t.Fatalf("cpu.GetTopology: %v", err)
	}
	seen := map[int]bool{}
	for id := range topo.Online.All() {
		if numa := topo.Numa[id]; numa >= 0 {
			seen[numa] = true
		}
	}
	for numa := range seen {
		nodes = append(nodes, numa)
	}
	slices.Sort(nodes)
	return topo, nodes
}

func TestNumaNotOnHost(t *testing.T) {
	preflight(t, 2)
	_, nodes := hostNuma(t)
	gosetBin, probeBin := setup(t)
	missing := nodes[len(nodes)-1] + 1
	if containsInt(nodes, missing) {
		t.Fatalf("test assumes %d is not a host numa node", missing)
	}

	res := runGoset(t, gosetBin, "-n", "1", "-numa", strconv.Itoa(missing), "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("-numa %d: expected error, got exit 0", missing)
	}
	if !strings.Contains(res.stderr, "host numa nodes") {
		t.Errorf("stderr should list the host numa nodes, got: %s", res.stderr)
	}
}

func TestNumaAutoStaysOnOneNode(t *testing.T) {
	preflight(t, 2)
	topo, _ := hostNuma(t)
	gosetBin, probeBin := setup(t)

	res := runGoset(t, gosetBin, "-n", "1", "-numa", "-1", "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	held := map[int]bool{}
	for _, id := range parseCPUList(t, parseProbe(t, res.stdout).Threads[0].Allowed) {
		held[topo.Numa[id]] = true
	}
	if len(held) != 1 {
		t.Errorf("-numa -1 selected cpus spread over nodes %v, want one", held)
	}
}

func TestNumaExplicitHoldsPool(t *testing.T) {
	preflight(t, 2)
	topo, nodes := hostNuma(t)
	gosetBin, probeBin := setup(t)
	want := nodes[0]

	res := runGoset(t, gosetBin, "-n", "1", "-numa", strconv.Itoa(want), "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	for _, id := range parseCPUList(t, parseProbe(t, res.stdout).Threads[0].Allowed) {
		if got := topo.Numa[id]; got != want {
			t.Errorf("cpu %d sits on node %d, want %d", id, got, want)
		}
	}
}

func TestNumaExplicitIncludeOnOtherNode(t *testing.T) {
	preflight(t, 4)
	topo, nodes := hostNuma(t)
	if len(nodes) < 2 {
		t.Skipf("host has %d numa node(s), need 2", len(nodes))
	}
	gosetBin, probeBin := setup(t)
	var foreign []int
	for id := range topo.Online.All() {
		if topo.Numa[id] != nodes[0] {
			foreign = append(foreign, id)
		}
	}

	res := runGoset(t, gosetBin, "-n", "1",
		"-numa", strconv.Itoa(nodes[0]),
		"-include", strconv.Itoa(foreign[0]), "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("-include %d with -numa %d: expected error, got exit 0",
			foreign[0], nodes[0])
	}
	if !strings.Contains(res.stderr, "another numa node") {
		t.Errorf("stderr should name the conflict, got: %s", res.stderr)
	}
}

func TestNumaExplicitBelowMinimum(t *testing.T) {
	preflight(t, 2)
	topo, nodes := hostNuma(t)
	gosetBin, probeBin := setup(t)
	keep := -1
	var drop []int
	for id := range topo.Online.All() {
		switch {
		case topo.Numa[id] != nodes[0]:
			drop = append(drop, id)
		case keep < 0:
			keep = id
		default:
			drop = append(drop, id)
		}
	}

	res := runGoset(t, gosetBin, "-n", "1",
		"-numa", strconv.Itoa(nodes[0]),
		"-exclude", csv(drop...), "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("expected error below the n+1 minimum, got exit 0")
	}
	if !strings.Contains(res.stderr, "remain on numa "+strconv.Itoa(nodes[0])) {
		t.Errorf("stderr should scope the shortfall to the node, got: %s", res.stderr)
	}
}

func TestNumaOffReportsAnyNode(t *testing.T) {
	preflight(t, 2)
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)

	res := runGoset(t, gosetBin, "-n", "1", "-numa", "-2",
		"-exclude", csv(ids[1:]...), "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("expected error below the n+1 minimum, got exit 0")
	}
	if !strings.Contains(res.stderr, "remain on any numa node") {
		t.Errorf("stderr should report the unconstrained pool, got: %s", res.stderr)
	}
}

func assertFenceMatchesTaskCore(t *testing.T, topo *cpu.Topology, selected *generic.Selection) {
	t.Helper()
	task := selected.Task.NextSet(0)
	for sibling := range selected.Fence.All() {
		if topo.Core[sibling] != topo.Core[task] {
			t.Errorf("fenced cpu %d is on core %d, task cpu %d is on core %d", sibling, topo.Core[sibling], task, topo.Core[task])
		}
		if sibling == task || sibling == selected.HouseKeeper {
			t.Errorf("fence holds cpu %d, the task or housekeeper cpu", sibling)
		}
	}
	for cpuID := range topo.Online.All() {
		onCore := topo.Core[cpuID] == topo.Core[task] && cpuID != task && cpuID != selected.HouseKeeper
		if onCore != selected.Fence.GetBit(cpuID) {
			t.Errorf("cpu %d: on task core (not housekeeper) = %v, in fence = %v", cpuID, onCore, selected.Fence.GetBit(cpuID))
		}
	}
}

func assertFenceOff(t *testing.T, topo *cpu.Topology) {
	t.Helper()
	unfenced, err := cpu.Selection(topo, &cli.Config{NThreads: 1, Numa: -2})
	if err != nil {
		t.Fatalf("Selection: %v", err)
	}
	if unfenced.Fence.Any() {
		t.Errorf("fence disabled, got fence %s", unfenced.Fence.String())
	}
}

func TestSelectionFenceBooksTaskSiblings(t *testing.T) {
	topo, err := cpu.GetTopology()
	if err != nil {
		t.Fatalf("cpu.GetTopology: %v", err)
	}
	selected, err := cpu.Selection(topo, &cli.Config{NThreads: 1, Numa: -2, Fence: true})
	if err != nil {
		t.Fatalf("Selection: %v", err)
	}
	assertFenceMatchesTaskCore(t, topo, selected)
	assertFenceOff(t, topo)
}

/*
a -fence run books whole cores: 2 task threads hold ceil(2/coreSize) cores,
whatever the noise says
*/
func TestSelectionFenceFillsCores(t *testing.T) {
	topo, err := cpu.GetTopology()
	if err != nil {
		t.Fatalf("cpu.GetTopology: %v", err)
	}
	if topo.Online.Count() < 3 {
		t.Skipf("need 2 task cpus and a housekeeper, have %d online", topo.Online.Count())
	}
	// a core that cannot fill leaves no whole core to book
	filled := map[int]int{}
	for cpuID := range topo.Online.All() {
		filled[topo.Core[cpuID]]++
	}
	coreSize := 0
	for _, size := range filled {
		if coreSize == 0 {
			coreSize = size
		} else if size != coreSize {
			t.Skipf("cores hold %v online thread(s), need a uniform core size", filled)
		}
	}

	selected, err := cpu.Selection(topo, &cli.Config{NThreads: 2, Numa: -2, Fence: true})
	if err != nil {
		t.Fatalf("Selection: %v", err)
	}
	cores := map[int]bool{}
	for cpuID := range selected.Task.All() {
		cores[topo.Core[cpuID]] = true
	}
	want := (2 + coreSize - 1) / coreSize
	if len(cores) != want {
		t.Errorf("task holds %d core(s) for 2 threads of %d, want %d", len(cores), coreSize, want)
	}
}
