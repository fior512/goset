package isolation

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goset/internal/generic"
)

type Cgroup struct {
	Path          string
	Name          string
	Partition     string   // kernel cpuset.cpus.partition: "member", "root", or "isolated"
	File          *os.File // 0o644, owner: read/write, else: read
	cpusetEnabled bool     // root subtree_control gained cpuset from this cgroup
	destroyed     bool
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func cgroupV2Available() bool {
	_, err := os.Stat(filepath.Join(generic.SysCgroup, generic.CgroupControllers))
	return err == nil
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func InitCgroup(name string, cpus generic.CPUSet, numa int) (*Cgroup, error) {
	// prerequirements: check v2 and cpuset
	if !cgroupV2Available() {
		return nil, fmt.Errorf("cgroup v2 unified hierarchy not found at %s", generic.SysCgroup)
	}
	ctl, err := generic.ReadFileTrim(filepath.Join(generic.SysCgroup, generic.CgroupControllers))
	if err != nil {
		return nil, err
	}
	if !strings.Contains(ctl, "cpuset") {
		return nil, fmt.Errorf("cpuset controller not present in root cgroup.controllers (%q)", ctl)
	}

	// Enable cpuset on the root subtree
	sub := filepath.Join(generic.SysCgroup, generic.CgroupSubtreeControl)
	cpusetEnabled := false
	if cur, err := generic.ReadFileTrim(sub); err == nil && !strings.Contains(cur, "cpuset") {
		if err := os.WriteFile(sub, []byte("+cpuset"), 0o644); err != nil {
			return nil, fmt.Errorf("enable cpuset in root subtree_control: %w", err)
		}
		cpusetEnabled = true
	}

	// Create the cgroup directory
	path := filepath.Join(generic.SysCgroup, filepath.Base(name))
	cgroup := &Cgroup{Path: path, Name: name, cpusetEnabled: cpusetEnabled}
	switch err := os.Mkdir(path, 0o755); {
	case err == nil:
	case os.IsExist(err):
		if err := cgroup.verifyLeftover(); err != nil {
			_ = cgroup.restoreSubtreeControl()
			return nil, err
		}
	default:
		_ = cgroup.restoreSubtreeControl()
		return nil, fmt.Errorf("mkdir %s: %w", path, err)
	}

	// Assign the pinned CPUs
	list := cpus.String()
	if err := os.WriteFile(filepath.Join(path, generic.CpusetCpus), []byte(list), 0o644); err != nil {
		cgroup.Destroy()
		return nil, fmt.Errorf("write cpuset.cpus: %w", err)
	}
	if numa >= 0 { // mems is optional
		if err := os.WriteFile(filepath.Join(path, generic.CpusetMems), []byte(strconv.Itoa(numa)), 0o644); err != nil {
			cgroup.Destroy()
			return nil, fmt.Errorf("write cpuset.mems: %w", err)
		}
	}
	// absent on older kernels
	_ = os.WriteFile(filepath.Join(path, generic.CpusetCpusExclusive), []byte(list), 0o644)

	// Try isolated, fall back to root
	partitionPath := filepath.Join(path, generic.CpusetCpusPartition)
	partitionType := "isolated" // avoid load balancing
	if cpus.Count() > 1 {
		// keep load balancing inside control cgroup
		partitionType = "root"
	}
	if err := os.WriteFile(partitionPath, []byte(partitionType), 0o644); err != nil {
		_ = os.WriteFile(partitionPath, []byte("root"), 0o644)
	}
	if partition, err := generic.ReadFileTrim(partitionPath); err == nil {
		cgroup.Partition = partition
		if strings.Contains(partition, "invalid") {
			cgroup.Destroy()
			return nil, fmt.Errorf("cpuset.cpus.partition: kernel reports %q", partition)
		}
	}

	file, err := os.Open(path)
	if err != nil {
		cgroup.Destroy()
		return nil, fmt.Errorf("open cgroup dir: %w", err)
	}
	cgroup.File = file // Keeping file descriptor open
	return cgroup, nil
}

// Diagnosis path
type CgroupInfo struct {
	Name      string
	Cpus      string
	Mems      string
	Partition string
	Procs     int
	Stat      map[string]uint64
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func ListCgroups() ([]CgroupInfo, error) {
	entries, err := os.ReadDir(generic.SysCgroup)
	if err != nil {
		return nil, err
	}
	var out []CgroupInfo
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), generic.CgroupIdentifier) {
			continue
		}

		path := filepath.Join(generic.SysCgroup, entry.Name())
		cpus, _ := generic.ReadFileTrim(filepath.Join(path, generic.CpusetCpus))
		mems, _ := generic.ReadFileTrim(filepath.Join(path, generic.CpusetMems))
		partition, _ := generic.ReadFileTrim(filepath.Join(path, generic.CpusetCpusPartition))
		procs, _ := generic.ReadFileTrim(filepath.Join(path, generic.CgroupProcs))
		cgroup := &Cgroup{Path: path}
		out = append(out, CgroupInfo{
			Name:      entry.Name(),
			Cpus:      cpus,
			Mems:      mems,
			Partition: partition,
			Procs:     len(strings.Fields(procs)),
			Stat:      cgroup.CPUStat(),
		})
	}
	return out, nil
}

