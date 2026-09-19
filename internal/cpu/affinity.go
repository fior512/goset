package cpu

import (
	"goset/internal/generic"
	"syscall"
	"unsafe"
)

// https://man7.org/linux/man-pages/man2/sched_setaffinity.2.html
func GetAffinity(pid int) (generic.CPUSet, error) {
	cpus := generic.CPUSet{}
	_, _, errno := syscall.RawSyscall(
		syscall.SYS_SCHED_GETAFFINITY,
		uintptr(pid),
		unsafe.Sizeof(cpus), // kernel wants bytes
		uintptr(unsafe.Pointer(&cpus[0])))

	if errno != 0 {
		return generic.CPUSet{}, errno
	}

	return cpus, nil
}

func SetAffinity(pid int, cpus generic.CPUSet) error {
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
