// threadprobe is a mock task for goset's integration harness. It reports,
// per OS thread, the affinity the kernel actually gave it (Cpus_allowed_list)
// and the CPU it happened to run on, plus the process's cgroup membership.
// It never asserts anything itself; the harness reads its JSON stdout.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type ThreadInfo struct {
	TID      int    `json:"tid"`
	Allowed  string `json:"allowed"`  // Cpus_allowed_list from /proc/self/task/<tid>/status
	Observed int    `json:"observed"` // processor field from /proc/self/task/<tid>/stat
	Err      string `json:"err,omitempty"`
}

type Report struct {
	PID     int          `json:"pid"`
	Workers int          `json:"workers"`
	Cgroup  string       `json:"cgroup"` // raw /proc/self/cgroup
	Threads []ThreadInfo `json:"threads"`
}

func main() {
	workers := flag.Int("workers", 1, "number of OS threads to pin and report")
	flag.Parse()

	rep := Report{PID: os.Getpid(), Workers: *workers}
	if data, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		rep.Cgroup = strings.TrimSpace(string(data))
	}

	var wg sync.WaitGroup
	results := make(chan ThreadInfo, *workers)
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- probeThread()
		}()
	}
	wg.Wait()
	close(results)
	for info := range results {
		rep.Threads = append(rep.Threads, info)
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(rep); err != nil {
		fmt.Fprintln(os.Stderr, "threadprobe: encode:", err)
		os.Exit(1)
	}
}

// probeThread locks the calling goroutine to its OS thread and reads that
// thread's own /proc entries, so Allowed/Observed reflect the pin goset
// applied to this specific thread, not just the process.
func probeThread() ThreadInfo {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tid := syscall.Gettid()
	info := ThreadInfo{TID: tid}

	allowed, err := cpusAllowedList(tid)
	if err != nil {
		info.Err = err.Error()
		return info
	}
	info.Allowed = allowed

	observed, err := observedCPU(tid)
	if err != nil {
		info.Err = err.Error()
		return info
	}
	info.Observed = observed
	return info
}

func cpusAllowedList(tid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/self/task/%d/status", tid))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if name, val, ok := strings.Cut(line, ":"); ok && name == "Cpus_allowed_list" {
			return strings.TrimSpace(val), nil
		}
	}
	return "", fmt.Errorf("Cpus_allowed_list not found for tid %d", tid)
}

// observedCPU reads field 39 (processor, 1-indexed) of /proc/self/task/<tid>/stat.
func observedCPU(tid int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/self/task/%d/stat", tid))
	if err != nil {
		return 0, err
	}
	// Field 2 (comm) is parenthesized and may contain spaces; split after it.
	_, rest, ok := strings.Cut(string(data), ") ")
	if !ok {
		return 0, fmt.Errorf("unexpected stat format for tid %d", tid)
	}
	fields := strings.Fields(rest)
	const processorField = 39 - 3 // fields[] is 0-indexed from field 3 onward
	if len(fields) <= processorField {
		return 0, fmt.Errorf("stat for tid %d has %d fields after comm, want > %d", tid, len(fields), processorField)
	}
	return strconv.Atoi(fields[processorField])
}
