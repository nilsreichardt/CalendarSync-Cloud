package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/inovex/CalendarSync/internal/config"
	"github.com/inovex/CalendarSync/internal/platform/store"
)

type rulePayload struct {
	Name               string               `json:"name"`
	SourceConnectionID string               `json:"sourceConnectionId"`
	SourceCalendarID   string               `json:"sourceCalendarId"`
	TargetConnectionID string               `json:"targetConnectionId"`
	TargetCalendarID   string               `json:"targetCalendarId"`
	PayloadMode        string               `json:"payloadMode"`
	Direction          string               `json:"direction"`
	Schedule           string               `json:"schedule"`
	Enabled            bool                 `json:"enabled"`
	DryRun             bool                 `json:"dryRun"`
	UpdateConcurrency  int                  `json:"updateConcurrency"`
	StartTime          config.SyncTime      `json:"start"`
	EndTime            config.SyncTime      `json:"end"`
	Filters            []config.Filter      `json:"filters"`
	Transformations    []config.Transformer `json:"transformations"`
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(r.Context(), w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rules, err := s.store.ListRules(r.Context(), user.ID)
		if err != nil {
			respondErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"rules": rules})
	case http.MethodPost:
		var payload rulePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			respondErr(w, http.StatusBadRequest, err.Error())
			return
		}
		rule, err := payload.toRule(user.ID)
		if err != nil {
			respondErr(w, http.StatusBadRequest, err.Error())
			return
		}
		created, err := s.store.CreateRule(r.Context(), rule)
		if err != nil {
			respondErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusCreated, created)
	default:
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleRuleByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/rules/")
	if strings.HasSuffix(path, "/run") {
		s.handleRuleRun(w, r)
		return
	}
	if strings.HasSuffix(path, "/cleanup") {
		s.handleRuleCleanup(w, r)
		return
	}
	ruleID, err := parseUUIDFromPath(r.URL.Path)
	if err != nil {
		respondErr(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	user, ok := s.requireUser(r.Context(), w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var payload rulePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			respondErr(w, http.StatusBadRequest, err.Error())
			return
		}
		rule, err := payload.toRule(user.ID)
		if err != nil {
			respondErr(w, http.StatusBadRequest, err.Error())
			return
		}
		rule.ID = ruleID
		updated, err := s.store.UpdateRule(r.Context(), rule)
		if err != nil {
			if err == sql.ErrNoRows {
				respondErr(w, http.StatusNotFound, "rule not found")
				return
			}
			respondErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteRule(r.Context(), user.ID, ruleID); err != nil {
			respondErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleRuleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.requireUser(r.Context(), w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondErr(w, http.StatusBadRequest, "invalid path")
		return
	}
	ruleID, err := uuid.Parse(parts[2])
	if err != nil {
		respondErr(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	if _, err := s.store.GetRule(r.Context(), user.ID, ruleID); err != nil {
		respondErr(w, http.StatusNotFound, "rule not found")
		return
	}
	run, err := s.store.CreateRun(r.Context(), ruleID, "manual")
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusAccepted, run)
}

func (s *Server) handleRuleCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.requireUser(r.Context(), w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondErr(w, http.StatusBadRequest, "invalid path")
		return
	}
	ruleID, err := uuid.Parse(parts[2])
	if err != nil {
		respondErr(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	if _, err := s.store.GetRule(r.Context(), user.ID, ruleID); err != nil {
		respondErr(w, http.StatusNotFound, "rule not found")
		return
	}
	run, err := s.store.CreateCleanupRun(r.Context(), user.ID, ruleID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondErr(w, http.StatusNotFound, "rule not found")
			return
		}
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusAccepted, run)
}

func (p rulePayload) toRule(userID uuid.UUID) (store.SyncRule, error) {
	sourceConnectionID, err := uuid.Parse(p.SourceConnectionID)
	if err != nil {
		return store.SyncRule{}, err
	}
	targetConnectionID, err := uuid.Parse(p.TargetConnectionID)
	if err != nil {
		return store.SyncRule{}, err
	}
	return store.SyncRule{
		UserID:             userID,
		Name:               p.Name,
		SourceConnectionID: sourceConnectionID,
		SourceCalendarID:   p.SourceCalendarID,
		TargetConnectionID: targetConnectionID,
		TargetCalendarID:   p.TargetCalendarID,
		PayloadMode:        p.PayloadMode,
		Direction:          p.Direction,
		Schedule:           p.Schedule,
		Enabled:            p.Enabled,
		DryRun:             p.DryRun,
		UpdateConcurrency:  p.UpdateConcurrency,
		StartTime:          p.StartTime,
		EndTime:            p.EndTime,
		Filters:            p.Filters,
		Transformations:    p.Transformations,
	}, nil
}
