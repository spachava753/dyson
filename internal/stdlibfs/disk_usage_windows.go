//go:build windows

package stdlibfs

import "golang.org/x/sys/windows"

func diskUsage(path string) (Usage, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return Usage{}, err
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &free, &total, &totalFree); err != nil {
		return Usage{}, err
	}
	return Usage{Total: total, Used: total - free, Free: free}, nil
}
