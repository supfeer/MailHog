package api

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/pat"
	"github.com/mailhog/MailHog-Server/config"
	"github.com/mailhog/data"
)

type loadCountingStorage struct {
	loads int32
}

func (storage *loadCountingStorage) Store(m *data.Message) (string, error) {
	return string(m.ID), nil
}

func (storage *loadCountingStorage) List(start, limit int) (*data.Messages, error) {
	messages := data.Messages{}
	return &messages, nil
}

func (storage *loadCountingStorage) Search(kind, query string, start, limit int) (*data.Messages, int, error) {
	messages := data.Messages{}
	return &messages, 0, nil
}

func (storage *loadCountingStorage) Count() int {
	return 0
}

func (storage *loadCountingStorage) DeleteOne(id string) error {
	return nil
}

func (storage *loadCountingStorage) DeleteAll() error {
	return nil
}

func (storage *loadCountingStorage) DeleteOlderThan(cutoff time.Time) error {
	return nil
}

func (storage *loadCountingStorage) Load(id string) (*data.Message, error) {
	atomic.AddInt32(&storage.loads, 1)
	return &data.Message{
		ID: data.MessageID(id),
		Content: &data.Content{
			Headers: map[string][]string{"Subject": {"loaded"}},
			Body:    "body",
			Size:    4,
		},
	}, nil
}

func TestCreateAPISkipsFullRealtimeLoadWithoutSubscribers(t *testing.T) {
	storage := &loadCountingStorage{}
	conf := config.DefaultConfig()
	conf.Storage = storage
	conf.MessageChan = make(chan *data.Message, 1)

	CreateAPI(conf, pat.New())
	conf.MessageChan <- &data.Message{
		ID: data.MessageID("compact-id"),
		Content: &data.Content{
			Headers: map[string][]string{"Subject": {"compact"}},
			Size:    1024,
		},
	}

	deadline := time.After(500 * time.Millisecond)
	for len(conf.MessageChan) > 0 {
		select {
		case <-deadline:
			t.Fatal("API fanout did not drain message queue")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	time.Sleep(50 * time.Millisecond)

	if got := atomic.LoadInt32(&storage.loads); got != 0 {
		t.Fatalf("got %d full message loads without realtime subscribers, want 0", got)
	}
}
