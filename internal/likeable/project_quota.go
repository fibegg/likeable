package likeable

import (
	"context"
	"strconv"
	"strings"
	"time"
)

const (
	defaultFreeMessageWindowHours = 5
	maxFreeMessageWindowHours     = 24
	freeMessageWindow             = time.Duration(defaultFreeMessageWindowHours) * time.Hour
)

func (s *Server) canSendMessage(ctx context.Context, user *User) bool {
	allowed, _, err := s.messageAllowance(ctx, user)
	return err == nil && allowed
}

func (s *Server) messageQuota(ctx context.Context, user *User) map[string]any {
	limit := s.freeMessageLimit(ctx)
	windowHours := s.freeMessageWindowHours(ctx)
	windowStart, resetAt := fixedUTCMessageWindow(time.Now(), time.Duration(windowHours)*time.Hour)
	used, _, err := s.store.UserMessageWindow(ctx, user.ID, windowStart)
	if err != nil {
		used = 0
	}
	lifetimeUsed, err := s.store.UserMessageCount(ctx, user.ID)
	if err != nil {
		lifetimeUsed = 0
	}
	paidRemaining, err := s.store.PaidMessageCreditBalance(ctx, user.ID)
	if err != nil {
		paidRemaining = 0
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return map[string]any{
		"used":          used,
		"limit":         limit,
		"remaining":     remaining,
		"paidRemaining": paidRemaining,
		"lifetimeUsed":  lifetimeUsed,
		"resetsAt":      resetAt.Format(time.RFC3339),
		"windowHours":   windowHours,
	}
}

func (s *Server) messageAllowance(ctx context.Context, user *User) (bool, bool, error) {
	limit := s.freeMessageLimit(ctx)
	windowStart, _ := s.freeMessageWindow(time.Now(), ctx)
	used, _, err := s.store.UserMessageWindow(ctx, user.ID, windowStart)
	if err != nil {
		return false, false, err
	}
	if used < limit {
		return true, false, nil
	}
	paidRemaining, err := s.store.PaidMessageCreditBalance(ctx, user.ID)
	if err != nil {
		return false, false, err
	}
	return paidRemaining > 0, true, nil
}

func (s *Server) freeMessageWindow(now time.Time, ctx context.Context) (time.Time, time.Time) {
	return fixedUTCMessageWindow(now, time.Duration(s.freeMessageWindowHours(ctx))*time.Hour)
}

func (s *Server) currentFreeMessageWindowStart(ctx context.Context) time.Time {
	start, _ := s.freeMessageWindow(time.Now(), ctx)
	return start
}

func fixedUTCMessageWindow(now time.Time, interval time.Duration) (time.Time, time.Time) {
	if interval <= 0 {
		interval = freeMessageWindow
	}
	if interval > 24*time.Hour {
		interval = 24 * time.Hour
	}
	now = now.UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	elapsed := now.Sub(dayStart)
	bucket := int64(elapsed / interval)
	start := dayStart.Add(time.Duration(bucket) * interval)
	end := start.Add(interval)
	nextDay := dayStart.Add(24 * time.Hour)
	if end.After(nextDay) {
		end = nextDay
	}
	return start, end
}

func (s *Server) freeMessageLimit(ctx context.Context) int {
	cfg, _ := s.store.ConfigMap(ctx)
	raw := strings.TrimSpace(cfg["free_messages"])
	if raw == "" {
		raw = "5"
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 5
	}
	return n
}

func (s *Server) freeMessageWindowHours(ctx context.Context) int {
	cfg, _ := s.store.ConfigMap(ctx)
	raw := strings.TrimSpace(cfg["free_message_window_hours"])
	if raw == "" {
		return defaultFreeMessageWindowHours
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 || n > maxFreeMessageWindowHours {
		return defaultFreeMessageWindowHours
	}
	return n
}

func (s *Server) projectCap(ctx context.Context) int {
	return s.baseProjectCap(ctx)
}

func (s *Server) baseProjectCap(ctx context.Context) int {
	cfg, _ := s.store.ConfigMap(ctx)
	raw := firstNonEmpty(cfg["project_cap"], "3")
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 3
	}
	if n < 0 {
		return 0
	}
	return n
}

func (s *Server) projectCapForUser(ctx context.Context, user *User) int {
	cap := s.baseProjectCap(ctx)
	if user == nil {
		return cap
	}
	paidSlots, _, err := s.store.ActiveProjectQuota(ctx, user.ID)
	if err != nil {
		return cap
	}
	return cap + paidSlots
}

func (s *Server) projectQuota(ctx context.Context, user *User) map[string]any {
	base := s.baseProjectCap(ctx)
	paidSlots := 0
	nextExpiresAt := ""
	if user != nil {
		slots, expiresAt, err := s.store.ActiveProjectQuota(ctx, user.ID)
		if err == nil {
			paidSlots = slots
			nextExpiresAt = expiresAt
		}
	}
	used := 0
	if user != nil {
		if count, err := s.store.ProjectCountForUser(ctx, user.ID); err == nil {
			used = count
		}
	}
	limit := base + paidSlots
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return map[string]any{
		"baseLimit":     base,
		"paidSlots":     paidSlots,
		"limit":         limit,
		"used":          used,
		"remaining":     remaining,
		"nextExpiresAt": nextExpiresAt,
	}
}
