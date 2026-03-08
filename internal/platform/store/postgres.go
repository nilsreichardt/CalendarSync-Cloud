package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/inovex/CalendarSync/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

func (p *PostgresStore) DB() *sql.DB {
	return p.db
}

func (p *PostgresStore) Close() error {
	return p.db.Close()
}

func (p *PostgresStore) UpsertUser(ctx context.Context, externalID, email string) (*User, error) {
	row := p.db.QueryRowContext(ctx, `
		INSERT INTO users (external_id, email)
		VALUES ($1, $2)
		ON CONFLICT (external_id) DO UPDATE SET email = EXCLUDED.email, updated_at = NOW()
		RETURNING id, external_id, email
	`, externalID, email)
	var user User
	if err := row.Scan(&user.ID, &user.ExternalID, &user.Email); err != nil {
		return nil, err
	}
	return &user, nil
}

func (p *PostgresStore) CreateGoogleConnection(ctx context.Context, userID uuid.UUID, googleSub, email, displayName string, isPrimary bool) (*GoogleConnection, error) {
	row := p.db.QueryRowContext(ctx, `
		INSERT INTO google_connections (user_id, google_sub, email, display_name, is_primary)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, google_sub) DO UPDATE SET
		  email = EXCLUDED.email,
		  display_name = EXCLUDED.display_name,
		  is_primary = EXCLUDED.is_primary,
		  updated_at = NOW()
		RETURNING id, user_id, google_sub, email, display_name, is_primary, created_at
	`, userID, googleSub, email, displayName, isPrimary)
	var c GoogleConnection
	if err := row.Scan(&c.ID, &c.UserID, &c.GoogleSub, &c.Email, &c.DisplayName, &c.IsPrimary, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *PostgresStore) ListGoogleConnections(ctx context.Context, userID uuid.UUID) ([]GoogleConnection, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, user_id, google_sub, email, display_name, is_primary, created_at
		FROM google_connections
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	connections := []GoogleConnection{}
	for rows.Next() {
		var c GoogleConnection
		if err := rows.Scan(&c.ID, &c.UserID, &c.GoogleSub, &c.Email, &c.DisplayName, &c.IsPrimary, &c.CreatedAt); err != nil {
			return nil, err
		}
		connections = append(connections, c)
	}
	return connections, rows.Err()
}

func (p *PostgresStore) ConnectionBelongsToUser(ctx context.Context, userID, connectionID uuid.UUID) (bool, error) {
	row := p.db.QueryRowContext(ctx, `
		SELECT EXISTS(
		  SELECT 1 FROM google_connections WHERE id = $1 AND user_id = $2
		)
	`, connectionID, userID)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (p *PostgresStore) DeleteGoogleConnection(ctx context.Context, userID, connectionID uuid.UUID) error {
	_, err := p.db.ExecContext(ctx, `DELETE FROM google_connections WHERE id = $1 AND user_id = $2`, connectionID, userID)
	return err
}

func (p *PostgresStore) ReplaceConnectionCalendars(ctx context.Context, connectionID uuid.UUID, calendars []ConnectionCalendar) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM connection_calendars WHERE connection_id = $1`, connectionID); err != nil {
		return err
	}

	for _, c := range calendars {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO connection_calendars (connection_id, calendar_id, summary, is_primary, access_role)
			VALUES ($1, $2, $3, $4, $5)
		`, connectionID, c.CalendarID, c.Summary, c.IsPrimary, c.AccessRole); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (p *PostgresStore) ListCalendarsByUser(ctx context.Context, userID uuid.UUID) ([]ConnectionCalendar, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT cc.connection_id, cc.calendar_id, cc.summary, cc.is_primary, cc.access_role
		FROM connection_calendars cc
		INNER JOIN google_connections gc ON gc.id = cc.connection_id
		WHERE gc.user_id = $1
		ORDER BY gc.email ASC, cc.summary ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	calendars := []ConnectionCalendar{}
	for rows.Next() {
		var c ConnectionCalendar
		if err := rows.Scan(&c.ConnectionID, &c.CalendarID, &c.Summary, &c.IsPrimary, &c.AccessRole); err != nil {
			return nil, err
		}
		calendars = append(calendars, c)
	}
	return calendars, rows.Err()
}

func (p *PostgresStore) ListCalendarsByConnection(ctx context.Context, connectionID uuid.UUID) ([]ConnectionCalendar, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT connection_id, calendar_id, summary, is_primary, access_role
		FROM connection_calendars
		WHERE connection_id = $1
		ORDER BY summary ASC
	`, connectionID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	calendars := []ConnectionCalendar{}
	for rows.Next() {
		var c ConnectionCalendar
		if err := rows.Scan(&c.ConnectionID, &c.CalendarID, &c.Summary, &c.IsPrimary, &c.AccessRole); err != nil {
			return nil, err
		}
		calendars = append(calendars, c)
	}
	return calendars, rows.Err()
}

func (p *PostgresStore) StoreEncryptedToken(ctx context.Context, token EncryptedToken) error {
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO encrypted_oauth_tokens (connection_id, provider, cipher_text, dek_cipher_text, nonce, key_version)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (connection_id) DO UPDATE SET
		  provider = EXCLUDED.provider,
		  cipher_text = EXCLUDED.cipher_text,
		  dek_cipher_text = EXCLUDED.dek_cipher_text,
		  nonce = EXCLUDED.nonce,
		  key_version = EXCLUDED.key_version,
		  updated_at = NOW()
	`, token.ConnectionID, token.Provider, token.CipherText, token.DEKCipher, token.Nonce, token.KeyVersion)
	return err
}

