package isolation

import (
	"goset/internal/cpu"
	"goset/internal/generic"
)

type Steering struct {
	/* Steering */
	Attempted   int
	Applied     int
	Rejected    int
	Errors      map[string]int    // errno string -> count
	Remaining   []string          // IRQ labels still permitting a benchmark CPU
	Saved       map[string]string // IRQ label -> original smp_affinity_list
	AppliedList string            // affinity list written to the steered IRQs

	/* IRQBalance */
	WasRunning bool
	ViaSystemd bool
	Note       string
}

// https://www.kernel.org/doc/html/latest/core-api/irq/irq-affinity.html
func Steer(topo *cpu.Topology, bench generic.CPUSet, housekeeper int) (*Steering, error) {
	steer := &Steering{}
	if err := steer.holdIRQBalance(); err != nil {
		return nil, err
	}
	if err := steer.steerIRQs(topo, bench, housekeeper); err != nil {
		steer.Release()
		return nil, err
	}
	return steer, nil
}
