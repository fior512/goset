// goset-bench evaluates whether goset's isolation helps a workload:
// it interleaves the benchmark dangling (baseline) against the
// benchmark through goset (-cgroup -steer) and reports the benchmark's
// headline metric for both modes side by side, plus goset's IRQ/ctxsw/
// throttle counters for the isolated mode.
//
// Usage:
//
//	sudo goset-bench [flags] -- ./bench [args...]   your benchmark
//	sudo goset-bench [flags] -- jitter               built-in mock
//
// goset is found via -goset-bin, PATH, or next to goset-bench.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"goset/benchmark"
	"goset/benchmark/plugin"
	"goset/internal/generic"
	rpt "goset/internal/report"
	"goset/internal/telemetry"
)

type sample struct {
	cpu       string // the '*'-marked benchmark cpu from goset's Selection table
	wall      float64
	global    map[string]float64
	telemetry map[string]float64
	metrics   []plugin.Metric
	structure map[string]int // output shape fingerprint, externalCmd only
}

func (s sample) metric(name string) (float64, bool) {
	for _, m := range s.metrics {
		if m.Name == name {
			return m.Value, true
		}
	}
	return 0, false
}

func (s sample) samples(name string) []float64 {
	for _, m := range s.metrics {
		if m.Name == name {
			return m.Samples
		}
	}
	return nil
}

var splitCols = regexp.MustCompile(`\s{2,}`)

func main() {
	fs := flag.NewFlagSet("goset-bench", flag.ExitOnError)
	name := fs.String("bench", "", "alias for a built-in after --: "+benchNames())
	runs := fs.Int("runs", 10, "interleaved runs per mode")
	threads := fs.Int("n", 1, "threads to book for the isolated run (goset -n)")
	settle := fs.Duration("settle", time.Second, "idle delay before each run, both modes start from the same post-idle state")
	cpu := fs.Int("cpu", -1, "pin isolated task to this CPU across runs (default: fresh quietest)")
	gosetBin := fs.String("goset-bin", defaultGosetBin, "path to the goset binary (default: PATH, then next to goset-bench)")
	match := fs.String("match", "", "regex selecting reported metric keys (overrides auto top-K)")
	topk := fs.Int("topk", 8, "max extracted metric keys reported per mode")
	dump := fs.Bool("dump", false, "print extracted keys and per-run sample counts before reports")
	for _, b := range benchmark.Registry {
		b.RegisterFlags(fs)
	}
	fs.Parse(os.Args[1:])

	var bench plugin.Benchmark
	if args := fs.Args(); len(args) > 0 {
		if b, ok := benchmark.Registry[args[0]]; ok && !strings.Contains(args[0], "/") {
			if len(args) > 1 {
				fmt.Fprintf(os.Stderr, "goset-bench: built-in %q takes no extra args\n", args[0])
				os.Exit(1)
			}
			bench = b
		} else {
			bench = &externalCmd{argv: args}
			if _, err := exec.LookPath(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "goset-bench: %q: %v\n", args[0], err)
				os.Exit(1)
			}
		}
	} else {
		var ok bool
		bench, ok = benchmark.Registry[*name]
		if !ok {
			fmt.Fprintf(os.Stderr, "goset-bench: unknown -bench %q, choices: %s (or pass a command after --)\n", *name, benchNames())
			os.Exit(1)
		}
	}

	gosetPath, err := resolveGoset(*gosetBin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "goset-bench:", err)
		os.Exit(1)
	}
	bin, err := bench.Build()
	if err != nil {
		fmt.Fprintln(os.Stderr, "goset-bench:", err)
		os.Exit(1)
	}
	argv := bench.Argv(bin)
	isoArgs := []string{"-n", strconv.Itoa(*threads), "-cgroup", "-steer"}
	if *cpu >= 0 {
		isoArgs = append(isoArgs, "-include", strconv.Itoa(*cpu))
	}
	isoArgs = append(isoArgs, "--")
	isoArgs = append(isoArgs, argv...)

	var baseline, isolated []sample
	for i := 0; i < *runs; i++ {
		time.Sleep(*settle)
		if s, ok := runBaseline(bench, argv); ok {
			baseline = append(baseline, s)
		}
		time.Sleep(*settle)
		if s, ok := run(gosetPath, bench, "isolated", i, isoArgs); ok {
			isolated = append(isolated, s)
		}
	}

	if *dump {
		dumpKeys("baseline", baseline)
		dumpKeys("isolated", isolated)
	}
	keys, err := selectMetrics(baseline, isolated, *match, *topk)
	if err != nil {
		fmt.Fprintln(os.Stderr, "goset-bench:", err)
		os.Exit(1)
	}
	driftWarn("baseline", baseline)
	driftWarn("isolated", isolated)
	structureWarn(baseline, isolated)
	report("baseline (dangling)", baseline, false, keys)
	report("isolated (-cgroup -steer)", isolated, true, keys)
}

