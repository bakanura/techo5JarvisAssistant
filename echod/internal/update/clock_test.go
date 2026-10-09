package update

import (
	"context"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServerTime(t *testing.T) {
	when := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	date := when.Format(http.TimeFormat)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Date", date)
		http.Redirect(w, r, "https://elsewhere.example/asset", http.StatusFound)
	}))
	defer srv.Close()
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Date", date)
	}))
	defer plain.Close()

	restore := clockRoots
	t.Cleanup(func() { clockRoots = restore })
	clockRoots = x509.NewCertPool()
	clockRoots.AddCert(srv.Certificate())

	got, err := serverTime(context.Background(), srv.URL+"/manifest.json")
	if err != nil || !got.Equal(when) {
		t.Fatalf("serverTime = %v, %v; want %v", got, err, when)
	}

	date = "Thu, 01 Jan 2015 00:00:00 GMT"
	if got, err := serverTime(context.Background(), srv.URL); err == nil {
		t.Errorf("a time before 2025 was taken: %v", got)
	}
	date = when.Format(http.TimeFormat)

	if got, err := serverTime(context.Background(), plain.URL); err == nil {
		t.Errorf("a plain HTTP server's time was taken: %v", got)
	}

	clockRoots = x509.NewCertPool() // a certificate from nobody we trust
	if got, err := serverTime(context.Background(), srv.URL); err == nil {
		t.Errorf("an untrusted server's time was taken: %v", got)
	}
}
