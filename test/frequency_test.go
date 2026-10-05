package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"goset/internal/generic"
	"goset/internal/telemetry"
)

// scaling_cur_freq holds kHz
func writeCurFreq(t *testing.T, root string, cpu, khz int) {
	t.Helper()
	dir := filepath.Join(root, fmt.Sprintf("cpu%d", cpu), generic.CpufreqDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, generic.ScalingCurFreq), []byte(fmt.Sprintf("%d\n", khz)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func freqValue(t *testing.T, src *telemetry.FreqSource, cpu int, name string) float64 {
	t.Helper()
	for _, counter := range src.Summary() {
		if counter.CPU == cpu && counter.Name == name {
			return counter.Value
		}
	}
	t.Fatalf("no %s counter for cpu %d", name, cpu)
	return 0
}

func TestFreqReadsSamplesInHz(t *testing.T) {
	root := t.TempDir()
	var cpus generic.CPUSet
	cpus.SetBit(3)
	writeCurFreq(t, root, 3, 2400000)

	src := &telemetry.FreqSource{Root: root}
	if err := src.Baseline(cpus); err != nil {
		t.Fatal(err)
	}
	for _, khz := range []int{2400000, 3600000} {
		writeCurFreq(t, root, 3, khz)
		if err := src.Poll(); err != nil {
			t.Fatal(err)
		}
	}
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}

	for name, want := range map[string]float64{
		generic.FreqMin: 2.4e9,
		generic.FreqMax: 3.6e9,
		generic.FreqAvg: 3e9,
	} {
		if got := freqValue(t, src, 3, name); got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}

func TestFreqReportsZeroWithoutASample(t *testing.T) {
	root := t.TempDir()
	var cpus generic.CPUSet
	cpus.SetBit(3)
	writeCurFreq(t, root, 3, 2400000)

	src := &telemetry.FreqSource{Root: root}
	if err := src.Baseline(cpus); err != nil {
		t.Fatal(err)
	}
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{generic.FreqMin, generic.FreqAvg, generic.FreqMax} {
		if got := freqValue(t, src, 3, name); got != 0 {
			t.Errorf("%s = %v after no poll, want 0: a run that never sampled reads 0", name, got)
		}
	}
}

func TestFreqReportsZeroOnACPUWithoutCpufreq(t *testing.T) {
	root := t.TempDir()
	var cpus generic.CPUSet
	cpus.SetBit(1)
	cpus.SetBit(7)
	writeCurFreq(t, root, 1, 2400000) // cpu 7 has no cpufreq dir

	src := &telemetry.FreqSource{Root: root}
	if err := src.Baseline(cpus); err != nil {
		t.Fatal(err)
	}
	writeCurFreq(t, root, 1, 2400000)
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}

	if got := freqValue(t, src, 1, generic.FreqAvg); got != 2.4e9 {
		t.Errorf("cpu 1 avg = %v, want 2.4e9", got)
	}
	if got := freqValue(t, src, 7, generic.FreqAvg); got != 0 {
		t.Errorf("cpu 7 avg = %v, want 0: a cpu the kernel does not expose reads 0", got)
	}
}
