package storage

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mailhog/data"
)

type maildirDiskStatsSnapshot struct {
	FreeBytes      int64
	FreeBytesKnown bool
}

type maildirDiskStatsFunc func(path string) (maildirDiskStatsSnapshot, error)

type maildirMaintenanceFile struct {
	name    string
	path    string
	size    int64
	modTime time.Time
}

type maildirMaintenanceStats struct {
	files          []maildirMaintenanceFile
	messageCount   int
	totalBytes     int64
	freeBytes      int64
	freeBytesKnown bool
}

func (maildir *Maildir) maintenanceLoop() {
	if _, err := maildir.RunMaintenance("startup"); err != nil {
		log.Printf("Maildir maintenance startup failed: %s", err)
	}

	ticker := time.NewTicker(maildir.maintenance.Interval)
	defer ticker.Stop()
	for range ticker.C {
		result, err := maildir.RunMaintenance("interval")
		if err != nil {
			log.Printf("Maildir maintenance failed: %s", err)
			continue
		}
		if result.Deleted > 0 {
			log.Printf("Maildir maintenance deleted %d messages and freed %d bytes", result.Deleted, result.FreedBytes)
		}
	}
}

// RunMaintenance performs one cleanup pass for configured Maildir limits.
func (maildir *Maildir) RunMaintenance(reason string) (MaintenanceResult, error) {
	return maildir.enforceMaintenance(reason, 0, 0)
}

func (maildir *Maildir) guardStorage(pendingBytes int64, pendingMessages int, reason string) error {
	if !maildir.maintenance.Active() {
		return nil
	}
	if !maildir.guardNeedsMaintenance(pendingBytes, pendingMessages) {
		return nil
	}
	_, err := maildir.enforceMaintenance(reason, pendingBytes, pendingMessages)
	return err
}

func (maildir *Maildir) guardNeedsMaintenance(pendingBytes int64, pendingMessages int) bool {
	if pendingBytes < 0 {
		pendingBytes = 0
	}
	if pendingMessages < 0 {
		pendingMessages = 0
	}

	policy := maildir.maintenance
	if policy.MaxBytes <= 0 && policy.MaxMessages <= 0 && policy.MinFreeBytes <= 0 {
		return false
	}

	cacheReady, messageCount, totalBytes := maildir.cachedMaintenanceStats()
	if !cacheReady {
		return true
	}

	if policy.MaxBytes > 0 && totalBytes+pendingBytes > policy.MaxBytes {
		return true
	}
	if policy.MaxMessages > 0 && messageCount+pendingMessages > policy.MaxMessages {
		return true
	}
	if policy.MinFreeBytes > 0 {
		diskStats := maildir.diskStats
		if diskStats == nil {
			diskStats = maildirDiskStats
		}
		disk, err := diskStats(maildir.Path)
		if err != nil || !disk.FreeBytesKnown {
			return true
		}
		if disk.FreeBytes < policy.MinFreeBytes {
			return true
		}
	}

	return false
}

func (maildir *Maildir) cachedMaintenanceStats() (bool, int, int64) {
	maildir.mu.RLock()
	defer maildir.mu.RUnlock()

	var totalBytes int64
	for _, entry := range maildir.entries {
		totalBytes += entry.size
	}
	return maildir.cacheReady, len(maildir.entries), totalBytes
}

func (maildir *Maildir) enforceMaintenance(reason string, pendingBytes int64, pendingMessages int) (MaintenanceResult, error) {
	result := MaintenanceResult{Reason: reason}
	if !maildir.maintenance.Active() {
		return result, nil
	}
	if pendingBytes < 0 {
		pendingBytes = 0
	}
	if pendingMessages < 0 {
		pendingMessages = 0
	}

	maildir.maintenanceMu.Lock()
	defer maildir.maintenanceMu.Unlock()

	stats, err := maildir.scanMaintenanceStats()
	if err != nil {
		return result, err
	}
	result.Before = stats.toMaintenanceStats()

	var firstErr error
	if maildir.maintenance.DeleteOlderThan > 0 {
		cutoff := time.Now().Add(-maildir.maintenance.DeleteOlderThan)
		sortMaintenanceFilesOldestFirst(stats.files)
		stats, err = maildir.deleteMaintenanceFiles(stats, func(file maildirMaintenanceFile) bool {
			return file.modTime.Before(cutoff)
		}, &result)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	sortMaintenanceFilesOldestFirst(stats.files)
	for maildir.overMaintenanceLimits(stats, pendingBytes, pendingMessages) && len(stats.files) > 0 {
		file := stats.files[0]
		if err := maildir.removeMaintenanceFile(file); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			stats.files = stats.files[1:]
			continue
		}
		result.Deleted++
		result.FreedBytes += file.size
		stats.removeFileAt(0)
	}

	if result.Deleted > 0 {
		maildir.refreshCache()
		stats, err = maildir.scanMaintenanceStats()
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	result.After = stats.toMaintenanceStats()

	if firstErr != nil {
		return result, firstErr
	}
	if maildir.overMaintenanceLimits(stats, pendingBytes, pendingMessages) {
		return result, ErrStorageLimitExceeded
	}
	return result, nil
}

func (maildir *Maildir) deleteMaintenanceFiles(stats maildirMaintenanceStats, shouldDelete func(maildirMaintenanceFile) bool, result *MaintenanceResult) (maildirMaintenanceStats, error) {
	var firstErr error
	for i := 0; i < len(stats.files); {
		file := stats.files[i]
		if !shouldDelete(file) {
			i++
			continue
		}
		if err := maildir.removeMaintenanceFile(file); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			i++
			continue
		}
		result.Deleted++
		result.FreedBytes += file.size
		stats.removeFileAt(i)
	}
	return stats, firstErr
}

