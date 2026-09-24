package jitter

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"goset/benchmark/plugin"
)

//go:embed jitter.c.tmpl
var source string

type Benchmark struct {
	ops    int
	ntimes int
}

func (b *Benchmark) Name() string { return "jitter" }

func (b *Benchmark) RegisterFlags(fs *flag.FlagSet) {
	fs.IntVar(&b.ops, "jitter-ops", 3_000_000, "jitter: chain steps per timed iteration")
	fs.IntVar(&b.ntimes, "jitter-ntimes", 300, "jitter: timed iterations, per-iteration samples kept for tail analysis")
}

func (b *Benchmark) Build() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	hash := sha256.Sum256([]byte(source))
	dir := filepath.Join(cacheDir, "goset-bench", "jitter-"+hex.EncodeToString(hash[:8]))
	bin := filepath.Join(dir, "jitter")
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("jitter: cache dir: %w", err)
	}
	src := filepath.Join(dir, "jitter.c")
	if err := os.WriteFile(src, []byte(source), 0o644); err != nil {
		return "", fmt.Errorf("jitter: write source: %w", err)
	}
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	cmd := exec.Command(cc, "-O2", "-o", bin, src)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("jitter: build: %w", err)
	}
	return bin, nil
}

func (b *Benchmark) Argv(bin string) []string {
	return []string{bin, strconv.Itoa(b.ops), strconv.Itoa(b.ntimes)}
}

var sampleRe = regexp.MustCompile(`(?m)^SAMPLE jitter (\d+) ([0-9.]+)`)

func (b *Benchmark) Metrics(output []byte) []plugin.Metric {
	var samples []float64
	for _, m := range sampleRe.FindAllSubmatch(output, -1) {
		if v, err := strconv.ParseFloat(string(m[2]), 64); err == nil {
			samples = append(samples, v)
		}
	}
	if len(samples) == 0 {
		return nil
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	med := sorted[len(sorted)/2]
	return []plugin.Metric{{Name: "iter", Display: "jitter iter", Value: med, Samples: samples}}
}