// collectMetrics extracts metrics plus the output-structure fingerprint
// when the benchmark provides one (externalCmd only).
func collectMetrics(bench plugin.Benchmark, out []byte) ([]plugin.Metric, map[string]int) {
	metrics := bench.Metrics(out)
	if ext, ok := bench.(*externalCmd); ok {
		return metrics, ext.fingerprint()
	}
	return metrics, nil
}

// selectMetrics picks the reported metric keys, joint over both modes so
// the side-by-side tables stay aligned. -match overrides everything. Auto
// mode drops config values (single sample per run, identical across every
// run of both modes) and ranks the rest deterministically: varying keys
// first (stability signal lives there), then present-in-all-runs,
// per-run distribution, sample count, name. cv is a result of this tool,
// never a selector.
func selectMetrics(baseline, isolated []sample, match string, topk int) ([]string, error) {
	all := append(append([]sample{}, baseline...), isolated...)
	names := unionNames(all)
	// Build display map for -match (match against display label when available)
	displays := map[string]string{}
	for _, name := range names {
		for _, s := range all {
			for _, m := range s.metrics {
				if m.Name == name {
					if m.Display != "" {
						displays[name] = m.Display
					} else {
						displays[name] = m.Name
					}
					break
				}
			}
			if _, ok := displays[name]; ok {
				break
			}
		}
	}
	if match != "" {
		re, err := regexp.Compile(match)
		if err != nil {
			return nil, fmt.Errorf("-match: %w", err)
		}
		var matched []string
		for _, name := range names {
			d := displays[name]
			if re.MatchString(d) || re.MatchString(name) {
				matched = append(matched, name)
			}
		}
		return matched, nil
	}
	type rank struct {
		name     string
		constant bool
		varies   bool
		runs     int
		hasDist  bool
		nSamples int
	}
	ranks := make([]rank, 0, len(names))
	for _, name := range names {
		r := rank{name: name, constant: true}
		var heads []float64
		for _, s := range all {
			if _, ok := s.metric(name); !ok {
				continue
			}
			r.runs++
			samples := s.samples(name)
			r.nSamples += len(samples)
			if len(samples) > 1 {
				r.hasDist = true
				r.constant = false
			}
			v, _ := s.metric(name)
			heads = append(heads, v)
		}
		if len(heads) > 0 && minOf(heads) != maxOf(heads) {
			r.varies = true
			r.constant = false
		}
		ranks = append(ranks, r)
	}
	sort.SliceStable(ranks, func(i, j int) bool {
		a, b := ranks[i], ranks[j]
		if a.varies != b.varies {
			return a.varies
		}
		if a.runs != b.runs {
			return a.runs > b.runs
		}
		if a.hasDist != b.hasDist {
			return a.hasDist
		}
		if a.constant != b.constant {
			return !a.constant // non-constants first
		}
		if a.nSamples != b.nSamples {
			return a.nSamples > b.nSamples
		}
		return a.name < b.name
	})
	keys := make([]string, 0, min(topk, len(ranks)))
	for i, r := range ranks {
		if i >= topk {
			break
		}
		keys = append(keys, r.name)
	}
	return keys, nil
}

