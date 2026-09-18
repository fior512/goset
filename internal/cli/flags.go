package cli

import (
	"flag"
	"fmt"
	"os"

	"goset/internal/cpu"
)

type Config struct {
	/* Target settings */
	Task []string // after "--"

	/* Thread selection */
	NThreads int
	Steering bool // (root) smp_affinity_list
	Cgroup   bool // (root if >1) thread amount

	/* Selection Preference */
	Include  cpu.CPUSet // subset of threads
	Exclude  cpu.CPUSet // subset of threads
	NumaNode int        // Numa node

	/* Telemetry */
	SamplingMS int // housekeeper (Not rankCPUs())

	/*DIAGNOSIS*/
	RmCgroup string
}


func isSudo() bool {
	return os.Geteuid() == 0
}


func (cfg *Config) Validate() error { // definition, bad behavior, suggestion
	/* Threads Selection */
	if cfg.NThreads < 1 {
		return fmt.Errorf("-n define how many threads to book, it can't be below 1.\n  1: pin only, >1: requires -cgroup")
	}
	if !cfg.Cgroup && cfg.NThreads > 1 {
		return fmt.Errorf("-n >1 requires -cgroup (sudo)")
	}
	if cfg.Steering && !isSudo() {
		return fmt.Errorf("-steer needs root")
	}
	if cfg.Cgroup && !isSudo() {
		return fmt.Errorf("-cgroup requires root")
	}

	/* Selection Preference */
	overlap := cfg.Include
	overlap.And(cfg.Exclude)
	if overlap.Any() {
		return fmt.Errorf("include and exclude can't overlap")
	}
	if cfg.NumaNode < -2 {
		return fmt.Errorf("-node can be -2:off(default), -1:Auto, 0..:node idx")
	}

	/* Telemetry */
	if cfg.SamplingMS < 0 {
		return fmt.Errorf("-interval can't be negative; 0: before/after only")
	}
	return nil
}


// Register binds flags with struct items
func Register(fs *flag.FlagSet) *Config {
	cfg := &Config{}
	fs.IntVar(&cfg.NThreads, "n", 1, "how many threads to book, minimum 1")
	fs.BoolVar(&cfg.Steering, "steer", false, "set smp_affinity_list (sudo)")
	fs.BoolVar(&cfg.Cgroup, "cgroup", false, "number of threads for cgroup containerization (sudo)")
	fs.Var(&cfg.Include, "include", "cpu list to force into the benchmark set, e.g. 2,4-6")
	fs.Var(&cfg.Exclude, "exclude", "cpu list to exclude from selection")
	fs.IntVar(&cfg.NumaNode, "node", -2, "constrain benchmark cpus to one NUMA node when possible")
	fs.IntVar(&cfg.SamplingMS, "interval", 1000, "interrupt sampling window for the housekeeper telemetry loop, ms (not used by cpu ranking)")
	fs.StringVar(&cfg.RmCgroup, "rm-cgroup", "", "remove a leaked goset cgroup by name (see diagnose Cgroups table)")
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
