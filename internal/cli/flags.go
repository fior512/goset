package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"goset/internal/generic"
)

type Config struct {
	/* Target settings */
	Task []string // after "--"

	/* Thread selection */
	NThreads int
	Steering bool // (root) smp_affinity_list
	Cgroup   bool // (root, -n > 1) cgroup v2 cpuset
	Fence    bool // book the SMT siblings of the selected cpus in the cgroup

	/* Selection Preference */
	Include generic.CPUSet // subset of threads
	Exclude generic.CPUSet // subset of threads
	Numa    int            // -2 off, -1 auto, 0..N

	/* Telemetry */
	SamplingMS int // housekeeper (Not rankCPUs())

	/*DIAGNOSIS*/
	RmCgroup string

	/* META */
	Version bool
}

func isSudo() bool {
	return os.Geteuid() == 0
}

func (cfg *Config) Validate() error {
	/* Threads Selection */
	if cfg.NThreads < 1 {
		return fmt.Errorf(
			"-n: how many threads to book for the task.\n"+
				"  got %d, minimum is 1.\n"+
				"  use 1 to pin only, or >1 with -cgroup", cfg.NThreads)
	}
	if !cfg.Cgroup && cfg.NThreads > 1 {
		return fmt.Errorf(
			"-n >1 requires a cgroup-backed thread pool.\n" +
				"  add -cgroup (needs sudo)")
	}
	if cfg.Fence && !cfg.Cgroup {
		return fmt.Errorf(
			"-fence books the SMT siblings inside the cgroup cpuset.\n" +
				"  add -cgroup (needs sudo)")
	}
	if cfg.Steering && !isSudo() {
		return fmt.Errorf(
			"-steer avoids steerable IRQs happening over selected threads.\n" +
				"  this requires sudo (root)")
	}
	if cfg.Cgroup && !isSudo() {
		return fmt.Errorf(
			"-cgroup confines the task to the selected cpus.\n" +
				"  this requires sudo (root)")
	}

	/* Selection Preference */
	overlap := cfg.Include
	overlap.And(cfg.Exclude)
	if overlap.Any() {
		return fmt.Errorf(
			"-include forces cpus in, -exclude forces cpus out.\n"+
				"  overlap: cpu(s) %s appear in both", overlap.String())
	}
	if cfg.Numa < -2 {
		return fmt.Errorf(
			"-numa constrains the task cpus and memory to one numa node.\n"+
				"  got %d, only accepts -2: off (multi nodes), -1: auto (single, best), 0..N: single node index", cfg.Numa)
	}

	/* Telemetry */
	if cfg.SamplingMS < 0 {
		return fmt.Errorf(
			"-interval sets the housekeeper's telemetry poll period, ms.\n"+
				"  got %d, can't be negative.\n"+
				"  0: beginning/end only, >0: poll every N ms", cfg.SamplingMS)
	}

	if cfg.RmCgroup != "" && !strings.HasPrefix(cfg.RmCgroup, generic.CgroupIdentifier) {
		return fmt.Errorf(
			"-rm-cgroup removes a goset cgroup by name.\n"+
				"  %q does not start with %q, so it isn't a goset cgroup.\n"+
				"  check the name against the diagnose Cgroups table (bare goset call)",
			cfg.RmCgroup, generic.CgroupIdentifier)
	}

	return nil
}

// Register binds flags with struct items
func Register(fs *flag.FlagSet) *Config {
	cfg := &Config{}
	fs.IntVar(&cfg.NThreads, "n", 1, "how many threads to book, minimum 1")
	fs.BoolVar(&cfg.Steering, "steer", false, "set smp_affinity_list (sudo)")
	fs.BoolVar(&cfg.Cgroup, "cgroup", false, "confine the task to the selected cpus (sudo, required for -n > 1)")
	fs.BoolVar(&cfg.Fence, "fence", false, "book the SMT siblings of the selected cpus so no other task runs on the core")
	fs.Var(&cfg.Include, "include", "cpu list to force into the benchmark set, e.g. 2,4-6")
	fs.Var(&cfg.Exclude, "exclude", "cpu list to exclude from selection")
	fs.IntVar(&cfg.Numa, "numa", -2, "constrain task cpus and memory to one numa node: -2 off, -1 auto, 0..N index")
	fs.IntVar(&cfg.SamplingMS, "interval", 100, "interrupt sampling window for the housekeeper telemetry loop, ms (not used by cpu ranking)")
	fs.StringVar(&cfg.RmCgroup, "rm-cgroup", "", "remove a leaked goset cgroup by name (see diagnose Cgroups table)")
	fs.BoolVar(&cfg.Version, "version", false, "print goset version and exit")
	return cfg
}

func Parse(args []string) (*Config, error) {
	fs := flag.NewFlagSet("goset", flag.ContinueOnError)
	cfg := Register(fs)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	cfg.Task = fs.Args() // task binary argument
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
