//go:build !windows

package xos

import "syscall"

func hostUmask(mask int) int {
	return syscall.Umask(mask)
}
