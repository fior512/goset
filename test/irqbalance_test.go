package integration

import (
	"os"
	"path/filepath"
	"testing"

	"goset/internal/isolation"
)

func TestScanProcCommFindsProcess(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "1234"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "1234", "comm"), []byte("irqbalance\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pid, err := isolation.ScanProcComm(root, "irqbalance")
	if err != nil {
		t.Fatalf("ScanProcComm: %v", err)
	}
	if pid != 1234 {
		t.Errorf("pid = %d, want 1234", pid)
	}
}

func TestScanProcCommAbsent(t *testing.T) {
	pid, err := isolation.ScanProcComm(t.TempDir(), "irqbalance")
	if err != nil {
		t.Fatalf("ScanProcComm: %v", err)
	}
	if pid != 0 {
		t.Errorf("pid = %d, want 0", pid)
	}
}

func TestScanProcCommSkipsNonNumeric(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "self"), 0o755); err != nil {
		t.Fatal(err)
	}
	pid, err := isolation.ScanProcComm(root, "irqbalance")
	if err != nil {
		t.Fatalf("ScanProcComm: %v", err)
	}
	if pid != 0 {
		t.Errorf("pid = %d, want 0", pid)
	}
}

func TestSteeringNilSafe(t *testing.T) {
	var steer *isolation.Steering
	steer.Release()
	if got := steer.Status(); got != "no" {
		t.Errorf("Status() = %q, want \"no\"", got)
	}
}

func TestSteeringStatus(t *testing.T) {
	notRunning := &isolation.Steering{}
	if got := notRunning.Status(); got != "no" {
		t.Errorf("not running Status() = %q, want \"no\"", got)
	}
	held := &isolation.Steering{WasRunning: true, ViaSystemd: true}
	if got := held.Status(); got != "yes" {
		t.Errorf("held Status() = %q, want \"yes\"", got)
	}
	unmanaged := &isolation.Steering{WasRunning: true, Note: "unmanaged pid 42"}
	if got := unmanaged.Status(); got != "unmanaged pid 42" {
		t.Errorf("unmanaged Status() = %q, want \"unmanaged pid 42\"", got)
	}
}
