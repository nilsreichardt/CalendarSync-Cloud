package api

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.requireUser(r.Context(), w, r)
	if !ok {
		return
	}
	runs, err := s.store.ListRuns(r.Context(), user.ID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"runs": runs})
}

func (s *Server) handleRunByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	runID, err := parseUUIDFromPath(r.URL.Path)
	if err != nil {
		respondErr(w, http.StatusBadRequest, "invalid run id")
		return
	}
	user, ok := s.requireUser(r.Context(), w, r)
	if !ok {
		return
	}
	run, err := s.store.GetRun(r.Context(), user.ID, runID)
	if err != nil {
		respondErr(w, http.StatusNotFound, "run not found")
		return
	}
	respondJSON(w, http.StatusOK, run)
}

func (s *Server) handleSchedulerDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	incoming := r.Header.Get("X-Scheduler-Secret")
	if incoming == "" || s.schedulerSecret == "" || incoming != s.schedulerSecret {
		respondErr(w, http.StatusUnauthorized, "invalid scheduler secret")
		return
	}
	now := time.Now().UTC()
	rows, err := s.store.DB().QueryContext(r.Context(), `
		SELECT id FROM sync_rules WHERE enabled = TRUE
	`)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	enqueued := 0
	minInterval := dispatchInterval()
	for rows.Next() {
		var ruleID uuid.UUID
		if err := rows.Scan(&ruleID); err != nil {
			respondErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		hasActive, err := s.store.HasActiveRun(r.Context(), ruleID)
		if err != nil {
			respondErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if hasActive {
			continue
		}

		last, err := s.store.LastRunCreatedAt(r.Context(), ruleID)
		if err != nil {
			respondErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if last.Valid && now.Sub(last.Time) < minInterval {
			continue
		}

		if _, err := s.store.CreateRun(r.Context(), ruleID, "schedule"); err == nil {
			enqueued++
		}
	}
	if err := rows.Err(); err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"enqueued": enqueued,
		"status":   strings.ToLower("OK"),
	})
}

func dispatchInterval() time.Duration {
	v := strings.TrimSpace(os.Getenv("DISPATCH_MIN_INTERVAL_SECONDS"))
	if v == "" {
		return 30 * time.Second
	}
	sec, err := strconv.Atoi(v)
	if err != nil || sec <= 0 {
		return 30 * time.Second
	}
	return time.Duration(sec) * time.Second
}
