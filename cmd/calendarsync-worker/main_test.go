package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type runOnceResult struct {
	worked bool
	err    error
}

type stubRunner struct {
	results []runOnceResult
	calls   int
}

func (s *stubRunner) RunOnce(_ context.Context) (bool, error) {
	if s.calls >= len(s.results) {
		return false, nil
	}
	result := s.results[s.calls]
	s.calls++
	return result.worked, result.err
}

func TestHandleRunRejectsMissingSecret(t *testing.T) {
	srv := &workerHTTPServer{
		runner:          &stubRunner{},
		schedulerSecret: "expected-secret",
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/worker/run", nil)
	rec := httptest.NewRecorder()

	srv.handleRun(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestHandleRunRejectsWrongSecret(t *testing.T) {
	srv := &workerHTTPServer{
		runner:          &stubRunner{},
		schedulerSecret: "expected-secret",
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/worker/run", nil)
	req.Header.Set("X-Scheduler-Secret", "wrong-secret")
	rec := httptest.NewRecorder()

	srv.handleRun(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestHandleRunAcceptsCorrectSecretAndNoWork(t *testing.T) {
	srv := &workerHTTPServer{
		runner: &stubRunner{
			results: []runOnceResult{
				{worked: false, err: nil},
			},
		},
		schedulerSecret: "expected-secret",
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/worker/run", nil)
	req.Header.Set("X-Scheduler-Secret", "expected-secret")
	rec := httptest.NewRecorder()

	srv.handleRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload["processed"] != float64(0) || payload["errors"] != float64(0) || payload["status"] != "ok" {
		t.Fatalf("unexpected response payload: %+v", payload)
	}
}

func TestHandleRunContinuesOnErrors(t *testing.T) {
	originalSleep := runLoopSleep
	runLoopSleep = func() {}
	defer func() {
		runLoopSleep = originalSleep
	}()

	srv := &workerHTTPServer{
		runner: &stubRunner{
			results: []runOnceResult{
				{worked: true, err: errors.New("boom")},
				{worked: true, err: nil},
				{worked: false, err: nil},
			},
		},
		schedulerSecret: "expected-secret",
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/worker/run", nil)
	req.Header.Set("X-Scheduler-Secret", "expected-secret")
	rec := httptest.NewRecorder()

	srv.handleRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload["processed"] != float64(2) || payload["errors"] != float64(1) || payload["status"] != "partial_error" {
		t.Fatalf("unexpected response payload: %+v", payload)
	}
}

func TestRunLockLeaseDefault(t *testing.T) {
	t.Setenv("RUN_LOCK_LEASE_SECONDS", "")
	if got := runLockLease(); got != 10*time.Minute {
		t.Fatalf("expected default 10m lease, got %s", got)
	}
}

func TestRunLockLeaseFromEnv(t *testing.T) {
	t.Setenv("RUN_LOCK_LEASE_SECONDS", "120")
	if got := runLockLease(); got != 120*time.Second {
		t.Fatalf("expected 120s lease, got %s", got)
	}
}

func TestRunLockLeaseInvalidValueFallsBackToDefault(t *testing.T) {
	t.Setenv("RUN_LOCK_LEASE_SECONDS", "nope")
	got := runLockLease()
	if got != 10*time.Minute {
		t.Fatalf("expected default 10m lease for invalid value, got %s", got)
	}
}

func TestRunLockLeaseNonPositiveFallsBackToDefault(t *testing.T) {
	t.Setenv("RUN_LOCK_LEASE_SECONDS", "0")
	got := runLockLease()
	if got != 10*time.Minute {
		t.Fatalf("expected default 10m lease for non-positive value, got %s", got)
	}
}