// unionNames lists every metric name present in any run, first-seen order.
func unionNames(samples []sample) []string {
	var names []string
	for _, s := range samples {
		for _, m := range s.metrics {
			if indexOf(names, m.Name) < 0 {
				names = append(names, m.Name)
			}
		}
	}
	return names
}

// dumpKeys prints extracted keys with their per-run sample counts, before
// any filtering: the debugging view for "why is my metric missing".
func dumpKeys(label string, samples []sample) {
	fmt.Fprintf(os.Stderr, "%s: extracted keys (samples per run)\n", label)
	for _, name := range unionNames(samples) {
		var counts []string
		for _, s := range samples {
			if _, ok := s.metric(name); ok {
				counts = append(counts, strconv.Itoa(len(s.samples(name))))
			} else {
				counts = append(counts, "-")
			}
		}
		fmt.Fprintf(os.Stderr, "  %-40s %s\n", name, strings.Join(counts, " "))
	}
}

// driftWarn compares output-shape fingerprints across runs of one mode.
// A diverging shape means key alignment may miscompare across runs.
func driftWarn(label string, samples []sample) {
	if len(samples) < 2 || samples[0].structure == nil {
		return
	}
	base := samples[0].structure
	warned := map[string]bool{}
	for i, s := range samples[1:] {
		if s.structure == nil {
			return
		}
		for shape, count := range base {
			if c := s.structure[shape]; c != count && !warned[shape] {
				fmt.Fprintf(os.Stderr, "%s: output drift: %q x%d in run 0, x%d in run %d; cross-run stats may miscompare\n",
					label, shape, count, c, i+1)
				warned[shape] = true
			}
		}
		for shape := range s.structure {
			if _, ok := base[shape]; !ok && !warned[shape] {
				fmt.Fprintf(os.Stderr, "%s: output drift: %q absent in run 0, present in run %d; cross-run stats may miscompare\n",
					label, shape, i+1)
				warned[shape] = true
			}
		}
	}
}

// structureWarn compares the first baseline run with the first isolated
// run. Divergent shapes mean the two modes did not print the same
// structure, so the side-by-side stats may pair different quantities.
func structureWarn(baseline, isolated []sample) {
	if len(baseline) == 0 || len(isolated) == 0 {
		return
	}
	base, other := baseline[0].structure, isolated[0].structure
	if base == nil || other == nil {
		return
	}
	var shapes []string
	for shape := range base {
		if _, ok := other[shape]; !ok {
			shapes = append(shapes, shape)
		}
	}
	for shape := range other {
		if _, ok := base[shape]; !ok {
			shapes = append(shapes, shape)
		}
	}
	sort.Strings(shapes)
	for _, shape := range shapes {
		fmt.Fprintf(os.Stderr, "baseline vs isolated: output structure differs: %q x%d baseline, x%d isolated; side-by-side stats may miscompare\n",
			shape, base[shape], other[shape])
	}
}

// defaultGosetBin is the -goset-bin default: resolved, not hardcoded.
const defaultGosetBin = "goset"

// resolveGoset: explicit flag, PATH, sibling of goset-bench, repo bin/.
// Sibling avoids exec.ErrDot for bare names under sudo from repo root.
func resolveGoset(flagVal string) (string, error) {
	if flagVal != defaultGosetBin {
		return flagVal, nil
	}
	if p, err := exec.LookPath(defaultGosetBin); err == nil {
		return p, nil
	}
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), defaultGosetBin))
	}
	candidates = append(candidates, "bin/goset", "./goset")
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("goset binary not found; build it: go build -o bin/goset ./cmd/goset")
}

