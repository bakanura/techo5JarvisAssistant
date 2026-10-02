package diag

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCertificatesCannotBeDisabled(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	if _, err := http.Get(srv.URL); err == nil {
		t.Fatal("a certificate nothing trusted signed was accepted")
	}
}
