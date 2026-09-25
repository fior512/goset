package generic

// paths
const (
	SysCPU         = "/sys/devices/system/cpu"
	SysNode        = "/sys/devices/system/node"
	SysCgroup      = "/sys/fs/cgroup"
	ProcRoot       = "/proc"
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

// cpufreq control files
const (
	CpufreqDir     = "cpufreq"
	ScalingDriver  = "scaling_driver"
	ScalingGov     = "scaling_governor"
	EnergyPref     = "energy_performance_preference"
	ScalingMinFreq = "scaling_min_freq"
	ScalingMaxFreq = "scaling_max_freq"
	ScalingCurFreq = "scaling_cur_freq"
)

// names
const (
	CgroupIdentifier = "goset-"
	LogPrefix        = "[GOSET]: "
)

// counter identity
const (
	SourceIRQ      = "irq"
	SourceThrottle = "throttle"
	SourceFreq     = "freq"
	SourceSched    = "sched"
	SourceIRQSteer = "irq steer"
)

const (
	IRQSoft         = "soft"
	IRQHard         = "hard"
	ThrottleCount   = "count"
	FreqMin         = "min MHz"
	FreqMax         = "max MHz"
	FreqAvg         = "avg MHz"
	SchedRunDelay   = "run_delay"
	SchedMigrations = "nr_migrations"
	IRQSteerDrift   = "drift"
)

// global report keys
const (
	GlobalIRQBalanceHeld    = "irqbalance held"
	GlobalIRQSteerApplied   = "irq steer applied"
	GlobalIRQSteerRejected  = "irq steer rejected"
	GlobalIRQSteerRemaining = "irq steer remaining"
	GlobalIRQSteerDrift     = "irq steer drift"
	GlobalCtxswVoluntary    = "ctxsw voluntary"
	GlobalCtxswInvoluntary  = "ctxsw involuntary"
	GlobalMigrations        = "migrations"
	GlobalRunDelay          = "run_delay"
	GlobalWall              = "wall"
	GlobalExit              = "exit"
)
