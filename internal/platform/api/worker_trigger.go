package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"google.golang.org/api/idtoken"
)

func (s *Server) triggerWorkerRun(ctx context.Context) error {
	if s.workerRunURL == "" {
		return nil
	}

	httpClient := s.workerHTTP
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	authFactory := s.workerAuth
	if authFactory == nil {
		authFactory = idtoken.NewClient
	}
	audience := workerAudience(s.workerRunURL)

	authClient, err := authFactory(ctx, audience)
	if err != nil {
		return fmt.Errorf("create idtoken client: %w", err)
	}
	authClient.Timeout = httpClient.Timeout

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.workerRunURL, nil)
	if err != nil {
		return fmt.Errorf("build worker request: %w", err)
	}
	req.Header.Set("X-Scheduler-Secret", s.schedulerSecret)

	resp, err := authClient.Do(req)
	if err != nil {
		return fmt.Errorf("trigger worker run: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if len(body) == 0 {
			return fmt.Errorf("worker run trigger returned %s", resp.Status)
		}
		return fmt.Errorf("worker run trigger returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}

func workerAudience(workerRunURL string) string {
	parsed, err := url.Parse(workerRunURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return workerRunURL
	}
	return parsed.Scheme + "://" + parsed.Host
}
