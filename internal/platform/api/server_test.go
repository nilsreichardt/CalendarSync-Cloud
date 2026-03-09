package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthorizeFrontendRequest(t *testing.T) {
	t.Run("allows request when no secret configured", func(t *testing.T) {
		srv := &Server{}
		req := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
		rec := httptest.NewRecorder()
		if !srv.authorizeFrontendRequest(rec, req) {
			t.Fatal("expected request to be allowed")
		}
	})

	t.Run("rejects request with missing secret", func(t *testing.T) {
		srv := &Server{frontendSecret: "top-secret"}
		req := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
		rec := httptest.NewRecorder()
		if srv.authorizeFrontendRequest(rec, req) {
			t.Fatal("expected request to be rejected")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})

	t.Run("allows request with matching secret", func(t *testing.T) {
		srv := &Server{frontendSecret: "top-secret"}
		req := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
		req.Header.Set("X-CalendarSync-Frontend-Secret", "top-secret")
		rec := httptest.NewRecorder()
		if !srv.authorizeFrontendRequest(rec, req) {
			t.Fatal("expected request to be allowed")
		}
	})
}
