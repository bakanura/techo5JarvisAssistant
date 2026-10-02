package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrivatePageNeedsAuthorizedSetupSession(t *testing.T) {
	old := letIn
	t.Cleanup(func() { letIn = old })

	open := func() bool { return true }
	h := privateAllowed(open, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	letIn = func(*http.Request) bool { return false }
	r := httptest.NewRequest(http.MethodGet, "/camera.jpg", nil)
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("unauthorized private read = %d, want 403", w.Code)
	}

	letIn = func(*http.Request) bool { return true }
	w = httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("authorized private read = %d, want 204", w.Code)
	}
}

func TestClosedPrivatePageIsNotFoundBeforeAuthorization(t *testing.T) {
	old := letIn
	t.Cleanup(func() { letIn = old })
	letIn = func(*http.Request) bool { return true }

	h := privateAllowed(func() bool { return false }, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/screen.png", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("closed private page = %d, want 404", w.Code)
	}
}

func TestHardenedHeadersAreApplied(t *testing.T) {
	h := hardenedHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/setup", nil))

	for k, want := range map[string]string{
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := w.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}
