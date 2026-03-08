package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/inovex/CalendarSync/internal/platform/store"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

type Server struct {
	store           *store.PostgresStore
	tokenCodec      *store.TokenCodec
	oauthConfig     oauth2.Config
	stateSecret     []byte
	schedulerSecret string
}

type Config struct {
	DatabaseURL         string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleRedirectURL   string
	OAuthStateSecretB64 string
	SchedulerSecret     string
}

func NewServer(ctx context.Context, cfg Config, tokenCodec *store.TokenCodec) (*Server, error) {
	st, err := store.NewPostgresStore(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	stateSecret, err := base64.StdEncoding.DecodeString(cfg.OAuthStateSecretB64)
	if err != nil {
		return nil, fmt.Errorf("invalid OAUTH_STATE_SECRET_B64: %w", err)
	}
	return &Server{
		store:      st,
		tokenCodec: tokenCodec,
		oauthConfig: oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Endpoint:     google.Endpoint,
			Scopes: []string{
				"openid",
				"email",
				"profile",
				calendar.CalendarReadonlyScope,
				calendar.CalendarEventsScope,
			},
		},
		stateSecret:     stateSecret,
		schedulerSecret: cfg.SchedulerSecret,
	}, nil
}

func (s *Server) Close() error {
	return s.store.Close()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/connections/google/start", s.handleGoogleStart)
	mux.HandleFunc("/api/connections/google/callback", s.handleGoogleCallback)
	mux.HandleFunc("/api/connections", s.handleConnections)
	mux.HandleFunc("/api/connections/", s.handleConnectionByID)
	mux.HandleFunc("/api/calendars", s.handleCalendars)

	mux.HandleFunc("/api/rules", s.handleRules)
	mux.HandleFunc("/api/rules/", s.handleRuleByID)

	mux.HandleFunc("/api/runs", s.handleRuns)
	mux.HandleFunc("/api/runs/", s.handleRunByID)
	mux.HandleFunc("/internal/scheduler/dispatch", s.handleSchedulerDispatch)

	return mux
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondErr(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func (s *Server) requireUser(ctx context.Context, w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	externalID := strings.TrimSpace(r.Header.Get("X-User-ID"))
	email := strings.TrimSpace(r.Header.Get("X-User-Email"))
	if externalID == "" || email == "" {
		respondErr(w, http.StatusUnauthorized, "missing X-User-ID or X-User-Email")
		return nil, false
	}
	user, err := s.store.UpsertUser(ctx, externalID, email)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	return user, true
}

func parseUUIDFromPath(path string) (uuid.UUID, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return uuid.Nil, errors.New("invalid path")
	}
	return uuid.Parse(parts[len(parts)-1])
}

func parseInt(v string, fallback int) int {
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
