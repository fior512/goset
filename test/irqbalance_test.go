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
	if got := steer.Status(); got != "absent" {
		t.Errorf("Status() = %q, want \"absent\"", got)
	}
}

func TestSteeringStatus(t *testing.T) {
	notRunning := &isolation.Steering{}
	if got := notRunning.Status(); got != "absent" {
		t.Errorf("not running Status() = %q, want \"absent\"", got)
	}
	held := &isolation.Steering{WasRunning: true, ViaSystemd: true}
	if got := held.Status(); got != "held" {
		t.Errorf("held Status() = %q, want \"held\"", got)
	}
	unmanaged := &isolation.Steering{WasRunning: true, Note: "unmanaged, still running pid 42"}
	if got := unmanaged.Status(); got != "unmanaged, still running pid 42" {
		t.Errorf("unmanaged Status() = %q, want \"unmanaged, still running pid 42\"", got)
	}
	survived := &isolation.Steering{WasRunning: true, ViaSystemd: true, Note: "still running pid 812"}
	if got := survived.Status(); got != "still running pid 812" {
		t.Errorf("survived Status() = %q, want the note", got)
	}
}
