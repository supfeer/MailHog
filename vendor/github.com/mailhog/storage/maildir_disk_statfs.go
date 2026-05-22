//go:build !windows
// +build !windows

package storage

import "syscall"

func maildirDiskStats(path string) (maildirDiskStatsSnapshot, error) {
	var stats syscall.Statfs_t
	if err := syscall.Statfs(path, &stats); err != nil {
		return maildirDiskStatsSnapshot{}, err
	}
	return maildirDiskStatsSnapshot{
		FreeBytes:      int64(stats.Bavail) * int64(stats.Bsize),
		FreeBytesKnown: true,
	}, nil
}
