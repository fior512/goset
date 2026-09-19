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
	Path      string
	Name      string
	Partition string   // kernel cpuset.cpus.partition: "member", "root", or "isolated"
	File      *os.File // 0o644, owner: read/write, else: read
	destroyed bool
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func cgroupV2Available() bool {
	_, err := os.Stat(filepath.Join(generic.SysCgroup, generic.CgroupControllers))
	return err == nil
}

func readFileTrim(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func InitCgroup(name string, cpus generic.CPUSet, memNode int) (*Cgroup, error) {
	// prerequirements: check v2 and cpuset
	if !cgroupV2Available() {
		return nil, fmt.Errorf("cgroup v2 unified hierarchy not found at %s", generic.SysCgroup)
	}
	ctl, err := readFileTrim(filepath.Join(generic.SysCgroup, generic.CgroupControllers))
	if err != nil {
		return nil, err
	}
	if !strings.Contains(ctl, "cpuset") {
		return nil, fmt.Errorf("cpuset controller not present in root cgroup.controllers (%q)", ctl)
	}

	// Enable cpuset on the root subtree
	sub := filepath.Join(generic.SysCgroup, generic.CgroupSubtreeControl)
	if cur, err := readFileTrim(sub); err == nil && !strings.Contains(cur, "cpuset") {
		if err := os.WriteFile(sub, []byte("+cpuset"), 0o644); err != nil {
			return nil, fmt.Errorf("enable cpuset in root subtree_control: %w", err)
		}
	}

	// Create the group directory
	path := filepath.Join(generic.SysCgroup, filepath.Base(name))
	if err := os.Mkdir(path, 0o755); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("mkdir %s: %w", path, err)
	}
	group := &Cgroup{Path: path, Name: name}

	// Assign the pinned CPUs
	list := cpus.String()
	if err := os.WriteFile(filepath.Join(path, generic.CpusetCpus), []byte(list), 0o644); err != nil {
		group.Destroy()
		return nil, fmt.Errorf("write cpuset.cpus: %w", err)
	}
	if memNode >= 0 { // mems is optional
		_ = os.WriteFile(filepath.Join(path, generic.CpusetMems), []byte(strconv.Itoa(memNode)), 0o644)
	}
	// absent on older kernels
	_ = os.WriteFile(filepath.Join(path, generic.CpusetCpusExclusive), []byte(list), 0o644)

	// Try isolated, fall back to root
	partitionPath := filepath.Join(path, generic.CpusetCpusPartition)
	if err := os.WriteFile(partitionPath, []byte("isolated"), 0o644); err != nil {
		_ = os.WriteFile(partitionPath, []byte("root"), 0o644)
	}
	if partition, err := readFileTrim(partitionPath); err == nil {
		group.Partition = partition
		if strings.Contains(partition, "invalid") {
			group.Destroy()
			return nil, fmt.Errorf("cpuset.cpus.partition: kernel reports %q", partition)
		}
	}

	file, err := os.Open(path)
	if err != nil {
		group.Destroy()
		return nil, fmt.Errorf("open cgroup dir: %w", err)
	}
	group.File = file // Keeping file descriptor open
	return group, nil
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
		cpus, _ := readFileTrim(filepath.Join(path, generic.CpusetCpus))
		mems, _ := readFileTrim(filepath.Join(path, generic.CpusetMems))
		partition, _ := readFileTrim(filepath.Join(path, generic.CpusetCpusPartition))
		procs, _ := readFileTrim(filepath.Join(path, generic.CgroupProcs))
		group := &Cgroup{Path: path}
		out = append(out, CgroupInfo{
			Name:      entry.Name(),
			Cpus:      cpus,
			Mems:      mems,
			Partition: partition,
			Procs:     len(strings.Fields(procs)),
			Stat:      group.CPUStat(),
		})
	}
	return out, nil
}

func (group *Cgroup) FD() int {
	if group == nil || group.File == nil {
		return -1
	}
	return int(group.File.Fd())
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func (group *Cgroup) CPUStat() map[string]uint64 {
	out := map[string]uint64{}
	if group == nil {
		return out
	}
	data, err := os.ReadFile(filepath.Join(group.Path, "cpu.stat"))
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
func (group *Cgroup) Destroy() error {
	if group == nil || group.destroyed {
		return nil
	}
	if group.File != nil {
		group.File.Close()
		group.File = nil
	}
	if group.Partition != "" && group.Partition != "member" { // Restore to member before removal
		if err := os.WriteFile(filepath.Join(group.Path, generic.CpusetCpusPartition), []byte("member"), 0o644); err != nil {
			return fmt.Errorf("restore %s to member: %w", group.Path, err)
		}
	}
	if err := os.Remove(group.Path); err != nil {
		return err
	}
	group.destroyed = true
	return nil
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func RemoveCgroup(name string) error {
	path := filepath.Join(generic.SysCgroup, filepath.Base(name))
	partition, _ := readFileTrim(filepath.Join(path, generic.CpusetCpusPartition))
	group := &Cgroup{Path: path, Name: name, Partition: partition}
	return group.Destroy()
}
