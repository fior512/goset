package generic

// paths
const (
	SysCPU         = "/sys/devices/system/cpu"
	SysNode        = "/sys/devices/system/node"
	SysCgroup      = "/sys/fs/cgroup"
	ProcIRQ        = "/proc/irq"
	ProcInterrupts = "/proc/interrupts"
	ProcCmd        = "/proc/cmdline"
)

// cgroup v2 control files
const (
	CgroupControllers    = "cgroup.controllers"
	CgroupSubtreeControl = "cgroup.subtree_control"
	CgroupProcs          = "cgroup.procs"
	CpusetCpus           = "cpuset.cpus"
	CpusetMems           = "cpuset.mems"
	CpusetCpusPartition  = "cpuset.cpus.partition"
	CpusetCpusExclusive  = "cpuset.cpus.exclusive"
)

// irq control files
const (
	SmpAffinityList       = "smp_affinity_list"
	EffectiveAffinityList = "effective_affinity_list"
)

// names
const (
	CgroupIdentifier = "goset-"
)
