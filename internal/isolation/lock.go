package isolation

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type Lock struct {
	file *os.File
}

func AcquireLock(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		holder := lockHolder(path)
		file.Close()
		if holder != "" {
			return nil, fmt.Errorf("%s held by pid %s: %w", path, holder, err)
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := file.Truncate(0); err == nil {
		_, _ = file.WriteAt([]byte(strconv.Itoa(os.Getpid())), 0)
	}
	return &Lock{file: file}, nil
}

func (lock *Lock) Release() {
	if lock == nil || lock.file == nil {
		return
	}
	_ = syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	lock.file.Close()
	lock.file = nil
}

func lockHolder(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
