package generic

type CPUScore struct {
	CPU          int
	Included     uint8  // 0|1
	Steerable    uint64 // numbered rows
	NonSteerable uint64 // named rows: LOC, RES, CAL, TLB
	Noise        uint64 // non-steerable IRQs, plus steerable ones without -steer
	SiblingLoad  uint64 // Noise of the SMT siblings, self excluded
	Numa         int
	KernelIsol   bool
	NohzFull     bool
	RcuNocb      bool
}

type Selection struct {
	Scores      []CPUScore // Selection -> Report
	Numa        int
	Task        CPUSet
	Fence       CPUSet // SMT siblings of Task, booked idle in the cgroup
	Unfenced    CPUSet // SMT siblings of Task the fence left open
	HouseKeeper int
}

// Booked returns task and fence
func (selected *Selection) Booked() CPUSet {
	booked := selected.Task
	booked.Or(selected.Fence)
	return booked
}
