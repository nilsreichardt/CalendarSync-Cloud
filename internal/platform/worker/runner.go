package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/inovex/CalendarSync/internal/adapter"
	"github.com/inovex/CalendarSync/internal/auth"
	"github.com/inovex/CalendarSync/internal/config"
	"github.com/inovex/CalendarSync/internal/platform/store"
	"github.com/inovex/CalendarSync/internal/sync"
)

type Runner struct {
	store              *store.PostgresStore
	tokenCodec         *store.TokenCodec
	googleClientID     string
	googleClientSecret string
}

func NewRunner(st *store.PostgresStore, tokenCodec *store.TokenCodec, googleClientID, googleClientSecret string) *Runner {
	return &Runner{
		store:              st,
		tokenCodec:         tokenCodec,
		googleClientID:     googleClientID,
		googleClientSecret: googleClientSecret,
	}
}

func (r *Runner) RunOnce(ctx context.Context) (bool, error) {
	run, err := r.store.ClaimNextPendingRun(ctx)
	if err != nil {
		return false, err
	}
	if run == nil {
		return false, nil
	}
	err = r.executeRun(ctx, run)
	if err != nil {
		_ = r.store.MarkRunError(ctx, run.ID, err)
		return true, err
	}
	return true, nil
}

func (r *Runner) executeRun(ctx context.Context, run *store.SyncRun) error {
	rule, err := r.store.GetRuleByID(ctx, run.RuleID)
	if err != nil {
		return err
	}

	locked, err := r.store.AcquireRuleLock(ctx, rule.ID, run.ID)
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("rule is already running")
	}
	defer func() {
		if releaseErr := r.store.ReleaseRuleLock(ctx, rule.ID); releaseErr != nil {
			log.Error("failed to release rule lock", "rule_id", rule.ID, "err", releaseErr)
		}
	}()

	sourceTokenRaw, err := r.store.GetEncryptedToken(ctx, rule.SourceConnectionID)
	if err != nil {
		return err
	}
	targetTokenRaw, err := r.store.GetEncryptedToken(ctx, rule.TargetConnectionID)
	if err != nil {
		return err
	}
	sourceToken, err := r.tokenCodec.DecryptToken(ctx, sourceTokenRaw)
	if err != nil {
		return err
	}
	targetToken, err := r.tokenCodec.DecryptToken(ctx, targetTokenRaw)
	if err != nil {
		return err
	}

	storage := &dbTokenStorage{
		calendars: map[string]calendarTokenBinding{
			rule.SourceCalendarID: {
				connectionID: rule.SourceConnectionID,
				token: auth.OAuth2Object{
					AccessToken:  sourceToken.AccessToken,
					RefreshToken: sourceToken.RefreshToken,
					Expiry:       sourceToken.Expiry.Format(time.RFC3339),
					TokenType:    sourceToken.TokenType,
				},
			},
			rule.TargetCalendarID: {
				connectionID: rule.TargetConnectionID,
				token: auth.OAuth2Object{
					AccessToken:  targetToken.AccessToken,
					RefreshToken: targetToken.RefreshToken,
					Expiry:       targetToken.Expiry.Format(time.RFC3339),
					TokenType:    targetToken.TokenType,
				},
			},
		},
	}
	storage.onWrite = func(calendarID string, token auth.OAuth2Object) error {
		binding, ok := storageLookup(rule, calendarID)
		if !ok {
			return nil
		}
		expiry, _ := time.Parse(time.RFC3339, token.Expiry)
		newToken := sourceToken
		if binding == rule.TargetConnectionID {
			newToken = targetToken
		}
		newToken.AccessToken = token.AccessToken
		newToken.RefreshToken = token.RefreshToken
		newToken.TokenType = token.TokenType
		newToken.Expiry = expiry
		encrypted, err := r.tokenCodec.EncryptToken(ctx, binding, "google", newToken)
		if err != nil {
			return err
		}
		return r.store.StoreEncryptedToken(ctx, *encrypted)
	}

	cfg := config.File{
		Source: config.Source{
			Adapter: config.Adapter{
				Type:     "google",
				Calendar: rule.SourceCalendarID,
				OAuth: config.OAuth{
					ClientID:  r.googleClientID,
					ClientKey: r.googleClientSecret,
				},
			},
		},
		Sink: config.Sink{
			Adapter: config.Adapter{
				Type:     "google",
				Calendar: rule.TargetCalendarID,
				OAuth: config.OAuth{
					ClientID:  r.googleClientID,
					ClientKey: r.googleClientSecret,
				},
			},
		},
		Sync: config.Sync{
			StartTime: rule.StartTime,
			EndTime:   rule.EndTime,
		},
		Filters:           rule.Filters,
		Transformations:   payloadTransformations(rule),
		UpdateConcurrency: rule.UpdateConcurrency,
	}

	start, err := timeFromConfig(cfg.Sync.StartTime)
	if err != nil {
		return err
	}
	end, err := timeFromConfig(cfg.Sync.EndTime)
	if err != nil {
		return err
	}

	sourceAdapter, err := adapter.NewSourceAdapterFromConfig(ctx, 0, false, config.NewAdapterConfig(cfg.Source.Adapter), storage, log.Default())
	if err != nil {
		return err
	}
	sinkAdapter, err := adapter.NewSinkAdapterFromConfig(ctx, 0, false, config.NewAdapterConfig(cfg.Sink.Adapter), storage, log.Default())
	if err != nil {
		return err
	}
	controller := sync.NewController(log.Default(), sourceAdapter, sinkAdapter, sync.TransformerFactory(cfg.Transformations), sync.FilterFactory(cfg.Filters))
	if cfg.UpdateConcurrency > 0 {
		controller.SetConcurrency(cfg.UpdateConcurrency)
	}

	if run.TriggerType == "cleanup" {
		if err := controller.CleanUp(ctx, start, end); err != nil {
			return err
		}
	} else {
		if err := controller.SynchroniseTimeframe(ctx, start, end, rule.DryRun); err != nil {
			return err
		}
	}

	return r.store.MarkRunDone(ctx, run.ID, map[string]int{"ok": 1})
}

func timeFromConfig(st config.SyncTime) (time.Time, error) {
	now := time.Now().UTC()
	switch st.Identifier {
	case "MonthStart":
		t := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return t.AddDate(0, st.Offset, 0), nil
	case "MonthEnd":
		t := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, time.UTC)
		return t.AddDate(0, st.Offset, 0), nil
	case "TodayStart":
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		return t.AddDate(0, 0, st.Offset), nil
	case "TodayEnd":
		t := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)
		return t.AddDate(0, 0, st.Offset), nil
	case "Now":
		return now.Add(time.Duration(st.Offset) * time.Hour), nil
	case "YearEnd":
		t := time.Date(now.Year(), 12, 31, 23, 59, 59, 0, time.UTC)
		return t.AddDate(st.Offset, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported sync identifier: %s", st.Identifier)
	}
}

func payloadTransformations(rule *store.SyncRule) []config.Transformer {
	if rule.PayloadMode == "busy" {
		return []config.Transformer{
			{Name: "ReplaceTitle", Config: config.CustomMap{"NewTitle": "Busy"}},
		}
	}
	return rule.Transformations
}

func storageLookup(rule *store.SyncRule, calendarID string) (uuid.UUID, bool) {
	if calendarID == rule.SourceCalendarID {
		return rule.SourceConnectionID, true
	}
	if calendarID == rule.TargetCalendarID {
		return rule.TargetConnectionID, true
	}
	return uuid.Nil, false
}
