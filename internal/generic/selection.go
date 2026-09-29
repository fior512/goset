package generic

type CPUScore struct {
	CPU          int
	Included     uint8  // 0|1
	Steerable    uint64 // numbered rows
	NonSteerable uint64 // named rows: LOC, RES, CAL, TLB
	SiblingLoad  uint64 // IRQs of the SMT siblings, self excluded
	Numa         int
	KernelIsol   bool
	NohzFull     bool
	RcuNocb      bool
}


type Selection struct {
	Scores      []CPUScore // SelectCPUs -> Report
	Numa        int
	Task        CPUSet
	Fence       CPUSet // SMT siblings of Task, booked idle in the cgroup
	HouseKeeper int
}

func (selected *Selection) Booked() CPUSet {
	booked := selected.Task
	booked.Or(selected.Fence)
	return booked
}
