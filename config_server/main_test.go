package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// newMux mirrors main()'s routing so the tests exercise the same patterns.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(rootPattern, handleRoot)
	mux.HandleFunc("/config/", handleConfig)
	mux.HandleFunc("/healthz", handleHealthz)
	return mux
}

func TestRootReturnsJSONBanner(t *testing.T) {
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	want := `{"app":"refract-config-server","message":"Relax, refract it"}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestRootIsNotGatedByTheToken(t *testing.T) {
	apiToken = "secret"
	defer func() { apiToken = "" }()
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET / without a token = %d, want 200", rec.Code)
	}
}

// "/" as a ServeMux pattern would swallow every unmatched path; only the
// exact root may answer, everything else stays a 404 as it was before.
func TestOtherPathsStayNotFound(t *testing.T) {
	for _, path := range []string{"/nope", "/backends", "/index.html", "/healthz/x"} {
		rec := httptest.NewRecorder()
		newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
	}
}

func TestRootRejectsWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST / = %d, want 405", rec.Code)
	}
}

func TestExistingEndpointsUnaffected(t *testing.T) {
	apiToken = "secret"
	defer func() { apiToken = "" }()

	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/healthz = %d, want 200", rec.Code)
	}
	rec = httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/config/cds.yaml", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/config/cds.yaml without a token = %d, want 401", rec.Code)
	}
}
