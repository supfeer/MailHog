package storage

import (
	"errors"
	"io"
	"time"

	"github.com/mailhog/data"
)

// ErrStorageLimitExceeded is returned when storage maintenance cannot free
// enough space for a new message.
var ErrStorageLimitExceeded = errors.New("storage limits exceeded")

// Storage represents a storage backend
type Storage interface {
	Store(m *data.Message) (string, error)
	List(start, limit int) (*data.Messages, error)
	Search(kind, query string, start, limit int) (*data.Messages, int, error)
	Count() int
	DeleteOne(id string) error
	DeleteAll() error
	DeleteOlderThan(cutoff time.Time) error
	Load(id string) (*data.Message, error)
}

// MessageWriter stores a message body incrementally. It is used by storage
// backends which can avoid keeping full SMTP DATA in memory.
type MessageWriter interface {
	WriteLine(line string) error
	Commit() (string, *data.Message, error)
	Abort() error
}

// StreamingStorage can persist SMTP DATA as it arrives.
type StreamingStorage interface {
	CreateMessageWriter(m *data.SMTPMessage, hostname string) (MessageWriter, error)
}

// PreviewStorage can return a memory-safe message representation for UI
// preview endpoints.
type PreviewStorage interface {
	LoadPreview(id string, maxSize int64) (*data.Message, error)
}

// DownloadStorage can stream an RFC822 message without loading it into memory.
type DownloadStorage interface {
	WriteMessageTo(id string, w io.Writer) error
}

// MessageBodyChunk is a bounded preview chunk of the displayable message body.
type MessageBodyChunk struct {
	ID         string
	Content    *data.Content
	Offset     int64
	NextOffset int64
	Limit      int64
	MaxSize    int64
	HasMore    bool
	Truncated  bool
	Source     string
}

// BodyPreviewStorage can return bounded display body chunks without loading
// the whole message or its attachments.
type BodyPreviewStorage interface {
	LoadBodyChunk(id string, offset int64, limit int64, maxSize int64) (*MessageBodyChunk, error)
}

// MaintenancePolicy configures automatic storage cleanup. Limits are disabled
// when their values are zero.
type MaintenancePolicy struct {
	Enabled         bool
	Interval        time.Duration
	DeleteOlderThan time.Duration
	MaxBytes        int64
	MaxMessages     int
	MinFreeBytes    int64
}

// HasLimits reports whether any cleanup guard is configured.
func (policy MaintenancePolicy) HasLimits() bool {
	return policy.DeleteOlderThan > 0 || policy.MaxBytes > 0 || policy.MaxMessages > 0 || policy.MinFreeBytes > 0
}

// Active reports whether maintenance should run.
func (policy MaintenancePolicy) Active() bool {
	return policy.Enabled && policy.HasLimits()
}

// MaintenanceStats describes storage state before or after cleanup.
type MaintenanceStats struct {
	MessageCount   int
	TotalBytes     int64
	FreeBytes      int64
	FreeBytesKnown bool
}

// MaintenanceResult describes one maintenance pass.
type MaintenanceResult struct {
	Reason     string
	Deleted    int
	FreedBytes int64
	Before     MaintenanceStats
	After      MaintenanceStats
}
