package worker

import (
	"context"
	"fmt"
	"sync/atomic"
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
	lockLease          time.Duration
}

func NewRunner(st *store.PostgresStore, tokenCodec *store.TokenCodec, googleClientID, googleClientSecret string) *Runner {
	return &Runner{
		store:              st,
		tokenCodec:         tokenCodec,
		googleClientID:     googleClientID,
		googleClientSecret: googleClientSecret,
		lockLease:          10 * time.Minute,
	}
}

func (r *Runner) RunOnce(ctx context.Context) (bool, error) {
	recovered, err := r.store.FailStaleRuns(ctx, r.lockLease)
	if err != nil {
		return false, err
	}
	if recovered > 0 {
		log.Warn("recovered stale runs", "count", recovered)
	}

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

	locked, err := r.store.AcquireRuleLock(ctx, rule.ID, run.ID, r.lockLease)
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("rule is already running")
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	var lockLost atomic.Bool
	stopHeartbeat := r.startLockHeartbeat(runCtx, rule.ID, run.ID, &lockLost, cancelRun)
	defer stopHeartbeat()

	defer func() {
		if releaseErr := r.store.ReleaseRuleLock(context.Background(), rule.ID, run.ID); releaseErr != nil {
			log.Error("failed to release rule lock", "rule_id", rule.ID, "err", releaseErr)
		}
	}()

	sourceTokenRaw, err := r.store.GetEncryptedToken(runCtx, rule.SourceConnectionID)
	if err != nil {
		return err
	}
	if rewritten, err := r.tokenCodec.ReencryptIfLegacy(runCtx, sourceTokenRaw); err != nil {
		return err
	} else if rewritten != nil {
		if err := r.store.StoreEncryptedToken(runCtx, *rewritten); err != nil {
			return err
		}
		sourceTokenRaw = rewritten
	}
	targetTokenRaw, err := r.store.GetEncryptedToken(runCtx, rule.TargetConnectionID)
	if err != nil {
		return err
	}
	if rewritten, err := r.tokenCodec.ReencryptIfLegacy(runCtx, targetTokenRaw); err != nil {
		return err
	} else if rewritten != nil {
		if err := r.store.StoreEncryptedToken(runCtx, *rewritten); err != nil {
			return err
		}
		targetTokenRaw = rewritten
	}
	sourceToken, err := r.tokenCodec.DecryptToken(runCtx, sourceTokenRaw)
	if err != nil {
		return err
	}
	targetToken, err := r.tokenCodec.DecryptToken(runCtx, targetTokenRaw)
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
		encrypted, err := r.tokenCodec.EncryptToken(runCtx, binding, "google", newToken)
		if err != nil {
			return err
		}
		return r.store.StoreEncryptedToken(runCtx, *encrypted)
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
	cleanupStart, cleanupEnd := cleanupWindow()

	sourceAdapter, err := adapter.NewSourceAdapterFromConfig(runCtx, 0, false, config.NewAdapterConfig(cfg.Source.Adapter), storage, log.Default())
	if err != nil {
		return err
	}
	sinkAdapter, err := adapter.NewSinkAdapterFromConfig(runCtx, 0, false, config.NewAdapterConfig(cfg.Sink.Adapter), storage, log.Default())
	if err != nil {
		return err
	}
	controller := sync.NewController(log.Default(), sourceAdapter, sinkAdapter, sync.TransformerFactory(cfg.Transformations), sync.FilterFactory(cfg.Filters))
	if cfg.UpdateConcurrency > 0 {
		controller.SetConcurrency(cfg.UpdateConcurrency)
	}

	if run.TriggerType == "cleanup" {
		// Hosted cleanup should remove all previously managed events for this rule,
		// not only events inside the current sync window.
		if err := controller.CleanUp(runCtx, cleanupStart, cleanupEnd); err != nil {
			return err
		}
	} else {
		if err := controller.SynchroniseTimeframe(runCtx, start, end, rule.DryRun); err != nil {
			return err
		}
	}

	if lockLost.Load() {
		return fmt.Errorf("run lock lease lost while executing rule")
	}

	return r.store.MarkRunDone(runCtx, run.ID, map[string]int{"ok": 1})
}

func (r *Runner) SetLockLease(lease time.Duration) {
	if lease > 0 {
		r.lockLease = lease
	}
}

func (r *Runner) startLockHeartbeat(
	ctx context.Context,
	ruleID uuid.UUID,
	runID uuid.UUID,
	lockLost *atomic.Bool,
	cancelRun context.CancelFunc,
) func() {
	interval := r.lockLease / 3
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				renewed, err := r.store.RenewRuleLock(ctx, ruleID, runID)
				if err != nil {
					log.Error("failed to renew rule lock", "rule_id", ruleID, "run_id", runID, "err", err)
					lockLost.Store(true)
					cancelRun()
					return
				}
				if !renewed {
					log.Error("rule lock no longer owned by current run", "rule_id", ruleID, "run_id", runID)
					lockLost.Store(true)
					cancelRun()
					return
				}
			}
		}
	}()

	return func() {
		ticker.Stop()
		<-done
	}
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

func cleanupWindow() (time.Time, time.Time) {
	return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
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
