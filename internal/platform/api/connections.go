package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/inovex/CalendarSync/internal/platform/store"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type googleUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (s *Server) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.requireUser(r.Context(), w, r)
	if !ok {
		return
	}
	state, err := s.signState(user.ID.String())
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	url := s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	respondJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx := r.Context()
	state, err := s.verifyState(r.URL.Query().Get("state"))
	if err != nil {
		respondErr(w, http.StatusBadRequest, err.Error())
		return
	}
	userID, err := uuid.Parse(state.UserID)
	if err != nil {
		respondErr(w, http.StatusBadRequest, "invalid user id in state")
		return
	}

	token, err := s.oauthConfig.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		respondErr(w, http.StatusBadRequest, err.Error())
		return
	}

	userInfo, err := fetchGoogleUserInfo(ctx, s.oauthConfig.Client(ctx, token))
	if err != nil {
		respondErr(w, http.StatusBadRequest, err.Error())
		return
	}

	connection, err := s.store.CreateGoogleConnection(ctx, userID, userInfo.Sub, userInfo.Email, userInfo.Name, false)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	encrypted, err := s.tokenCodec.EncryptToken(ctx, connection.ID, "google", token)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.store.StoreEncryptedToken(ctx, *encrypted); err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.syncCalendarsFromToken(ctx, connection.ID, token); err != nil {
		respondErr(w, http.StatusBadGateway, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"connectionId": connection.ID.String()})
}

func fetchGoogleUserInfo(ctx context.Context, client *http.Client) (*googleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		by, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo request failed: %s", string(by))
	}
	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.requireUser(r.Context(), w, r)
	if !ok {
		return
	}
	connections, err := s.store.ListGoogleConnections(r.Context(), user.ID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"connections": connections})
}

func (s *Server) handleConnectionByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/connections/")
	if strings.HasSuffix(path, "/calendars") {
		s.handleConnectionCalendars(w, r)
		return
	}

	connectionID, err := parseUUIDFromPath(r.URL.Path)
	if err != nil {
		respondErr(w, http.StatusBadRequest, "invalid connection id")
		return
	}
	user, ok := s.requireUser(r.Context(), w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodDelete {
		if err := s.store.DeleteGoogleConnection(r.Context(), user.ID, connectionID); err != nil {
			respondErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (s *Server) handleCalendars(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.requireUser(r.Context(), w, r)
	if !ok {
		return
	}
	calendars, err := s.store.ListCalendarsByUser(r.Context(), user.ID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"calendars": calendars})
}

func (s *Server) handleConnectionCalendars(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := s.requireUser(r.Context(), w, r)
	if !ok {
		return
	}
	trimmed := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/connections/"), "/calendars")
	connectionID, err := uuid.Parse(strings.Trim(trimmed, "/"))
	if err != nil {
		respondErr(w, http.StatusBadRequest, "invalid connection id")
		return
	}
	belongs, err := s.store.ConnectionBelongsToUser(r.Context(), user.ID, connectionID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !belongs {
		respondErr(w, http.StatusNotFound, "connection not found")
		return
	}
	calendars, err := s.store.ListCalendarsByConnection(r.Context(), connectionID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"calendars": calendars})
}

func (s *Server) RefreshCalendarsForConnection(ctx context.Context, connectionID uuid.UUID) error {
	encrypted, err := s.store.GetEncryptedToken(ctx, connectionID)
	if err != nil {
		return err
	}
	token, err := s.tokenCodec.DecryptToken(ctx, encrypted)
	if err != nil {
		return err
	}
	return s.syncCalendarsFromToken(ctx, connectionID, token)
}

func (s *Server) syncCalendarsFromToken(ctx context.Context, connectionID uuid.UUID, token *oauth2.Token) error {
	svc, err := calendar.NewService(ctx, option.WithHTTPClient(s.oauthConfig.Client(ctx, token)))
	if err != nil {
		return err
	}
	list, err := svc.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return err
	}
	calendars := make([]store.ConnectionCalendar, 0, len(list.Items))
	for _, item := range list.Items {
		calendars = append(calendars, store.ConnectionCalendar{
			ConnectionID: connectionID,
			CalendarID:   item.Id,
			Summary:      item.Summary,
			IsPrimary:    item.Primary,
			AccessRole:   item.AccessRole,
		})
	}
	return s.store.ReplaceConnectionCalendars(ctx, connectionID, calendars)
}
