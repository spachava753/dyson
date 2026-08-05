//go:build windows

package xos

import "golang.org/x/sys/windows"

func hostUmask(mask int) int {
	previous, _, _ := windows.NewLazySystemDLL("msvcrt.dll").NewProc("_umask").Call(uintptr(mask))
	return int(previous)
}
