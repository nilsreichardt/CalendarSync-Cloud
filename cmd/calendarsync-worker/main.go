package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	platformcrypto "github.com/inovex/CalendarSync/internal/platform/crypto"
	"github.com/inovex/CalendarSync/internal/platform/store"
	"github.com/inovex/CalendarSync/internal/platform/worker"
)

type runOnceRunner interface {
	RunOnce(ctx context.Context) (bool, error)
}

type workerHTTPServer struct {
	runner          runOnceRunner
	schedulerSecret string
}

var runLoopSleep = func() {
	time.Sleep(500 * time.Millisecond)
}

func main() {
	ctx := context.Background()
	st, err := store.NewPostgresStore(mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			log.Printf("store close error: %v", err)
		}
	}()

	tokenCodec, err := newTokenCodec(ctx)
	if err != nil {
		log.Fatal(err)
	}

	runner := worker.NewRunner(st, tokenCodec, mustEnv("GOOGLE_OAUTH_CLIENT_ID"), mustEnv("GOOGLE_OAUTH_CLIENT_SECRET"))
	runner.SetLockLease(runLockLease())
	srv := &workerHTTPServer{
		runner:          runner,
		schedulerSecret: mustEnv("SCHEDULER_SHARED_SECRET"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/internal/worker/run", srv.handleRun)

	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}
	log.Printf("calendarsync-worker listening on :%s", addr)
	log.Fatal(http.ListenAndServe(":"+addr, mux))
}

func (s *workerHTTPServer) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "method not allowed",
		})
		return
	}

	incoming := r.Header.Get("X-Scheduler-Secret")
	if incoming == "" || s.schedulerSecret == "" || incoming != s.schedulerSecret {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"error": "invalid scheduler secret",
		})
		return
	}

	processed, errors := drainRuns(r.Context(), s.runner)
	status := "ok"
	if errors > 0 {
		status = "partial_error"
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"processed": processed,
		"errors":    errors,
		"status":    status,
	})
}

func drainRuns(ctx context.Context, runner runOnceRunner) (int, int) {
	processed := 0
	errors := 0

	for {
		worked, err := runner.RunOnce(ctx)
		if err != nil {
			errors++
			log.Printf("run error: %v", err)
		}
		if !worked {
			return processed, errors
		}
		processed++
		runLoopSleep()
	}
}

func respondJSON(w http.ResponseWriter, status int, payload map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func newTokenCodec(ctx context.Context) (*store.TokenCodec, error) {
	if key := os.Getenv("KMS_CRYPTO_KEY"); key != "" {
		envelope, err := platformcrypto.NewKMSEnvelope(ctx, key, os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
		if err != nil {
			return nil, err
		}
		staticKey := os.Getenv("CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64")
		if staticKey == "" {
			return store.NewTokenCodec(envelope), nil
		}
		legacy, err := platformcrypto.NewStaticEnvelope(staticKey)
		if err != nil {
			return nil, err
		}
		return store.NewTokenCodecWithLegacy(envelope, legacy), nil
	}
	staticKey := mustEnv("CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64")
	envelope, err := platformcrypto.NewStaticEnvelope(staticKey)
	if err != nil {
		return nil, err
	}
	return store.NewTokenCodec(envelope), nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return v
}

func runLockLease() time.Duration {
	v := strings.TrimSpace(os.Getenv("RUN_LOCK_LEASE_SECONDS"))
	if v == "" {
		return 10 * time.Minute
	}
	seconds, err := strconv.Atoi(v)
	if err != nil || seconds <= 0 {
		log.Printf("invalid RUN_LOCK_LEASE_SECONDS=%q, using default 600", v)
		return 10 * time.Minute
	}
	return time.Duration(seconds) * time.Second
}
