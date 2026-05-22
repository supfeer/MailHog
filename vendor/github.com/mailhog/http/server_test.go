package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicAuthHandlerAllowsHealthCheckWithoutCredentials(t *testing.T) {
	previous := Authorised
	Authorised = func(string, string) bool { return false }
	defer func() { Authorised = previous }()

	handler := BasicAuthHandler(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, req *nethttp.Request) {
		w.WriteHeader(200)
	}))

	for _, path := range []string{"/healthz", "/mailhog/healthz"} {
		req := httptest.NewRequest("GET", path, nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if got, want := recorder.Code, 200; got != want {
			t.Fatalf("got status %d for %s, want %d", got, path, want)
		}
	}
}

func TestBasicAuthHandlerRequiresCredentialsForNonHealthCheck(t *testing.T) {
	previous := Authorised
	Authorised = func(string, string) bool { return false }
	defer func() { Authorised = previous }()

	handler := BasicAuthHandler(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, req *nethttp.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/api/v2/messages", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if got, want := recorder.Code, 401; got != want {
		t.Fatalf("got status %d, want %d", got, want)
	}
}
