package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRootReturnsBanner(t *testing.T) {
	rec := httptest.NewRecorder()
	root(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	want := `{"app":"refract","message":"Relax, refract it"}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestOtherPathsStillEcho(t *testing.T) {
	rec := httptest.NewRecorder()
	root(rec, httptest.NewRequest(http.MethodGet, "/anything?a=1", nil))
	if !strings.Contains(rec.Body.String(), `"method":"GET"`) || !strings.Contains(rec.Body.String(), `"a":"1"`) {
		t.Errorf("expected echo response, got %s", rec.Body.String())
	}
}

func TestPostToRootStillEchoes(t *testing.T) {
	rec := httptest.NewRecorder()
	root(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hi")))
	if !strings.Contains(rec.Body.String(), `"data":"hi"`) {
		t.Errorf("expected echo response, got %s", rec.Body.String())
	}
}
