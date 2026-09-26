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
	HouseKeeper int
}
