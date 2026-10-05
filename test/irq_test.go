package integration

import (
	"os"
	"path/filepath"
	"testing"

	"goset/internal/generic"
	"goset/internal/telemetry"
)

func writeInterrupts(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, generic.ProcInterruptsName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const interruptsTwoCpus = "           CPU0    CPU1\n  0:         10      20\n NMI:         1       2\n"
const interruptsTwoCpusLater = "           CPU0    CPU1\n  0:        110     220\n NMI:         3       4\n"

func irqValue(t *testing.T, src *telemetry.IRQSource, cpu int, name string) float64 {
	t.Helper()
	for _, counter := range src.Summary() {
		if counter.CPU == cpu && counter.Name == name {
			return counter.Value
		}
	}
	t.Fatalf("no %s counter for cpu %d", name, cpu)
	return 0
}

func TestIRQReportsTheDeltaPerCPU(t *testing.T) {
	root := t.TempDir()
	writeInterrupts(t, root, interruptsTwoCpus)

	var cpus generic.CPUSet
	cpus.SetBit(0)
	cpus.SetBit(1)
	src := &telemetry.IRQSource{Root: root}
	if err := src.Baseline(cpus); err != nil {
		t.Fatal(err)
	}
	writeInterrupts(t, root, interruptsTwoCpusLater)
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}

	for cpu, want := range map[int]float64{0: 100, 1: 200} {
		if got := irqValue(t, src, cpu, generic.IRQSteerable); got != want {
			t.Errorf("cpu %d steerable = %v, want %v", cpu, got, want)
		}
	}
	for cpu, want := range map[int]float64{0: 2, 1: 2} {
		if got := irqValue(t, src, cpu, generic.IRQNonSteerable); got != want {
			t.Errorf("cpu %d non-steerable = %v, want %v", cpu, got, want)
		}
	}
}

func TestIRQReportsZeroForACPUWithoutAColumn(t *testing.T) {
	root := t.TempDir()
	writeInterrupts(t, root, interruptsTwoCpus)

	var cpus generic.CPUSet
	cpus.SetBit(0)
	cpus.SetBit(7) // /proc/interrupts holds no CPU7 column
	src := &telemetry.IRQSource{Root: root}
	if err := src.Baseline(cpus); err != nil {
		t.Fatal(err)
	}
	writeInterrupts(t, root, interruptsTwoCpusLater)
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}

	if got := irqValue(t, src, 0, generic.IRQSteerable); got != 100 {
		t.Errorf("cpu 0 steerable = %v, want 100", got)
	}
	if got := irqValue(t, src, 7, generic.IRQSteerable); got != 0 {
		t.Errorf("cpu 7 steerable = %v, want 0: a cpu the file holds no column for reads 0", got)
	}
}

func TestIRQReportsZeroWhenTheEndReadFails(t *testing.T) {
	root := t.TempDir()
	writeInterrupts(t, root, interruptsTwoCpus)

	var cpus generic.CPUSet
	cpus.SetBit(0)
	src := &telemetry.IRQSource{Root: root}
	if err := src.Baseline(cpus); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, generic.ProcInterruptsName)); err != nil {
		t.Fatal(err)
	}
	if err := src.Stop(); err == nil {
		t.Fatal("Stop on an unreadable interrupts file, want an error")
	}
	if got := irqValue(t, src, 0, generic.IRQSteerable); got != 0 {
		t.Errorf("steerable = %v after a failed end read, want 0", got)
	}
}