func benchNames() string {
	names := make([]string, 0, len(benchmark.Registry))
	for name := range benchmark.Registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func run(gosetBin string, bench plugin.Benchmark, label string, i int, gosetArgs []string) (sample, bool) {
	cmd := exec.Command(gosetBin, gosetArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s run %d failed: %v\n%s%s", label, i, err, stdout.Bytes(), stderr.Bytes())
		return sample{}, false
	}
	s := parseReport(stderr.Bytes())
	if s.global[generic.GlobalExit] != 0 {
		fmt.Fprintf(os.Stderr, "%s run %d: task exited %v, run discarded\n%s%s", label, i, s.global[generic.GlobalExit], stdout.Bytes(), stderr.Bytes())
		return sample{}, false
	}
	s.metrics, s.structure = collectMetrics(bench, taskOutput(stdout.Bytes(), stderr.Bytes()))
	return s, true
}

// gosetReportMarker starts goset's own report on stderr (printed by
// internal/runner/telemetry.go): everything from it on is goset's output,
// not the task's.
const gosetReportMarker = "----------------- GOSET -----------------"

// taskOutput rebuilds the task's output: its stdout plus its stderr up to
// goset's report marker, minus goset's own log lines. Metric extraction
// must never see goset's tables: they carry numbers that vary per run and
// would be mistaken for benchmark metrics.
func taskOutput(stdout, stderr []byte) []byte {
	cut := stderr
	if i := bytes.Index(stderr, []byte(gosetReportMarker)); i >= 0 {
		cut = stderr[:i]
	}
	var lines []string
	for _, line := range strings.Split(string(cut), "\n") {
		if strings.HasPrefix(line, generic.LogPrefix) {
			continue
		}
		lines = append(lines, line)
	}
	out := append(stdout, '\n')
	return append(out, strings.Join(lines, "\n")...)
}

// runBaseline runs the benchmark binary directly, dangling on whatever
// core the linux load balancer picks: no goset, no pin, no cgroup, no
// telemetry polling that could collide with the task. Only wall, the
// task's own output, and post-exit wait4 rusage are kept.
func runBaseline(bench plugin.Benchmark, argv []string) (sample, bool) {
	cmd := exec.Command(argv[0], argv[1:]...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	start := time.Now()
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "baseline run failed: %v\n", err)
		return sample{}, false
	}
	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "baseline run failed: %v\n%s\n", err, buf.Bytes())
		return sample{}, false
	}
	s := sample{global: map[string]float64{}, telemetry: map[string]float64{}}
	s.wall = time.Since(start).Seconds()
	s.metrics, s.structure = collectMetrics(bench, buf.Bytes())
	if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
		s.global[generic.GlobalCtxswVoluntary] = float64(ru.Nvcsw)
		s.global[generic.GlobalCtxswInvoluntary] = float64(ru.Nivcsw)
	}
	return s, true
}

// parseReport scrapes goset's Global and Telemetry tables (see
// internal/report/render.go) since goset has no --export json yet.
func parseReport(out []byte) sample {
	s := sample{global: map[string]float64{}, telemetry: map[string]float64{}}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	var section string
	var header []string
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			section, header = "", nil
			continue
		}
		if trimmed == "Global" || trimmed == "Telemetry" || trimmed == "Selection" {
			section, header = trimmed, nil
			continue
		}
		if section == "" {
			continue
		}
		fields := splitCols.Split(trimmed, -1)
		if header == nil {
			header = fields
			continue
		}
		switch section {
		case "Global":
			if len(fields) == 2 {
				if fields[0] == generic.GlobalWall {
					s.wall = parseDuration(fields[1])
				} else if fields[0] == generic.GlobalRunDelay {
					s.global[fields[0]] = parseDuration(fields[1])
				} else {
					s.global[fields[0]] = parseCount(fields[1])
				}
			}
		case "Telemetry":
			if idx := indexOf(header, "sum"); idx >= 0 && idx < len(fields) {
				s.telemetry[fields[0]] = parseCount(fields[idx])
			}
		case "Selection":
			if idx := indexOf(header, "sel"); idx >= 0 && idx < len(fields) && strings.TrimSpace(fields[idx]) == "*" {
				s.cpu = fields[0]
			}
		}
	}
	return s
}

