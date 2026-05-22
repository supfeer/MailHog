package storage

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mailhog/data"
)

func TestMaildirListHonorsStartLimitAndOrdersByModifiedTime(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-list")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	writeMaildirTestMessage(t, dir, "old", "old", time.Unix(100, 0))
	writeMaildirTestMessage(t, dir, "newest", "newest", time.Unix(300, 0))
	writeMaildirTestMessage(t, dir, "middle", "middle", time.Unix(200, 0))

	maildir := CreateMaildir(dir)
	maildir.refreshCache()
	messages, err := maildir.List(1, 1)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(*messages), 1; got != want {
		t.Fatalf("got %d messages, want %d", got, want)
	}
	if got, want := string((*messages)[0].ID), "middle"; got != want {
		t.Fatalf("got message %q, want %q", got, want)
	}
}

func TestMaildirListReturnsEmptyWhenStartPastEnd(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-list-empty")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	writeMaildirTestMessage(t, dir, "only", "only", time.Unix(100, 0))

	maildir := CreateMaildir(dir)
	maildir.refreshCache()
	messages, err := maildir.List(1, 1)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(*messages), 0; got != want {
		t.Fatalf("got %d messages, want %d", got, want)
	}
}

func TestMaildirSearchUsesCacheAndOrdersByModifiedTime(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-search")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	writeMaildirTestMessageTo(t, dir, "old", "target@example.com", "old", time.Unix(100, 0))
	writeMaildirTestMessageTo(t, dir, "newest", "other@example.com", "newest", time.Unix(300, 0))
	writeMaildirTestMessageTo(t, dir, "middle", "target@example.com", "middle", time.Unix(200, 0))

	maildir := CreateMaildir(dir)
	maildir.refreshCache()
	messages, total, err := maildir.Search("to", "target@example.com", 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := total, 2; got != want {
		t.Fatalf("got total %d, want %d", got, want)
	}
	if got, want := len(*messages), 1; got != want {
		t.Fatalf("got %d messages, want %d", got, want)
	}
	if got, want := string((*messages)[0].ID), "middle"; got != want {
		t.Fatalf("got message %q, want %q", got, want)
	}
}

func TestMaildirStoreUpdatesCacheImmediately(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-store-cache")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	maildir := CreateMaildir(dir)
	msg := (&data.SMTPMessage{
		From: "sender@example.com",
		To:   []string{"instant@example.com"},
		Data: "Subject: instant\r\n\r\nbody",
		Helo: "localhost",
	}).Parse("mailhog.example")

	if _, err := maildir.Store(msg); err != nil {
		t.Fatal(err)
	}

	messages, total, err := maildir.Search("to", "instant@example.com", 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := total, 1; got != want {
		t.Fatalf("got total %d, want %d", got, want)
	}
	if got, want := len(*messages), 1; got != want {
		t.Fatalf("got %d messages, want %d", got, want)
	}
	if (*messages)[0].Raw != nil {
		t.Fatal("cached search result should not include raw message data")
	}
	if (*messages)[0].Content == nil || (*messages)[0].Content.Body != "" {
		t.Fatal("cached search result should include headers without body")
	}
}

func TestMaildirDeleteOlderThanRemovesFilesAndCacheEntries(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-delete-older")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	writeMaildirTestMessage(t, dir, "old", "old", time.Unix(100, 0))
	writeMaildirTestMessage(t, dir, "new", "new", time.Unix(300, 0))

	maildir := CreateMaildir(dir)
	maildir.refreshCache()

	if err := maildir.DeleteOlderThan(time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}

	if got, want := maildir.Count(), 1; got != want {
		t.Fatalf("got count %d, want %d", got, want)
	}

	messages, err := maildir.List(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(*messages), 1; got != want {
		t.Fatalf("got %d messages, want %d", got, want)
	}
	if got, want := string((*messages)[0].ID), "new"; got != want {
		t.Fatalf("got message %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(err) {
		t.Fatalf("old message file still exists or stat failed with unexpected error: %v", err)
	}
}

func TestMaildirMaintenanceDeletesOlderThan(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-maintenance-age")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	now := time.Now()
	writeMaildirTestMessage(t, dir, "old", "old", now.Add(-2*time.Hour))
	writeMaildirTestMessage(t, dir, "new", "new", now)

	maildir := CreateMaildir(dir)
	maildir.maintenance = MaintenancePolicy{
		Enabled:         true,
		Interval:        time.Hour,
		DeleteOlderThan: time.Hour,
	}

	result, err := maildir.RunMaintenance("test")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Deleted, 1; got != want {
		t.Fatalf("got deleted %d, want %d", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(err) {
		t.Fatalf("old message file still exists or stat failed with unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); err != nil {
		t.Fatalf("new message file missing: %v", err)
	}
}

func TestMaildirMaintenanceEvictsOldestByMaxBytes(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-maintenance-bytes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	writeMaildirTestMessageBody(t, dir, "old", "old", strings.Repeat("o", 2048), time.Unix(100, 0))
	writeMaildirTestMessageBody(t, dir, "new", "new", "small", time.Unix(300, 0))
	newSize := maildirTestFileSize(t, filepath.Join(dir, "new"))

	maildir := CreateMaildir(dir)
	maildir.maintenance = MaintenancePolicy{
		Enabled:  true,
		Interval: time.Hour,
		MaxBytes: newSize,
	}

	result, err := maildir.RunMaintenance("test")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Deleted, 1; got != want {
		t.Fatalf("got deleted %d, want %d", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(err) {
		t.Fatalf("old message file still exists or stat failed with unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); err != nil {
		t.Fatalf("new message file missing: %v", err)
	}
}

func TestMaildirMaintenanceEvictsOldestByMaxMessages(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-maintenance-count")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	writeMaildirTestMessage(t, dir, "old", "old", time.Unix(100, 0))
	writeMaildirTestMessage(t, dir, "middle", "middle", time.Unix(200, 0))
	writeMaildirTestMessage(t, dir, "new", "new", time.Unix(300, 0))

	maildir := CreateMaildir(dir)
	maildir.maintenance = MaintenancePolicy{
		Enabled:     true,
		Interval:    time.Hour,
		MaxMessages: 2,
	}

	result, err := maildir.RunMaintenance("test")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Deleted, 1; got != want {
		t.Fatalf("got deleted %d, want %d", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(err) {
		t.Fatalf("old message file still exists or stat failed with unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "middle")); err != nil {
		t.Fatalf("middle message file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); err != nil {
		t.Fatalf("new message file missing: %v", err)
	}
}

func TestMaildirMaintenanceEvictsOldestByMinFreeBytes(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-maintenance-free")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	writeMaildirTestMessageBody(t, dir, "old", "old", strings.Repeat("o", 512), time.Unix(100, 0))
	writeMaildirTestMessageBody(t, dir, "new", "new", strings.Repeat("n", 128), time.Unix(300, 0))

	maildir := CreateMaildir(dir)
	maildir.maintenance = MaintenancePolicy{
		Enabled:      true,
		Interval:     time.Hour,
		MinFreeBytes: 600,
	}
	maildir.diskStats = maildirTestFreeBytesProvider(1200)

	result, err := maildir.RunMaintenance("test")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Deleted, 1; got != want {
		t.Fatalf("got deleted %d, want %d", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(err) {
		t.Fatalf("old message file still exists or stat failed with unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); err != nil {
		t.Fatalf("new message file missing: %v", err)
	}
}

func TestMaildirMaintenanceSkipsTempFiles(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-maintenance-temp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	now := time.Now()
	writeMaildirTestMessage(t, dir, "old", "old", now.Add(-2*time.Hour))
	tempPath := filepath.Join(dir, maildirTempPrefix+"active")
	if err := ioutil.WriteFile(tempPath, []byte("temp"), 0660); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(tempPath, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}

	maildir := CreateMaildir(dir)
	maildir.maintenance = MaintenancePolicy{
		Enabled:         true,
		Interval:        time.Hour,
		DeleteOlderThan: time.Hour,
	}

	if _, err := maildir.RunMaintenance("test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tempPath); err != nil {
		t.Fatalf("temp file should be preserved: %v", err)
	}
}

func TestMaildirCommitGuardEvictsOldMessagesAndKeepsCurrentWrite(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-guard-evict")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	writeMaildirTestMessageBody(t, dir, "old", "old", strings.Repeat("o", 2048), time.Unix(100, 0))
	maildir := CreateMaildir(dir)
	writer, err := maildir.CreateMessageWriter(&data.SMTPMessage{
		From: "sender@example.com",
		To:   []string{"recipient@example.com"},
		Helo: "localhost",
	}, "mailhog.example")
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteLine("Subject: current"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteLine(""); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteLine("body"); err != nil {
		t.Fatal(err)
	}

	maildir.maintenance = MaintenancePolicy{
		Enabled:  true,
		Interval: time.Hour,
		MaxBytes: 1024,
	}

	id, _, err := writer.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(err) {
		t.Fatalf("old message file still exists or stat failed with unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, id)); err != nil {
		t.Fatalf("current message file missing: %v", err)
	}
}

func TestMaildirCommitGuardAbortsWhenLimitsCannotBeSatisfied(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-guard-full")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	maildir := CreateMaildir(dir)
	maildir.maintenance = MaintenancePolicy{
		Enabled:  true,
		Interval: time.Hour,
		MaxBytes: 1,
	}

	writer, err := maildir.CreateMessageWriter(&data.SMTPMessage{
		From: "sender@example.com",
		To:   []string{"recipient@example.com"},
		Helo: "localhost",
	}, "mailhog.example")
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteLine("Subject: too large"); err != nil {
		t.Fatal(err)
	}

	if _, _, err := writer.Commit(); err != ErrStorageLimitExceeded {
		t.Fatalf("got error %v, want %v", err, ErrStorageLimitExceeded)
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), maildirTempPrefix) {
			t.Fatalf("temp file %s should have been removed", file.Name())
		}
	}
}

func TestMaildirWriteMessageToStreamsMessageWithoutEnvelope(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-download")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	raw := &data.SMTPMessage{
		From: "sender@example.com",
		To:   []string{"recipient@example.com"},
		Data: "Subject: download\r\nX-Test: yes\r\n\r\nbody",
		Helo: "localhost",
	}
	dataBytes, err := ioutil.ReadAll(raw.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "download-id"), dataBytes, 0660); err != nil {
		t.Fatal(err)
	}

	maildir := CreateMaildir(dir)
	var out bytes.Buffer
	if err := maildir.WriteMessageTo("download-id", &out); err != nil {
		t.Fatal(err)
	}

	want := "Subject: download\r\nX-Test: yes\r\n\r\nbody"
	if got := out.String(); got != want {
		t.Fatalf("got streamed message %q, want %q", got, want)
	}
}

func TestMaildirLoadBodyChunkSkipsMultipartAttachments(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-body-mime")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	raw := &data.SMTPMessage{
		From: "sender@example.com",
		To:   []string{"recipient@example.com"},
		Data: strings.Join([]string{
			"Subject: preview",
			"Content-Type: multipart/mixed; boundary=\"test-boundary\"",
			"",
			"--test-boundary",
			"Content-Type: text/plain; charset=\"utf-8\"",
			"",
			"visible body",
			"--test-boundary",
			"Content-Type: application/octet-stream",
			"Content-Disposition: attachment; filename=\"large.bin\"",
			"",
			strings.Repeat("x", 1024),
			"--test-boundary--",
		}, "\r\n"),
		Helo: "localhost",
	}
	writeMaildirRawMessage(t, dir, "mime-id", raw)

	maildir := CreateMaildir(dir)
	chunk, err := maildir.LoadBodyChunk("mime-id", 0, 1024, 10*1024)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := chunk.Content.Body, "visible body"; got != want {
		t.Fatalf("got body chunk %q, want %q", got, want)
	}
	if strings.Contains(chunk.Content.Body, "xxx") {
		t.Fatal("body chunk should not include attachment content")
	}
}

func TestMaildirLoadBodyChunkHonorsOffsetLimitAndMaxSize(t *testing.T) {
	dir, err := ioutil.TempDir("", "mailhog-maildir-body-chunk")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	raw := &data.SMTPMessage{
		From: "sender@example.com",
		To:   []string{"recipient@example.com"},
		Data: "Subject: chunks\r\nContent-Type: text/plain\r\n\r\n" + strings.Repeat("a", 20),
		Helo: "localhost",
	}
	writeMaildirRawMessage(t, dir, "chunk-id", raw)

	maildir := CreateMaildir(dir)
	first, err := maildir.LoadBodyChunk("chunk-id", 0, 8, 12)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := first.Content.Body, "aaaaaaaa"; got != want {
		t.Fatalf("got first chunk %q, want %q", got, want)
	}
	if !first.HasMore || first.Truncated || first.NextOffset != 8 {
		t.Fatalf("unexpected first chunk state: %+v", first)
	}

	second, err := maildir.LoadBodyChunk("chunk-id", first.NextOffset, 8, 12)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := second.Content.Body, "aaaa"; got != want {
		t.Fatalf("got second chunk %q, want %q", got, want)
	}
	if second.HasMore || !second.Truncated || second.NextOffset != 12 {
		t.Fatalf("unexpected second chunk state: %+v", second)
	}
}

func writeMaildirTestMessage(t *testing.T, dir, id, subject string, modTime time.Time) {
	writeMaildirTestMessageTo(t, dir, id, "recipient@example.com", subject, modTime)
}

func writeMaildirTestMessageTo(t *testing.T, dir, id, to, subject string, modTime time.Time) {
	writeMaildirTestMessageBodyTo(t, dir, id, to, subject, "body", modTime)
}

func writeMaildirTestMessageBody(t *testing.T, dir, id, subject, body string, modTime time.Time) {
	writeMaildirTestMessageBodyTo(t, dir, id, "recipient@example.com", subject, body, modTime)
}

func writeMaildirTestMessageBodyTo(t *testing.T, dir, id, to, subject, body string, modTime time.Time) {
	t.Helper()

	raw := &data.SMTPMessage{
		From: "sender@example.com",
		To:   []string{to},
		Data: "Subject: " + subject + "\r\n\r\n" + body,
		Helo: "localhost",
	}
	bytes, err := ioutil.ReadAll(raw.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, id)
	if err := ioutil.WriteFile(path, bytes, 0660); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func maildirTestFileSize(t *testing.T, path string) int64 {
	t.Helper()

	fileinfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fileinfo.Size()
}

func maildirTestFreeBytesProvider(capacity int64) maildirDiskStatsFunc {
	return func(path string) (maildirDiskStatsSnapshot, error) {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return maildirDiskStatsSnapshot{}, err
		}
		var used int64
		for _, file := range files {
			if file.IsDir() || strings.HasPrefix(file.Name(), maildirTempPrefix) {
				continue
			}
			used += file.Size()
		}
		return maildirDiskStatsSnapshot{
			FreeBytes:      capacity - used,
			FreeBytesKnown: true,
		}, nil
	}
}

func writeMaildirRawMessage(t *testing.T, dir, id string, raw *data.SMTPMessage) {
	t.Helper()

	dataBytes, err := ioutil.ReadAll(raw.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, id), dataBytes, 0660); err != nil {
		t.Fatal(err)
	}
}
