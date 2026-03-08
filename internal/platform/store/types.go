package store

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/inovex/CalendarSync/internal/config"
)

type User struct {
	ID         uuid.UUID
	ExternalID string
	Email      string
}

type GoogleConnection struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"userId"`
	GoogleSub   string    `json:"googleSub"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	IsPrimary   bool      `json:"isPrimary"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ConnectionCalendar struct {
	ConnectionID uuid.UUID `json:"connectionId"`
	CalendarID   string    `json:"calendarId"`
	Summary      string    `json:"summary"`
	IsPrimary    bool      `json:"isPrimary"`
	AccessRole   string    `json:"accessRole"`
}

type EncryptedToken struct {
	ConnectionID uuid.UUID
	Provider     string
	CipherText   []byte
	DEKCipher    []byte
	Nonce        []byte
	KeyVersion   string
}

type SyncRule struct {
	ID                 uuid.UUID              `json:"id"`
	UserID             uuid.UUID              `json:"userId"`
	Name               string                 `json:"name"`
	SourceConnectionID uuid.UUID              `json:"sourceConnectionId"`
	SourceCalendarID   string                 `json:"sourceCalendarId"`
	TargetConnectionID uuid.UUID              `json:"targetConnectionId"`
	TargetCalendarID   string                 `json:"targetCalendarId"`
	PayloadMode        string                 `json:"payloadMode"`
	Direction          string                 `json:"direction"`
	Schedule           string                 `json:"schedule"`
	Enabled            bool                   `json:"enabled"`
	DryRun             bool                   `json:"dryRun"`
	UpdateConcurrency  int                    `json:"updateConcurrency"`
	StartTime          config.SyncTime        `json:"start"`
	EndTime            config.SyncTime        `json:"end"`
	Filters            []config.Filter        `json:"filters"`
	Transformations    []config.Transformer   `json:"transformations"`
	CreatedAt          time.Time              `json:"createdAt"`
	UpdatedAt          time.Time              `json:"updatedAt"`
	Options            map[string]interface{} `json:"options,omitempty"`
}

type SyncRun struct {
	ID          uuid.UUID       `json:"id"`
	RuleID      uuid.UUID       `json:"ruleId"`
	TriggerType string          `json:"triggerType"`
	Status      string          `json:"status"`
	Summary     json.RawMessage `json:"summary"`
	Error       string          `json:"error"`
	StartedAt   *time.Time      `json:"startedAt,omitempty"`
	FinishedAt  *time.Time      `json:"finishedAt,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
}