func indexOf(fields []string, want string) int {
	for i, f := range fields {
		if f == want {
			return i
		}
	}
	return -1
}

// parseDuration inverts report.FormatTime.
func parseDuration(s string) float64 {
	units := []struct {
		suffix string
		mult   float64
	}{{"ns", 1e-9}, {"us", 1e-6}, {"ms", 1e-3}, {"s", 1}}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			v, _ := strconv.ParseFloat(strings.TrimSuffix(s, u.suffix), 64)
			return v * u.mult
		}
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseCount inverts report.FormatValue (k/M/B/T SI-style suffixes).
func parseCount(s string) float64 {
	units := map[byte]float64{'k': 1e3, 'M': 1e6, 'B': 1e9, 'T': 1e12}
	if len(s) > 0 {
		if mult, ok := units[s[len(s)-1]]; ok {
			v, _ := strconv.ParseFloat(s[:len(s)-1], 64)
			return v * mult
		}
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// counterKeys are the goset counters shown per run and summarized. Kept
// short and per-run visible: an HPC engineer needs to see which run was
// the outlier, not just a folded average.
var counterKeys = []string{
	telemetry.Counter{Source: telemetry.SourceIRQ, Name: telemetry.IRQSoft}.Label(),
	telemetry.Counter{Source: telemetry.SourceIRQ, Name: telemetry.IRQHard}.Label(),
	telemetry.Counter{Source: telemetry.SourceThrottle, Name: telemetry.ThrottleCount}.Label(),
}
var globalKeys = []string{
	generic.GlobalCtxswVoluntary, generic.GlobalCtxswInvoluntary, generic.GlobalMigrations, generic.GlobalRunDelay,
	generic.GlobalIRQSteerApplied, generic.GlobalIRQSteerRejected, generic.GlobalIRQSteerRemaining,
}

func report(label string, samples []sample, hasGoset bool, metricNames []string) {
	fmt.Printf("\n%s (n=%d)\n", label, len(samples))
	if len(samples) == 0 {
		fmt.Println("  no successful runs")
		return
	}

	// Build display labels and column ordinals from the first sample that has them
	displays := map[string]string{}
	cols := map[string]int{}
	for _, name := range metricNames {
		for _, s := range samples {
			for _, m := range s.metrics {
				if m.Name == name {
					if m.Display != "" {
						displays[name] = m.Display
					} else {
						displays[name] = m.Name
					}
					cols[name] = m.Col
					break
				}
			}
			if _, ok := displays[name]; ok {
				break
			}
		}
	}

	// Restore source-column order: group by row prefix (name before " #"),
	// then sort by Col within each group
	ordered := restoreColumnOrder(metricNames, cols)

	// Dedupe safety net: if two distinct keys share the same display label,
	// append " #<col>" to the later one so headers/stats stay unique.
	seen := map[string]bool{}
	for _, name := range ordered {
		d := displays[name]
		if seen[d] {
			displays[name] = fmt.Sprintf("%s #%d", d, cols[name])
		} else {
			seen[d] = true
		}
	}

	active := activeGlobalKeys(samples)

	header := []string{"run", "cpu"}
	for _, name := range ordered {
		header = append(header, displays[name])
	}
	header = append(header, "wall")
	if hasGoset {
		header = append(header, counterKeys...)
	}
	header = append(header, active...)

	rows := make([][]string, len(samples))
	for i, s := range samples {
		row := []string{fmt.Sprintf("%d", i), s.cpu}
		for _, name := range ordered {
			if v, ok := s.metric(name); ok {
				row = append(row, cellFormatter(displays[name])(v))
			} else {
				row = append(row, "-")
			}
		}
		row = append(row, fmt.Sprintf("%.3fs", s.wall))
		if hasGoset {
			for _, key := range counterKeys {
				row = append(row, fmt.Sprintf("%.0f", s.telemetry[key]))
			}
		}
		for _, key := range active {
			if v, ok := s.global[key]; ok {
				row = append(row, cellFormatter(key)(v))
			} else {
				row = append(row, "-")
			}
		}
		rows[i] = row
	}
	rpt.Render(os.Stdout, rpt.Table{Header: header, Rows: rows})

	labels := []string{"wall(s)"}
	for _, name := range ordered {
		labels = append(labels, displays[name])
	}
	labels = append(labels, counterKeys...)
	labels = append(labels, active...)
	width := statWidth(labels)

	fmt.Println()
	for _, name := range ordered {
		name := name
		values := make([]float64, 0, len(samples))
		for _, s := range samples {
			if v, ok := s.metric(name); ok {
				values = append(values, v)
			}
		}
		printStat(truncLabel(displays[name], width), width, values)
	}
	printStat(truncLabel("wall(s)", width), width, getKey(samples, func(s sample) float64 { return s.wall }))
	if hasGoset {
		for _, key := range counterKeys {
			key := key
			printStat(truncLabel(key, width), width, getKey(samples, func(s sample) float64 { return s.telemetry[key] }))
		}
	}
	for _, key := range active {
		key := key
		values := make([]float64, 0, len(samples))
		for _, s := range samples {
			if v, ok := s.global[key]; ok {
				values = append(values, v)
			}
		}
		printStat(truncLabel(key, width), width, values)
	}

	var tails []sampleTail
	for _, name := range ordered {
		pooled := pool(samples, func(s sample) []float64 { return s.samples(name) })
		if len(pooled) > 0 {
			tails = append(tails, sampleTail{label: truncLabel(displays[name], width), values: pooled})
		}
	}
	if len(tails) > 0 {
		fmt.Println()
		fmt.Println("  tails (pooled per-iteration samples)")
		for _, tail := range tails {
			printTail(tail.label, width, tail.values)
		}
	}
}

// sampleTail pairs a display label with its pooled per-iteration samples.
type sampleTail struct {
	label  string
	values []float64
}

// statWidth is the label column width shared by both stats sections: the
// longest label in play, clamped so a verbose benchmark cannot push the
// numbers off the right edge.
func statWidth(labels []string) int {
	widest := 0
	for _, label := range labels {
		if len(label) > widest {
			widest = len(label)
		}
	}
	if widest < 18 {
		widest = 18
	}
	if widest > 32 {
		widest = 32
	}
	return widest
}

// truncLabel returns label truncated to width with ellipsis if it exceeds width.
// width must be >= 3 for ellipsis to apply; otherwise returns label as-is.
func truncLabel(label string, width int) string {
	if len(label) <= width {
		return label
	}
	if width < 3 {
		return label[:width]
	}
	return label[:width-3] + "..."
}

// restoreColumnOrder groups metric names by their row prefix (text before
// " #") and sorts by Col within each group to restore source-column order.
func restoreColumnOrder(names []string, cols map[string]int) []string {
	groups := map[string][]string{}
	var groupOrder []string
	for _, name := range names {
		prefix := name
		if idx := strings.LastIndex(name, " #"); idx >= 0 {
			prefix = name[:idx]
		}
		if _, ok := groups[prefix]; !ok {
			groups[prefix] = nil
			groupOrder = append(groupOrder, prefix)
		}
		groups[prefix] = append(groups[prefix], name)
	}
	var ordered []string
	for _, prefix := range groupOrder {
		group := groups[prefix]
		sort.SliceStable(group, func(i, j int) bool {
			return cols[group[i]] < cols[group[j]]
		})
		ordered = append(ordered, group...)
	}
	return ordered
}

// activeGlobalKeys keeps globalKeys order but drops keys no sample has:
// baseline has no irq steer rows, isolated has all goset rows.
func activeGlobalKeys(samples []sample) []string {
	var active []string
	for _, key := range globalKeys {
		for _, s := range samples {
			if _, ok := s.global[key]; ok {
				active = append(active, key)
				break
			}
		}
	}
	return active
}

// printStat prints avg/median/sd/cv%/min/max for one series. cv (the
// coefficient of variation, sd as a percentage of the mean) is the number
// that actually says whether a spread is large or small; raw sd alone
// isn't comparable across metrics of different scale.
// width is the column width for the label (from statWidth in report()).
func printStat(label string, width int, values []float64) {
	if len(values) == 0 {
		fmt.Printf("  %-*s no data\n", width, label)
		return
	}
	mean := avg(values)
	cv := 0.0
	if mean != 0 {
		cv = stddev(values) / mean * 100
	}
	f := cellFormatter(label)
	fmt.Printf("  %-*s n=%-4d avg=%-12s med=%-12s sd=%-12s cv=%5.1f%%  min=%-12s max=%s\n",
		width, label, len(values), f(mean), f(median(values)), f(stddev(values)), cv, f(minOf(values)), f(maxOf(values)))
}

// isTimeLabel marks the two series carried as raw time: the jitter
// built-in's "jitter iter" holds nanoseconds, goset's "run_delay" holds
// seconds (parseDuration). Exact match only, both names owned by goset:
// extracted benchmarks can print any row name, so only a namespaced
// built-in label and goset's own key may trigger time formatting.
func isTimeLabel(label string) bool {
	return label == generic.GlobalRunDelay || label == "jitter iter"
}

func fmtVal(label string, v float64) string {
	if label == generic.GlobalRunDelay {
		return rpt.FormatTime(time.Duration(v * float64(time.Second)))
	}
	return rpt.FormatTime(time.Duration(v))
}

// cellFormatter picks the value formatter. Extracted benchmark values are
// printed raw: their unit already sits in the header next to the label.
func cellFormatter(label string) func(float64) string {
	if !isTimeLabel(label) {
		return func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
	}
	return func(v float64) string { return fmtVal(label, v) }
}

// pool concatenates every run's per-iteration samples for one metric,
// so tail stats reflect all iterations of a condition, not one run.
func pool(samples []sample, get func(sample) []float64) []float64 {
	var out []float64
	for _, s := range samples {
		out = append(out, get(s)...)
	}
	return out
}

// printTail reports the tail of a pooled per-iteration distribution.
// A quiet pinning mechanism should not move p50, it should shrink
// p99.9/p50 (tail inflation) relative to the unpinned baseline.
// width is the column width for the label (from statWidth in report()).
func printTail(label string, width int, values []float64) {
	if len(values) == 0 {
		return
	}
	p50 := percentile(values, 0.50)
	p99 := percentile(values, 0.99)
	p999 := percentile(values, 0.999)
	ratio := 0.0
	if p50 != 0 {
		ratio = p999 / p50
	}
	f := cellFormatter(label)
	fmt.Printf("  %-*s n=%-6d p50=%-12s p99=%-12s p99.9=%-12s tail=%.2fx\n",
		width, label, len(values), f(p50), f(p99), f(p999), ratio)
}

func percentile(values []float64, p float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	idx := int(p * float64(len(sorted)-1))
	return sorted[idx]
}

func getKey(samples []sample, get func(sample) float64) []float64 {
	values := make([]float64, len(samples))
	for i, s := range samples {
		values[i] = get(s)
	}
	return values
}

func minOf(values []float64) float64 {
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxOf(values []float64) float64 {
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func avg(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func stddev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := avg(values)
	var acc float64
	for _, v := range values {
		diff := v - mean
		acc += diff * diff
	}
	return math.Sqrt(acc / float64(len(values)))
}