func (p *PostgresStore) GetEncryptedToken(ctx context.Context, connectionID uuid.UUID) (*EncryptedToken, error) {
	row := p.db.QueryRowContext(ctx, `
		SELECT connection_id, provider, cipher_text, dek_cipher_text, nonce, key_version
		FROM encrypted_oauth_tokens
		WHERE connection_id = $1
	`, connectionID)
	var t EncryptedToken
	if err := row.Scan(&t.ConnectionID, &t.Provider, &t.CipherText, &t.DEKCipher, &t.Nonce, &t.KeyVersion); err != nil {
		return nil, err
	}
	return &t, nil
}

func (p *PostgresStore) CreateRule(ctx context.Context, rule SyncRule) (*SyncRule, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO sync_rules (
		  user_id, name, source_connection_id, source_calendar_id, target_connection_id, target_calendar_id,
		  payload_mode, direction, sync_start_identifier, sync_start_offset, sync_end_identifier, sync_end_offset,
		  enabled, schedule, dry_run, update_concurrency
		) VALUES (
		  $1, $2, $3, $4, $5, $6,
		  $7, $8, $9, $10, $11, $12,
		  $13, $14, $15, $16
		)
		RETURNING id, created_at, updated_at
	`, rule.UserID, rule.Name, rule.SourceConnectionID, rule.SourceCalendarID, rule.TargetConnectionID, rule.TargetCalendarID,
		rule.PayloadMode, rule.Direction, rule.StartTime.Identifier, rule.StartTime.Offset, rule.EndTime.Identifier, rule.EndTime.Offset,
		rule.Enabled, rule.Schedule, rule.DryRun, rule.UpdateConcurrency)

	if err := row.Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return nil, err
	}

	if err := p.replaceRuleParts(ctx, tx, rule.ID, rule.Filters, rule.Transformations); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return p.GetRule(ctx, rule.UserID, rule.ID)
}

func (p *PostgresStore) UpdateRule(ctx context.Context, rule SyncRule) (*SyncRule, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	res, err := tx.ExecContext(ctx, `
		UPDATE sync_rules SET
		  name = $1,
		  source_connection_id = $2,
		  source_calendar_id = $3,
		  target_connection_id = $4,
		  target_calendar_id = $5,
		  payload_mode = $6,
		  direction = $7,
		  sync_start_identifier = $8,
		  sync_start_offset = $9,
		  sync_end_identifier = $10,
		  sync_end_offset = $11,
		  enabled = $12,
		  schedule = $13,
		  dry_run = $14,
		  update_concurrency = $15,
		  updated_at = NOW()
		WHERE id = $16 AND user_id = $17
	`, rule.Name, rule.SourceConnectionID, rule.SourceCalendarID, rule.TargetConnectionID, rule.TargetCalendarID, rule.PayloadMode, rule.Direction,
		rule.StartTime.Identifier, rule.StartTime.Offset, rule.EndTime.Identifier, rule.EndTime.Offset,
		rule.Enabled, rule.Schedule, rule.DryRun, rule.UpdateConcurrency, rule.ID, rule.UserID)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, sql.ErrNoRows
	}

	if err := p.replaceRuleParts(ctx, tx, rule.ID, rule.Filters, rule.Transformations); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return p.GetRule(ctx, rule.UserID, rule.ID)
}

func (p *PostgresStore) replaceRuleParts(ctx context.Context, tx *sql.Tx, ruleID uuid.UUID, filters []config.Filter, transformations []config.Transformer) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM sync_rule_filters WHERE rule_id = $1`, ruleID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sync_rule_transformations WHERE rule_id = $1`, ruleID); err != nil {
		return err
	}

	for i, f := range filters {
		cfg, err := json.Marshal(f.Config)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO sync_rule_filters (rule_id, name, config, ordinal) VALUES ($1, $2, $3, $4)
		`, ruleID, f.Name, cfg, i); err != nil {
			return err
		}
	}

	for i, t := range transformations {
		cfg, err := json.Marshal(t.Config)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO sync_rule_transformations (rule_id, name, config, ordinal) VALUES ($1, $2, $3, $4)
		`, ruleID, t.Name, cfg, i); err != nil {
			return err
		}
	}
	return nil
}

func (p *PostgresStore) ListRules(ctx context.Context, userID uuid.UUID) ([]SyncRule, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id FROM sync_rules WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	rules := []SyncRule{}
	for rows.Next() {
		var ruleID uuid.UUID
		if err := rows.Scan(&ruleID); err != nil {
			return nil, err
		}
		rule, err := p.GetRule(ctx, userID, ruleID)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}
	return rules, rows.Err()
}

func (p *PostgresStore) GetRule(ctx context.Context, userID, ruleID uuid.UUID) (*SyncRule, error) {
	row := p.db.QueryRowContext(ctx, `
		SELECT
		  id, user_id, name, source_connection_id, source_calendar_id, target_connection_id, target_calendar_id,
		  payload_mode, direction, schedule, enabled, dry_run, update_concurrency,
		  sync_start_identifier, sync_start_offset, sync_end_identifier, sync_end_offset,
		  created_at, updated_at
		FROM sync_rules
		WHERE id = $1 AND user_id = $2
	`, ruleID, userID)
	var r SyncRule
	if err := row.Scan(
		&r.ID, &r.UserID, &r.Name, &r.SourceConnectionID, &r.SourceCalendarID, &r.TargetConnectionID, &r.TargetCalendarID,
		&r.PayloadMode, &r.Direction, &r.Schedule, &r.Enabled, &r.DryRun, &r.UpdateConcurrency,
		&r.StartTime.Identifier, &r.StartTime.Offset, &r.EndTime.Identifier, &r.EndTime.Offset, &r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return nil, err
	}
	filters, err := p.loadFilters(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	transformations, err := p.loadTransformations(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	r.Filters = filters
	r.Transformations = transformations
	return &r, nil
}

func (p *PostgresStore) loadFilters(ctx context.Context, ruleID uuid.UUID) ([]config.Filter, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT name, config
		FROM sync_rule_filters
		WHERE rule_id = $1
		ORDER BY ordinal ASC
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var result []config.Filter
	for rows.Next() {
		var name string
		var raw []byte
		if err := rows.Scan(&name, &raw); err != nil {
			return nil, err
		}
		cfg := config.CustomMap{}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return nil, err
			}
		}
		result = append(result, config.Filter{Name: name, Config: cfg})
	}
	return result, rows.Err()
}

func (p *PostgresStore) loadTransformations(ctx context.Context, ruleID uuid.UUID) ([]config.Transformer, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT name, config
		FROM sync_rule_transformations
		WHERE rule_id = $1
		ORDER BY ordinal ASC
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var result []config.Transformer
	for rows.Next() {
		var name string
		var raw []byte
		if err := rows.Scan(&name, &raw); err != nil {
			return nil, err
		}
		cfg := config.CustomMap{}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return nil, err
			}
		}
		result = append(result, config.Transformer{Name: name, Config: cfg})
	}
	return result, rows.Err()
}

func (p *PostgresStore) DeleteRule(ctx context.Context, userID, ruleID uuid.UUID) error {
	_, err := p.db.ExecContext(ctx, `DELETE FROM sync_rules WHERE id = $1 AND user_id = $2`, ruleID, userID)
	return err
}

func (p *PostgresStore) CreateRun(ctx context.Context, ruleID uuid.UUID, triggerType string) (*SyncRun, error) {
	row := p.db.QueryRowContext(ctx, `
		INSERT INTO sync_runs (rule_id, trigger_type, status)
		VALUES ($1, $2, 'pending')
		RETURNING id, rule_id, trigger_type, status, summary, error, started_at, finished_at, created_at
	`, ruleID, triggerType)
	var run SyncRun
	if err := row.Scan(&run.ID, &run.RuleID, &run.TriggerType, &run.Status, &run.Summary, &run.Error, &run.StartedAt, &run.FinishedAt, &run.CreatedAt); err != nil {
		return nil, err
	}
	return &run, nil
}

func (p *PostgresStore) CreateCleanupRun(ctx context.Context, userID, ruleID uuid.UUID) (*SyncRun, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	res, err := tx.ExecContext(ctx, `
		UPDATE sync_rules
		SET enabled = FALSE, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
	`, ruleID, userID)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, sql.ErrNoRows
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO sync_runs (rule_id, trigger_type, status)
		VALUES ($1, 'cleanup', 'pending')
		RETURNING id, rule_id, trigger_type, status, summary, error, started_at, finished_at, created_at
	`, ruleID)
	var run SyncRun
	if err := row.Scan(&run.ID, &run.RuleID, &run.TriggerType, &run.Status, &run.Summary, &run.Error, &run.StartedAt, &run.FinishedAt, &run.CreatedAt); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &run, nil
}