func (cgroup *Cgroup) FD() int {
	if cgroup == nil || cgroup.File == nil {
		return -1
	}
	return int(cgroup.File.Fd())
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func (cgroup *Cgroup) CPUStat() map[string]uint64 {
	out := map[string]uint64{}
	if cgroup == nil {
		return out
	}
	data, err := os.ReadFile(filepath.Join(cgroup.Path, "cpu.stat"))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			out[fields[0]] = val
		}
	}
	return out
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func (cgroup *Cgroup) Destroy() error {
	if cgroup == nil || cgroup.destroyed {
		return nil
	}
	if cgroup.File != nil {
		cgroup.File.Close()
		cgroup.File = nil
	}
	if cgroup.Partition != "" && cgroup.Partition != "member" { // Restore to member before removal
		if err := os.WriteFile(filepath.Join(cgroup.Path, generic.CpusetCpusPartition), []byte("member"), 0o644); err != nil {
			return fmt.Errorf("restore %s to member: %w", cgroup.Path, err)
		}
	}
	if err := os.Remove(cgroup.Path); err != nil {
		return err
	}
	cgroup.destroyed = true
	return cgroup.restoreSubtreeControl()
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func (cgroup *Cgroup) verifyLeftover() error {
	if !strings.HasPrefix(filepath.Base(cgroup.Name), generic.CgroupIdentifier) {
		return fmt.Errorf("%s exists and is not a %s cgroup", cgroup.Path, generic.CgroupIdentifier)
	}
	procs, err := generic.ReadFileTrim(filepath.Join(cgroup.Path, generic.CgroupProcs))
	if err != nil {
		return fmt.Errorf("read %s in %s: %w", generic.CgroupProcs, cgroup.Path, err)
	}
	if tasks := strings.Fields(procs); len(tasks) > 0 {
		return fmt.Errorf(
			"%s holds %d task(s) and is not a leftover.\n"+
				"  pids: %s\n"+
				"  check them, then remove the cgroup with -rm-cgroup",
			cgroup.Path, len(tasks), taskPids(tasks))
	}
	return nil
}

// one pid per thread
func taskPids(tasks []string) string {
	const shown = 8
	if len(tasks) <= shown {
		return strings.Join(tasks, " ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(tasks[:shown], " "), len(tasks)-shown)
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func (cgroup *Cgroup) restoreSubtreeControl() error {
	if cgroup == nil || !cgroup.cpusetEnabled {
		return nil
	}
	cgroup.cpusetEnabled = false
	sub := filepath.Join(generic.SysCgroup, generic.CgroupSubtreeControl)
	if err := os.WriteFile(sub, []byte("-cpuset"), 0o644); err != nil {
		return fmt.Errorf("disable cpuset in root subtree_control: %w", err)
	}
	return nil
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func RemoveCgroup(name string) error {
	path := filepath.Join(generic.SysCgroup, filepath.Base(name))
	partition, _ := generic.ReadFileTrim(filepath.Join(path, generic.CpusetCpusPartition))
	cgroup := &Cgroup{Path: path, Name: name, Partition: partition}
	return cgroup.Destroy()
}
