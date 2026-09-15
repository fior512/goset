package isolation

/*
Process:
check v2, enable cpuset, mkdir, write cpuset.cpus,
set partition mode (isolated, else root fallback), open dir FD.
Destroy() restores "member" mode and removes the dir
*/

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goset/internal/cpu"
)

type Cgroup struct {
	Path      string
	Name      string
	Partition string   // kernel cpuset.cpus.partition: "member", "root", or "isolated"
	File      *os.File // 0o644, owner: read/write, else: read
}


func cgroupV2Available() bool {
	_, err := os.Stat(filepath.Join("/sys/fs/cgroup", "cgroup.controllers"))
	return err == nil
}


func readFileTrim(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}


func InitCgroup(name string, cpus cpu.CPUSet, memNode int) (*Cgroup, error) {
	// prerequirements: check v2 and cpuset
	if !cgroupV2Available() {
		return nil, fmt.Errorf("cgroup v2 unified hierarchy not found at %s", "/sys/fs/cgroup")
	}
	ctl, err := readFileTrim(filepath.Join("/sys/fs/cgroup", "cgroup.controllers"))
	if err != nil {
		return nil, err
	}
	if !strings.Contains(ctl, "cpuset") {
		return nil, fmt.Errorf("cpuset controller not present in root cgroup.controllers (%q)", ctl)
	}

	// Enable cpuset on the root subtree
	sub := filepath.Join("/sys/fs/cgroup", "cgroup.subtree_control")
	if cur, err := readFileTrim(sub); err == nil && !strings.Contains(cur, "cpuset") {
		if err := os.WriteFile(sub, []byte("+cpuset"), 0o644); err != nil {
			return nil, fmt.Errorf("enable cpuset in root subtree_control: %w", err)
		}
	}

	// Create the group directory
	path := filepath.Join("/sys/fs/cgroup", name)
	if err := os.Mkdir(path, 0o755); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("mkdir %s: %w", path, err)
	}
	group := &Cgroup{Path: path, Name: name}

	// Assign the pinned CPUs
	list := cpus.String()
	if err := os.WriteFile(filepath.Join(path, "cpuset.cpus"), []byte(list), 0o644); err != nil {
		group.Destroy()
		return nil, fmt.Errorf("write cpuset.cpus: %w", err)
	}
	if memNode >= 0 { // mems is optional
		_ = os.WriteFile(filepath.Join(path, "cpuset.mems"), []byte(strconv.Itoa(memNode)), 0o644)
	}
	// absent on older kernels
	_ = os.WriteFile(filepath.Join(path, "cpuset.cpus.exclusive"), []byte(list), 0o644)

	// Try isolated, fall back to root
	if err := os.WriteFile(filepath.Join(path, "cpuset.cpus.partition"), []byte("isolated"), 0o644); err != nil {
		_ = os.WriteFile(filepath.Join(path, "cpuset.cpus.partition"), []byte("root"), 0o644)
	}
	if v, err := readFileTrim(filepath.Join(path, "cpuset.cpus.partition")); err == nil {
		group.Partition = v
	}

	file, err := os.Open(path)
	if err != nil {
		group.Destroy()
		return nil, fmt.Errorf("open cgroup dir: %w", err)
	}
	group.File = file // Keeping file descriptor open
	return group, nil
}


func (group *Cgroup) FD() int {
	if group == nil || group.File == nil {
		return -1
	}
	return int(group.File.Fd())
}


func (group *Cgroup) CPUStat() map[string]uint64 {
	out := map[string]uint64{}
	if group == nil {
		return out
	}
	b, err := os.ReadFile(filepath.Join(group.Path, "cpu.stat"))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		if v, err := strconv.ParseUint(f[1], 10, 64); err == nil {
			out[f[0]] = v
		}
	}
	return out
}


func (group *Cgroup) Destroy() {
	if group == nil {
		return
	}
	if group.File != nil {
		group.File.Close()
		group.File = nil
	}
	if group.Partition != "" && group.Partition != "member" { // Restore to member before removal
		_ = os.WriteFile(filepath.Join(group.Path, "cpuset.cpus.partition"), []byte("member"), 0o644)
	}
	_ = os.Remove(group.Path)
}
