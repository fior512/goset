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
	Cgroup   bool // (root) cgroup v2 cpuset
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

func (config *Config) Validate() error {
	/* Threads Selection */
	if config.NThreads < 1 {
		return fmt.Errorf(
			"-n: how many threads to book for the task.\n"+
				"  got %d, minimum is 1", config.NThreads)
	}
	if config.Fence && !config.Cgroup {
		return fmt.Errorf(
			"-fence books the SMT siblings inside the cgroup cpuset.\n" +
				"  add -cgroup (needs sudo)")
	}
	if config.Steering && !isSudo() {
		return fmt.Errorf(
			"-steer avoids steerable IRQs happening over selected threads.\n" +
				"  this requires sudo (root)")
	}
	if config.Cgroup && !isSudo() {
		return fmt.Errorf(
			"-cgroup confines the task to the selected cpus.\n" +
				"  this requires sudo (root)")
	}

	/* Selection Preference */
	overlap := config.Include
	overlap.And(config.Exclude)
	if overlap.Any() {
		return fmt.Errorf(
			"-include forces cpus in, -exclude forces cpus out.\n"+
				"  overlap: cpu(s) %s appear in both", overlap.String())
	}
	if config.Numa < -2 {
		return fmt.Errorf(
			"-numa constrains the task cpus and memory to one numa node.\n"+
				"  got %d, only accepts -2: off (multi nodes), -1: auto (single, best), 0..N: single node index", config.Numa)
	}

	/* Telemetry */
	if config.SamplingMS < 0 {
		return fmt.Errorf(
			"-interval sets the housekeeper's telemetry poll period, ms.\n"+
				"  got %d, can't be negative.\n"+
				"  0: beginning/end only, >0: poll every N ms", config.SamplingMS)
	}

	if config.RmCgroup != "" && !strings.HasPrefix(config.RmCgroup, generic.CgroupIdentifier) {
		return fmt.Errorf(
			"-rm-cgroup removes a goset cgroup by name.\n"+
				"  %q does not start with %q, so it isn't a goset cgroup.\n"+
				"  check the name against the diagnose Cgroups table (bare goset call)",
			config.RmCgroup, generic.CgroupIdentifier)
	}

	return nil
}

// Register binds flags with struct items
func Register(flags *flag.FlagSet) *Config {
	config := &Config{}
	flags.IntVar(&config.NThreads, "n", 1, "how many threads to book, minimum 1")
	flags.BoolVar(&config.Steering, "steer", false, "set smp_affinity_list (sudo)")
	flags.BoolVar(&config.Cgroup, "cgroup", false, "confine the task to the selected cpus (sudo)")
	flags.BoolVar(&config.Fence, "fence", false, "book the SMT siblings of the selected cpus so no other task runs on the core")
	flags.Var(&config.Include, "include", "cpu list to force into the benchmark set, e.g. 2,4-6")
	flags.Var(&config.Exclude, "exclude", "cpu list to exclude from selection")
	flags.IntVar(&config.Numa, "numa", -2, "constrain task cpus and memory to one numa node: -2 off, -1 auto, 0..N index")
	flags.IntVar(&config.SamplingMS, "interval", 100, "interrupt sampling window for the housekeeper telemetry loop, ms (not used by cpu ranking)")
	flags.StringVar(&config.RmCgroup, "rm-cgroup", "", "remove a leaked goset cgroup by name (see diagnose Cgroups table)")
	flags.BoolVar(&config.Version, "version", false, "print goset version and exit")
	return config
}

func Parse(args []string) (*Config, error) {
	flags := flag.NewFlagSet("goset", flag.ContinueOnError)
	config := Register(flags)
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	config.Task = flags.Args() // task binary argument
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return config, nil
}
