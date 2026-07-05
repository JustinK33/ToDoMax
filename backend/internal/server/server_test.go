package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzWithoutDB(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
