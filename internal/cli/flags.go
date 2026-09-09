package cli

import (
	"flag"
	"fmt"
)

type Config struct {
	/* Target settings */
	Cmd []string // argv after the "--" separator

	/* Thread selection */
	ManualPin   int  // user pin to thread N
	SetAffinity bool // (sudo) smp_affinity_list
	Cgroup      int  // (sudo) multithread / container

	/* rights */
	Sudo bool
}


func (cfg *Config) Validate() error {
	if len(cfg.Cmd) == 0 {
		return fmt.Errorf("-pin requires a target after --")
	}
	if cfg.SetAffinity && !cfg.Sudo {
		return fmt.Errorf("-set-affinity needs --sudo ")
	}
	if cfg.Cgroup > 0 && !cfg.Sudo {
		return fmt.Errorf("-cgroup requires --sudo")
	}

	return nil
}


// Register binds flags with struct items
func Register(fs *flag.FlagSet) *Config {
	cfg := &Config{}
	fs.IntVar(&cfg.ManualPin, "pin", -1, "bypass autoselection of threads")
	fs.BoolVar(&cfg.SetAffinity, "set-affinity", false, "set smp_affinity_list (sudo)")
	fs.IntVar(&cfg.Cgroup, "cgroup", 0, "number of threads for cgroup containerization (sudo)")
	fs.BoolVar(&cfg.Sudo, "sudo", false, "run the privileged protocol")
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
