package cli

import (
	"flag"
	"fmt"
	"goset/internal/cpu"
	"os"
)

type Config struct {
	/* Target settings */
	Cmd []string // after "--"

	/* Thread selection */
	SetAffinity bool // (root) smp_affinity_list
	Cgroup      int  // (root) thread amount

	/* Selection Preference */
	Include    cpu.CPUSet // subset of threads
	Exclude    cpu.CPUSet // subset of threads
	PreferNode bool       // Numa node

	/* Telemetry */
	SamplingMS int // time window for telemetry
}


func isSudo() bool {
	return os.Geteuid() == 0
}


func (cfg *Config) Validate() error {
	if len(cfg.Cmd) == 0 {
		return fmt.Errorf("-pin requires a target after --")
	}
	/* Threads Selection */
	if cfg.SetAffinity && !isSudo() {
		return fmt.Errorf("-set-affinity needs root")
	}
	if cfg.Cgroup > 0 && !isSudo() {
		return fmt.Errorf("-cgroup requires root")
	}
	if cfg.Cgroup < 0 {
		return fmt.Errorf("-cgroup can't be negative")
	}

	/* Selection Preference */
	if cfg.Exclude.IsSubset(cfg.Include) {
		return fmt.Errorf("include and exclude can't overlap")
	}

	
	/* Telemetry */
	if cfg.SamplingMS < -1 {
		return fmt.Errorf("-sampling-ms can't be inferior to -1; -1: desactivated, 0:before/after only")
	}
	return nil
}


// Register binds flags with struct items
func Register(fs *flag.FlagSet) *Config { 
	cfg := &Config{}
	fs.BoolVar(&cfg.SetAffinity, "set-affinity", false, "set smp_affinity_list (sudo)")
	fs.IntVar(&cfg.Cgroup, "cgroup", 0, "number of threads for cgroup containerization (sudo)")
	fs.Var(&cfg.Include, "include", "cpu list to force into the benchmark set, e.g. 2,4-6")
	fs.Var(&cfg.Exclude, "exclude", "cpu list to exclude from selection")
	fs.BoolVar(&cfg.PreferNode, "prefer-node", true, "constrain benchmark cpus to one NUMA node when possible")
	fs.IntVar(&cfg.SamplingMS, "sampling-ms", 1000, "interrupt sampling window for cpu ranking, ms")
	return cfg
}


func (cfg *Config) SetCmd(args []string) {
	cfg.Cmd = args
}


func Parse(args []string) (*Config, error) {
	fs := flag.NewFlagSet("goset", flag.ContinueOnError)
	cfg := Register(fs)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	cfg.SetCmd(fs.Args())
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
