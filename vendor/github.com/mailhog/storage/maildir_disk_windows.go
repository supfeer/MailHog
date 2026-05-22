//go:build windows
// +build windows

package storage

func maildirDiskStats(path string) (maildirDiskStatsSnapshot, error) {
	return maildirDiskStatsSnapshot{FreeBytes: -1, FreeBytesKnown: false}, nil
}
