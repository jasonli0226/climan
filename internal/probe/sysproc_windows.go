//go:build windows

package probe

import "syscall"

// createNoWindow keeps console programs from opening a window.
const createNoWindow = 0x08000000

func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: createNoWindow}
}
