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
	Warning    string
}

// https://www.kernel.org/doc/html/latest/core-api/irq/irq-affinity.html
func Steer(topo *cpu.Topology, bench generic.CPUSet, housekeeper int) (*Steering, error) {
	steering := &Steering{}
	if err := steering.holdIRQBalance(); err != nil {
		return nil, err
	}
	if err := steering.steerIRQs(topo, bench, housekeeper); err != nil {
		steering.Release()
		return nil, err
	}
	return steering, nil
}
