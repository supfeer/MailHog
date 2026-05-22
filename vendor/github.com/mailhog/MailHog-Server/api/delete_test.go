package api

import (
	"net/http"
	"testing"
	"time"
)

func TestDeleteOlderThanCutoffParsesUnixMilliseconds(t *testing.T) {
	req, err := http.NewRequest("DELETE", "/api/v3/messages?older_than=1700000000000", nil)
	if err != nil {
		t.Fatal(err)
	}

	cutoff, ok, err := deleteOlderThanCutoff(req)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected cutoff to be present")
	}
	if got, want := cutoff, time.Unix(0, 1700000000000*int64(time.Millisecond)); !got.Equal(want) {
		t.Fatalf("got cutoff %s, want %s", got, want)
	}
}

func TestDeleteOlderThanCutoffParsesRFC3339(t *testing.T) {
	req, err := http.NewRequest("DELETE", "/api/v3/messages?olderThan=2026-05-22T12:34:56Z", nil)
	if err != nil {
		t.Fatal(err)
	}

	cutoff, ok, err := deleteOlderThanCutoff(req)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected cutoff to be present")
	}
	if got, want := cutoff.Format(time.RFC3339), "2026-05-22T12:34:56Z"; got != want {
		t.Fatalf("got cutoff %s, want %s", got, want)
	}
}

func TestDeleteOlderThanCutoffRejectsInvalidValue(t *testing.T) {
	req, err := http.NewRequest("DELETE", "/api/v3/messages?older_than=not-a-date", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = deleteOlderThanCutoff(req)
	if err == nil {
		t.Fatal("expected invalid cutoff error")
	}
}
