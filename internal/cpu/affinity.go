package cpu

import (
	"fmt"
	"syscall"
	"unsafe"
)

const cpuSetBytes = 128 //1024 bits(threads)

func GetAffinity(pid int) ([]int, error) {
	var mask [cpuSetBytes]byte
	_, _, errno := syscall.RawSyscall(
		syscall.SYS_SCHED_GETAFFINITY,
		uintptr(pid), 
		uintptr(cpuSetBytes),
		uintptr(unsafe.Pointer(&mask[0])))

	if errno != 0 {
		return nil, errno
	}

	var cpus []int
	for i := 0; i<cpuSetBytes*8; i++ {
		if mask[i/8]&(1<<uint(i%8)) != 0 { // check avail
			cpus = append(cpus, i)
		}
	}

	return cpus, nil
}


func SetAffinity(pid int, cpus []int) error {
	var mask [cpuSetBytes]byte

	for _, c := range cpus { // idx, elem
		if c < 0 || c > cpuSetBytes*8 {
			fmt.Errorf("cpu %d out of mask range, currently support %d", c, cpuSetBytes*8)
		}
		mask[c/8] |= 1 << uint(c%8) //select processor
	}

	return nil
}
