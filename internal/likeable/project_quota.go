package likeable

import (
	"context"
	"strconv"
	"strings"
	"time"
)

const freeMessageWindow = 5 * time.Hour

func (s *Server) canSendMessage(ctx context.Context, user *User) bool {
	allowed, _, err := s.messageAllowance(ctx, user)
	return err == nil && allowed
}

func (s *Server) messageQuota(ctx context.Context, user *User) map[string]any {
	limit := s.freeMessageLimit(ctx)
	windowStart := messageAllowanceWindowStart(time.Now())
	used, oldestAt, err := s.store.UserMessageWindow(ctx, user.ID, windowStart)
	if err != nil {
		used = 0
		oldestAt = ""
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
	resetAt := nextFreeMessageResetAt(oldestAt, time.Now()).Format(time.RFC3339)
	return map[string]any{
		"used":          used,
		"limit":         limit,
		"remaining":     remaining,
		"paidRemaining": paidRemaining,
		"lifetimeUsed":  lifetimeUsed,
		"resetsAt":      resetAt,
	}
}

func (s *Server) messageAllowance(ctx context.Context, user *User) (bool, bool, error) {
	limit := s.freeMessageLimit(ctx)
	used, _, err := s.store.UserMessageWindow(ctx, user.ID, messageAllowanceWindowStart(time.Now()))
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

func messageAllowanceWindowStart(now time.Time) time.Time {
	return now.UTC().Add(-freeMessageWindow)
}

func nextFreeMessageResetAt(oldestAt string, now time.Time) time.Time {
	now = now.UTC()
	oldest, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(oldestAt))
	if err != nil || oldest.IsZero() {
		return now
	}
	resetAt := oldest.UTC().Add(freeMessageWindow)
	if resetAt.Before(now) {
		return now
	}
	return resetAt
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