func (p *PostgresStore) HasActiveRun(ctx context.Context, ruleID uuid.UUID) (bool, error) {
	row := p.db.QueryRowContext(ctx, `
		SELECT EXISTS(
		  SELECT 1
		  FROM sync_runs
		  WHERE rule_id = $1
		    AND status IN ('pending', 'running')
		)
	`, ruleID)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (p *PostgresStore) LastRunCreatedAt(ctx context.Context, ruleID uuid.UUID) (sql.NullTime, error) {
	row := p.db.QueryRowContext(ctx, `
		SELECT created_at
		FROM sync_runs
		WHERE rule_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, ruleID)
	var created sql.NullTime
	if err := row.Scan(&created); err != nil {
		if err == sql.ErrNoRows {
			return sql.NullTime{}, nil
		}
		return sql.NullTime{}, err
	}
	return created, nil
}

func (p *PostgresStore) ListRuns(ctx context.Context, userID uuid.UUID) ([]SyncRun, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT r.id, r.rule_id, r.trigger_type, r.status, r.summary, r.error, r.started_at, r.finished_at, r.created_at
		FROM sync_runs r
		INNER JOIN sync_rules sr ON sr.id = r.rule_id
		WHERE sr.user_id = $1
		ORDER BY r.created_at DESC
		LIMIT 200
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	runs := []SyncRun{}
	for rows.Next() {
		var run SyncRun
		if err := rows.Scan(&run.ID, &run.RuleID, &run.TriggerType, &run.Status, &run.Summary, &run.Error, &run.StartedAt, &run.FinishedAt, &run.CreatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (p *PostgresStore) GetRun(ctx context.Context, userID, runID uuid.UUID) (*SyncRun, error) {
	row := p.db.QueryRowContext(ctx, `
		SELECT r.id, r.rule_id, r.trigger_type, r.status, r.summary, r.error, r.started_at, r.finished_at, r.created_at
		FROM sync_runs r
		INNER JOIN sync_rules sr ON sr.id = r.rule_id
		WHERE r.id = $1 AND sr.user_id = $2
	`, runID, userID)
	var run SyncRun
	if err := row.Scan(&run.ID, &run.RuleID, &run.TriggerType, &run.Status, &run.Summary, &run.Error, &run.StartedAt, &run.FinishedAt, &run.CreatedAt); err != nil {
		return nil, err
	}
	return &run, nil
}

func (p *PostgresStore) ClaimNextPendingRun(ctx context.Context) (*SyncRun, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	row := tx.QueryRowContext(ctx, `
		WITH next_run AS (
		  SELECT id
		  FROM sync_runs
		  WHERE status = 'pending'
		  ORDER BY created_at ASC
		  LIMIT 1
		  FOR UPDATE SKIP LOCKED
		)
		UPDATE sync_runs sr
		SET status = 'running', started_at = NOW()
		FROM next_run
		WHERE sr.id = next_run.id
		RETURNING sr.id, sr.rule_id, sr.trigger_type, sr.status, sr.summary, sr.error, sr.started_at, sr.finished_at, sr.created_at
	`)
	var run SyncRun
	if err := row.Scan(&run.ID, &run.RuleID, &run.TriggerType, &run.Status, &run.Summary, &run.Error, &run.StartedAt, &run.FinishedAt, &run.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &run, nil
}

func (p *PostgresStore) MarkRunDone(ctx context.Context, runID uuid.UUID, summary map[string]int) error {
	by, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = p.db.ExecContext(ctx, `
		UPDATE sync_runs
		SET status = 'success', summary = $2, finished_at = NOW()
		WHERE id = $1
	`, runID, by)
	return err
}

func (p *PostgresStore) MarkRunError(ctx context.Context, runID uuid.UUID, runErr error) error {
	_, err := p.db.ExecContext(ctx, `
		UPDATE sync_runs
		SET status = 'failed', error = $2, finished_at = NOW()
		WHERE id = $1
	`, runID, runErr.Error())
	return err
}

func (p *PostgresStore) AppendRunLog(ctx context.Context, runID uuid.UUID, level, message string) error {
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO sync_run_logs (run_id, level, message) VALUES ($1, $2, $3)
	`, runID, level, message)
	return err
}

func (p *PostgresStore) GetRuleByID(ctx context.Context, ruleID uuid.UUID) (*SyncRule, error) {
	row := p.db.QueryRowContext(ctx, `SELECT user_id FROM sync_rules WHERE id = $1`, ruleID)
	var userID uuid.UUID
	if err := row.Scan(&userID); err != nil {
		return nil, err
	}
	return p.GetRule(ctx, userID, ruleID)
}

func (p *PostgresStore) AcquireRuleLock(ctx context.Context, ruleID, runID uuid.UUID) (bool, error) {
	_, err := p.db.ExecContext(ctx, `INSERT INTO rule_locks (rule_id, run_id) VALUES ($1, $2)`, ruleID, runID)
	if err == nil {
		return true, nil
	}
	if strings.Contains(err.Error(), "duplicate key") {
		return false, nil
	}
	return false, err
}

func (p *PostgresStore) ReleaseRuleLock(ctx context.Context, ruleID uuid.UUID) error {
	_, err := p.db.ExecContext(ctx, `DELETE FROM rule_locks WHERE rule_id = $1`, ruleID)
	return err
}
