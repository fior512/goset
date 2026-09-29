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
	RunLockDir     = "/run/lock"
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
	SteerLockName    = "goset-steer.lock"
	IRQBalanceComm   = "irqbalance"
	IRQBalanceUnit   = "irqbalance.service"
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
	IRQSteerable    = "steerable"
	IRQNonSteerable = "non-steerable"
	ThrottleCount   = "count"
	FreqMin         = "min"
	FreqMax         = "max"
	FreqAvg         = "avg"
	SchedRunDelay   = "run_delay"
	SchedMigrations = "nr_migrations"
	IRQSteerDrift   = "drift"
)

// telemetry columns
const (
	TelemetryCPU             = "cpu"
	TelemetryAll             = "all" // footer row: reduction over the task cpus
	TelemetryFreqMin         = "GHz min"
	TelemetryFreqAvg         = "GHz avg"
	TelemetryFreqMax         = "GHz max"
	TelemetryThrottle        = "throttle"
	TelemetryIRQSteerable    = "irq steerable"
	TelemetryIRQNonSteerable = "irq non-steerable"
)

// run report scopes
const (
	ScopeTask  = "task"
	ScopeSched = "sched"
	ScopeSteer = "steer"
)

// run report keys
const (
	RunWall              = "wall"
	RunExit              = "exit"
	RunSamples           = "samples"
	RunCtxswVoluntary    = "ctxsw voluntary"
	RunCtxswInvoluntary  = "ctxsw involuntary"
	RunMigrations        = "migrations"
	RunRunDelay          = "run_delay"
	RunIRQSteerApplied   = "applied"
	RunIRQSteerRejected  = "rejected"
	RunIRQSteerRemaining = "remaining"
	RunIRQSteerDrift     = "drift"
	RunIRQBalanceHeld    = "irqbalance held"
)
