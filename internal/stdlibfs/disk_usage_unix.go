//go:build !windows

package stdlibfs

import "syscall"

func diskUsage(path string) (Usage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return Usage{}, err
	}
	blockSize := uint64(stat.Bsize)
	total := stat.Blocks * blockSize
	free := stat.Bavail * blockSize
	return Usage{Total: total, Used: total - free, Free: free}, nil
}