func (maildir *Maildir) scanMaintenanceStats() (maildirMaintenanceStats, error) {
	files, err := ioutil.ReadDir(maildir.Path)
	if err != nil {
		return maildirMaintenanceStats{}, err
	}

	stats := maildirMaintenanceStats{
		files:     make([]maildirMaintenanceFile, 0, len(files)),
		freeBytes: -1,
	}
	for _, fileinfo := range files {
		if fileinfo.IsDir() || strings.HasPrefix(fileinfo.Name(), maildirTempPrefix) {
			continue
		}
		item := maildirMaintenanceFile{
			name:    fileinfo.Name(),
			path:    filepath.Join(maildir.Path, fileinfo.Name()),
			size:    fileinfo.Size(),
			modTime: fileinfo.ModTime(),
		}
		stats.files = append(stats.files, item)
		stats.messageCount++
		stats.totalBytes += item.size
	}

	diskStats := maildir.diskStats
	if diskStats == nil {
		diskStats = maildirDiskStats
	}
	disk, err := diskStats(maildir.Path)
	if err != nil {
		if maildir.maintenance.MinFreeBytes > 0 {
			return stats, err
		}
		return stats, nil
	}
	stats.freeBytes = disk.FreeBytes
	stats.freeBytesKnown = disk.FreeBytesKnown
	return stats, nil
}

func (maildir *Maildir) overMaintenanceLimits(stats maildirMaintenanceStats, pendingBytes int64, pendingMessages int) bool {
	policy := maildir.maintenance
	if policy.MaxBytes > 0 && stats.totalBytes+pendingBytes > policy.MaxBytes {
		return true
	}
	if policy.MaxMessages > 0 && stats.messageCount+pendingMessages > policy.MaxMessages {
		return true
	}
	if policy.MinFreeBytes > 0 {
		if !stats.freeBytesKnown {
			return true
		}
		if stats.freeBytes < policy.MinFreeBytes {
			return true
		}
	}
	return false
}

func (maildir *Maildir) removeMaintenanceFile(file maildirMaintenanceFile) error {
	err := os.Remove(file.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (stats *maildirMaintenanceStats) removeFileAt(index int) {
	file := stats.files[index]
	stats.totalBytes -= file.size
	if stats.totalBytes < 0 {
		stats.totalBytes = 0
	}
	stats.messageCount--
	if stats.messageCount < 0 {
		stats.messageCount = 0
	}
	if stats.freeBytesKnown {
		stats.freeBytes += file.size
	}
	stats.files = append(stats.files[:index], stats.files[index+1:]...)
}

func (stats maildirMaintenanceStats) toMaintenanceStats() MaintenanceStats {
	return MaintenanceStats{
		MessageCount:   stats.messageCount,
		TotalBytes:     stats.totalBytes,
		FreeBytes:      stats.freeBytes,
		FreeBytesKnown: stats.freeBytesKnown,
	}
}

func sortMaintenanceFilesOldestFirst(files []maildirMaintenanceFile) {
	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime.Equal(files[j].modTime) {
			return files[i].name < files[j].name
		}
		return files[i].modTime.Before(files[j].modTime)
	})
}

func maildirEstimateMessageBytes(m *data.Message) int64 {
	if m == nil || m.Raw == nil {
		return 0
	}
	size := len("HELO:<" + m.Raw.Helo + ">\r\n")
	size += len("FROM:<" + m.Raw.From + ">\r\n")
	for _, to := range m.Raw.To {
		size += len("TO:<" + to + ">\r\n")
	}
	size += len("\r\n")
	size += len(m.Raw.Data)
	return int64(size)
}
