package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/api/idtoken"
)

func TestTriggerWorkerRun(t *testing.T) {
	t.Run("no worker url is a no-op", func(t *testing.T) {
		srv := &Server{}
		if err := srv.triggerWorkerRun(context.Background()); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("successful trigger posts with scheduler secret", func(t *testing.T) {
		var gotMethod string
		var gotSecret string
		var gotPath string
		var gotAudience string

		worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotSecret = r.Header.Get("X-Scheduler-Secret")
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
		defer worker.Close()

		srv := &Server{
			workerRunURL:    worker.URL + "/internal/worker/run",
			schedulerSecret: "scheduler-secret",
			workerHTTP:      &http.Client{Timeout: time.Second},
			workerAuth: func(_ context.Context, audience string, _ ...idtoken.ClientOption) (*http.Client, error) {
				gotAudience = audience
				return worker.Client(), nil
			},
		}

		if err := srv.triggerWorkerRun(context.Background()); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if gotMethod != http.MethodPost {
			t.Fatalf("expected %s, got %s", http.MethodPost, gotMethod)
		}
		if gotSecret != "scheduler-secret" {
			t.Fatalf("expected scheduler secret header, got %q", gotSecret)
		}
		if gotPath != "/internal/worker/run" {
			t.Fatalf("expected run path, got %q", gotPath)
		}
		if gotAudience != worker.URL {
			t.Fatalf("expected audience %q, got %q", worker.URL, gotAudience)
		}
	})

	t.Run("returns error on non-2xx", func(t *testing.T) {
		worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "bad worker", http.StatusBadGateway)
		}))
		defer worker.Close()

		srv := &Server{
			workerRunURL: worker.URL,
			workerAuth: func(_ context.Context, _ string, _ ...idtoken.ClientOption) (*http.Client, error) {
				return worker.Client(), nil
			},
		}

		if err := srv.triggerWorkerRun(context.Background()); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when auth client creation fails", func(t *testing.T) {
		srv := &Server{
			workerRunURL: "https://worker.example.com/internal/worker/run",
			workerAuth: func(_ context.Context, _ string, _ ...idtoken.ClientOption) (*http.Client, error) {
				return nil, errors.New("auth failed")
			},
		}

		if err := srv.triggerWorkerRun(context.Background()); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
