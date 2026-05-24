package likeable

import (
	"context"
	"strconv"
	"strings"
	"time"
)

const (
	defaultFreeHourWindowHours = 5
	maxFreeHourWindowHours     = 24
	maxPromptImproveChargeMin  = 60
	freeHourWindow             = time.Duration(defaultFreeHourWindowHours) * time.Hour
	msPerHour                  = int64(time.Hour / time.Millisecond)
)

func (s *Server) canSendMessage(ctx context.Context, user *User) bool {
	allowed, err := s.hourAllowance(ctx, user)
	return err == nil && allowed
}

func (s *Server) hourQuota(ctx context.Context, user *User) map[string]any {
	limitMs := s.freeHourLimitMs(ctx)
	windowHours := s.freeHourWindowHours(ctx)
	now := time.Now().UTC()
	windowStart, resetAt := fixedUTCHourWindow(now, time.Duration(windowHours)*time.Hour)
	usedMs, err := s.store.UserWorkMsBetween(ctx, user.ID, windowStart, resetAt, now)
	if err != nil {
		usedMs = 0
	}
	lifetimeUsedMs, err := s.store.UserLifetimeWorkMs(ctx, user.ID, now)
	if err != nil {
		lifetimeUsedMs = 0
	}
	paidRemainingMs, err := s.store.PaidHourCreditBalance(ctx, user.ID)
	if err != nil {
		paidRemainingMs = 0
	}
	remainingMs := limitMs - usedMs
	if remainingMs < 0 {
		remainingMs = 0
	}
	return map[string]any{
		"usedMs":          usedMs,
		"limitMs":         limitMs,
		"remainingMs":     remainingMs,
		"paidRemainingMs": paidRemainingMs,
		"lifetimeUsedMs":  lifetimeUsedMs,
		"resetsAt":        resetAt.Format(time.RFC3339),
		"windowHours":     windowHours,
	}
}

func (s *Server) hourAllowance(ctx context.Context, user *User) (bool, error) {
	if user == nil {
		return false, nil
	}
	paidRemainingMs, err := s.store.PaidHourCreditBalance(ctx, user.ID)
	if err != nil {
		return false, err
	}
	if paidRemainingMs < 0 {
		return false, nil
	}
	limitMs := s.freeHourLimitMs(ctx)
	now := time.Now().UTC()
	windowStart, resetAt := s.freeHourWindow(now, ctx)
	usedMs, err := s.store.UserWorkMsBetween(ctx, user.ID, windowStart, resetAt, now)
	if err != nil {
		return false, err
	}
	if usedMs < limitMs {
		return true, nil
	}
	return paidRemainingMs > 0, nil
}

func (s *Server) freeHourWindow(now time.Time, ctx context.Context) (time.Time, time.Time) {
	return fixedUTCHourWindow(now, time.Duration(s.freeHourWindowHours(ctx))*time.Hour)
}

func (s *Server) currentFreeHourWindowStart(ctx context.Context) time.Time {
	start, _ := s.freeHourWindow(time.Now(), ctx)
	return start
}

func fixedUTCHourWindow(now time.Time, interval time.Duration) (time.Time, time.Time) {
	if interval <= 0 {
		interval = freeHourWindow
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

func (s *Server) freeHourLimit(ctx context.Context) int {
	cfg, _ := s.store.ConfigMap(ctx)
	raw := strings.TrimSpace(cfg["free_hours"])
	if raw == "" {
		raw = "5"
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 5
	}
	return n
}

func (s *Server) freeHourLimitMs(ctx context.Context) int64 {
	return int64(s.freeHourLimit(ctx)) * msPerHour
}

func (s *Server) freeHourWindowHours(ctx context.Context) int {
	cfg, _ := s.store.ConfigMap(ctx)
	raw := strings.TrimSpace(cfg["free_hour_window_hours"])
	if raw == "" {
		return defaultFreeHourWindowHours
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 || n > maxFreeHourWindowHours {
		return defaultFreeHourWindowHours
	}
	return n
}

func (s *Server) promptImproveChargeMinutes(ctx context.Context) int {
	cfg, _ := s.store.ConfigMap(ctx)
	raw := strings.TrimSpace(cfg["prompt_improve_charge_minutes"])
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	if n > maxPromptImproveChargeMin {
		return maxPromptImproveChargeMin
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
