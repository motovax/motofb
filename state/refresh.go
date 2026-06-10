package state

import (
	"context"
	"errors"
	"time"

	fberr "github.com/motovax/motofb/errors"
	"github.com/motovax/motofb/graphql"
)

func isRetryableAPIError(err error) bool {
	var e *fberr.Error
	if errors.As(err, &e) {
		if e.Code == graphql.ErrNotLoggedIn || e.Code == graphql.ErrRefreshCookies {
			return true
		}
	}
	return errors.Is(err, fberr.ErrSessionExpired) || errors.Is(err, fberr.ErrFacebookAPI)
}

func (s *State) withRetry(ctx context.Context, retries int, fn func() (any, error)) (any, error) {
	out, err := fn()
	for retries > 0 && isRetryableAPIError(err) {
		if refreshErr := s.Refresh(ctx); refreshErr != nil {
			return out, err
		}
		retries--
		out, err = fn()
	}
	return out, err
}

// EnableAutoRefresh starts a background token refresh loop (default interval 1h, check every 5m).
func (s *State) EnableAutoRefresh(interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	s.autoRefreshMu.Lock()
	defer s.autoRefreshMu.Unlock()
	if s.autoRefreshEnabled {
		return
	}
	s.autoRefreshEnabled = true
	s.refreshInterval = interval
	s.autoRefreshStop = make(chan struct{})
	go s.autoRefreshLoop()
}

// DisableAutoRefresh stops the background refresh loop.
func (s *State) DisableAutoRefresh() {
	s.autoRefreshMu.Lock()
	defer s.autoRefreshMu.Unlock()
	if !s.autoRefreshEnabled {
		return
	}
	s.autoRefreshEnabled = false
	close(s.autoRefreshStop)
	s.autoRefreshStop = nil
}

func (s *State) autoRefreshLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	lastRefresh := time.Now()
	for {
		select {
		case <-s.autoRefreshStop:
			return
		case <-ticker.C:
			s.autoRefreshMu.Lock()
			enabled := s.autoRefreshEnabled
			interval := s.refreshInterval
			s.autoRefreshMu.Unlock()
			if !enabled {
				return
			}
			if time.Since(lastRefresh) >= interval {
				ctx := context.Background()
				if err := s.Refresh(ctx); err == nil {
					lastRefresh = time.Now()
				}
			}
		}
	}
}