package cpu

import (
	"syscall"
	"unsafe"
)

func GetAffinity(pid int) (CPUSet, error) {
	cpus := CPUSet{}
	_, _, errno := syscall.RawSyscall(
		syscall.SYS_SCHED_GETAFFINITY,
		uintptr(pid),
		unsafe.Sizeof(cpus), // kernel wants bytes
		uintptr(unsafe.Pointer(&cpus[0])))

	if errno != 0 {
		return CPUSet{}, errno
	}

	return cpus, nil
}


func SetAffinity(pid int, cpus CPUSet) error {
	_, _, errno := syscall.RawSyscall(
		syscall.SYS_SCHED_SETAFFINITY,
		uintptr(pid),
		unsafe.Sizeof(cpus), // kernel wants bytes
		uintptr(unsafe.Pointer(&cpus[0])))
	if errno != 0 {
		return errno
	}
	return nil
}
