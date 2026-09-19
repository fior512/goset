package stream

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
	"strconv"
	"strings"

	"goset/benchmark/plugin"
)

//go:embed stream.c.tmpl
var source string

type Benchmark struct {
	size    int
	ntimes  int
	metrics string
}


func (b *Benchmark) Name() string { return "stream" }

func (b *Benchmark) RegisterFlags(fs *flag.FlagSet) {
	fs.IntVar(&b.size, "stream-size", 20_000_000, "STREAM: array elements per array")
	fs.IntVar(&b.ntimes, "stream-ntimes", 300, "STREAM: timed iterations, per-iteration samples kept for tail analysis")
	fs.StringVar(&b.metrics, "stream-metrics", "Triad",
		"STREAM: comma-separated kernels to report, in order (Copy,Scale,Add,Triad)")
}


func (b *Benchmark) Build() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	hash := sha256.Sum256([]byte(source))
	dir := filepath.Join(cacheDir, "goset-bench", "stream-"+hex.EncodeToString(hash[:8]))
	bin := filepath.Join(dir, "stream")
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("stream: cache dir: %w", err)
	}
	src := filepath.Join(dir, "stream.c")
	if err := os.WriteFile(src, []byte(source), 0o644); err != nil {
		return "", fmt.Errorf("stream: write source: %w", err)
	}
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	cmd := exec.Command(cc, "-O2", "-o", bin, src)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("stream: build: %w", err)
	}
	return bin, nil
}


func (b *Benchmark) Argv(bin string) []string {
	return []string{bin, strconv.Itoa(b.size), strconv.Itoa(b.ntimes)}
}


var kernelRe = regexp.MustCompile(`(?m)^(Copy|Scale|Add|Triad)\s+([0-9.]+)`)
var sampleRe = regexp.MustCompile(`(?m)^SAMPLE (Copy|Scale|Add|Triad) (\d+) ([0-9.]+)`)

func (b *Benchmark) Metrics(output []byte) []plugin.Metric {
	best := map[string]float64{}
	for _, m := range kernelRe.FindAllSubmatch(output, -1) {
		if v, err := strconv.ParseFloat(string(m[2]), 64); err == nil {
			best[string(m[1])] = v
		}
	}

	samples := map[string][]float64{}
	for _, m := range sampleRe.FindAllSubmatch(output, -1) {
		kernel := string(m[1])
		if v, err := strconv.ParseFloat(string(m[3]), 64); err == nil {
			samples[kernel] = append(samples[kernel], v)
		}
	}

	var out []plugin.Metric
	for _, name := range strings.Split(b.metrics, ",") {
		name = strings.TrimSpace(name)
		if v, ok := best[name]; ok {
			out = append(out, plugin.Metric{Name: name + " MB/s", Value: v, Samples: samples[name]})
		}
	}
	return out
}
