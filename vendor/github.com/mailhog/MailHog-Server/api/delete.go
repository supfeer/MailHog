package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mailhog/storage"
)

func deleteMessages(storageBackend storage.Storage, req *http.Request) (int, error) {
	cutoff, ok, err := deleteOlderThanCutoff(req)
	if err != nil {
		return http.StatusBadRequest, err
	}
	if ok {
		return http.StatusOK, storageBackend.DeleteOlderThan(cutoff)
	}
	return http.StatusOK, storageBackend.DeleteAll()
}

func deleteOlderThanCutoff(req *http.Request) (time.Time, bool, error) {
	query := req.URL.Query()
	for _, key := range []string{"older_than", "olderThan", "older_than_ms", "olderThanMs"} {
		value := strings.TrimSpace(query.Get(key))
		if value == "" {
			continue
		}
		cutoff, err := parseDeleteOlderThan(value, strings.HasSuffix(strings.ToLower(key), "ms"))
		return cutoff, true, err
	}
	return time.Time{}, false, nil
}

func parseDeleteOlderThan(value string, forceMilliseconds bool) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("older_than value is empty")
	}

	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		if n <= 0 {
			return time.Time{}, fmt.Errorf("older_than must be positive")
		}
		if forceMilliseconds || len(value) > 10 {
			return time.Unix(0, n*int64(time.Millisecond)), nil
		}
		return time.Unix(n, 0), nil
	}

	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if cutoff, err := time.Parse(layout, value); err == nil {
			return cutoff, nil
		}
	}

	return time.Time{}, fmt.Errorf("older_than must be Unix seconds, Unix milliseconds, or RFC3339 timestamp")
}
