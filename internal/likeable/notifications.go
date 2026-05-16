package likeable

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

func (s *Server) addSystemNoticeAndEmail(ctx context.Context, user *User, severity, body, subject, emailBody string) {
	if user == nil {
		return
	}
	notice, err := s.store.AddUserNotice(ctx, UserNotice{UserID: user.ID, Sender: "system", Severity: severity, Body: body})
	if err != nil {
		log.Printf("add system notice for %s: %v", user.Email, err)
		return
	}
	if subject != "" && emailBody != "" {
		s.sendUserEmailAsync(user.Email, subject, emailBody)
	}
	_ = notice
}

func (s *Server) notifyMessageQuotaIfNeeded(ctx context.Context, user *User) {
	if user == nil {
		return
	}
	limit := s.freeMessageLimit(ctx)
	if limit <= 0 {
		return
	}
	windowHours := s.freeMessageWindowHours(ctx)
	windowStart, _ := s.freeMessageWindow(time.Now(), ctx)
	used, _, err := s.store.UserMessageWindow(ctx, user.ID, windowStart)
	if err != nil {
		log.Printf("message quota notice count for %s: %v", user.Email, err)
		return
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	threshold := limit / 5
	if threshold < 1 {
		threshold = 1
	}
	if threshold > 5 {
		threshold = 5
	}
	paidRemaining, _ := s.store.PaidMessageCreditBalance(ctx, user.ID)
	if remaining <= threshold {
		prefix := "Message quota:"
		exists, err := s.store.NoticeExistsSince(ctx, user.ID, "system", prefix, windowStart)
		if err == nil && !exists {
			body := fmt.Sprintf("%s You have %d/%d free messages remaining in this %d-hour window.", prefix, remaining, limit, windowHours)
			if paidRemaining > 0 {
				body += fmt.Sprintf(" Your %d paid credits are used only after free messages are spent.", paidRemaining)
			} else {
				body += " Buy a message pack if you need more before the next reset."
			}
			s.addSystemNoticeAndEmail(ctx, user, "warning", body, "Likeable message quota running low", body+"\n\nManage credits:\n"+s.profileURL())
		}
	}
	if remaining == 0 && paidRemaining > 0 && paidRemaining <= 3 {
		prefix := "Paid credits:"
		exists, err := s.store.NoticeExistsSince(ctx, user.ID, "system", prefix, windowStart)
		if err == nil && !exists {
			body := fmt.Sprintf("%s You have %d paid message credits left.", prefix, paidRemaining)
			s.addSystemNoticeAndEmail(ctx, user, "warning", body, "Likeable paid credits running low", body+"\n\nBuy more credits:\n"+s.profileURL())
		}
	}
}

func (s *Server) notifyProjectQuotaIfNeeded(ctx context.Context, user *User) {
	if user == nil {
		return
	}
	cap := s.projectCapForUser(ctx, user)
	if cap <= 0 {
		return
	}
	count, err := s.store.ProjectCountForUser(ctx, user.ID)
	if err != nil {
		log.Printf("project quota notice count for %s: %v", user.Email, err)
		return
	}
	remaining := cap - count
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 1 {
		return
	}
	prefix := fmt.Sprintf("Project quota: You are using %d/%d", count, cap)
	exists, err := s.store.NoticeExistsSince(ctx, user.ID, "system", prefix, time.Now().UTC().Add(-7*24*time.Hour))
	if err != nil || exists {
		return
	}
	body := prefix + " project slots."
	if remaining == 0 {
		body += " Delete or export older projects before creating another one."
	} else {
		body += " You have one project slot left."
	}
	s.addSystemNoticeAndEmail(ctx, user, "warning", body, "Likeable project quota running low", body+"\n\nManage projects:\n"+s.profileURL())
}

func (s *Server) notifyMessageCreditsPurchased(ctx context.Context, userID string, credits int) {
	if credits <= 0 {
		return
	}
	user, err := s.store.UserByID(ctx, userID)
	if err != nil {
		log.Printf("load user for purchase notice %s: %v", userID, err)
		return
	}
	body := fmt.Sprintf("Message credits purchased: %d paid message credits were added to your Likeable account.", credits)
	s.addSystemNoticeAndEmail(ctx, user, "info", body, "Likeable message credits added", body+"\n\nView your balance:\n"+s.profileURL())
}

func (s *Server) notifyProjectQuotaPurchased(ctx context.Context, userID string, slots int, expiresAt time.Time) {
	if slots <= 0 {
		return
	}
	user, err := s.store.UserByID(ctx, userID)
	if err != nil {
		log.Printf("load user for project quota purchase notice %s: %v", userID, err)
		return
	}
	body := fmt.Sprintf("Project quota purchased: %d extra project slot is active until %s.", slots, expiresAt.UTC().Format("2006-01-02 15:04 UTC"))
	if slots != 1 {
		body = fmt.Sprintf("Project quota purchased: %d extra project slots are active until %s.", slots, expiresAt.UTC().Format("2006-01-02 15:04 UTC"))
	}
	s.addSystemNoticeAndEmail(ctx, user, "info", body, "Likeable project quota added", body+"\n\nManage projects:\n"+s.profileURL())
}

func (s *Server) notifyProjectExportReady(ctx context.Context, user *User, project *Project, repoURL string) {
	if user == nil || project == nil || strings.TrimSpace(repoURL) == "" {
		return
	}
	body := fmt.Sprintf("Project export ready: %q has been exported to GitHub.\n\n%s", project.Title, repoURL)
	s.addSystemNoticeAndEmail(ctx, user, "info", body, "Likeable project export ready", body)
}

func (s *Server) notifyProjectArchiveReady(ctx context.Context, user *User, projectTitle, downloadURL string, expiresAt time.Time) {
	if user == nil || strings.TrimSpace(downloadURL) == "" {
		return
	}
	body := fmt.Sprintf("Project archive ready: %q is ready to download.\n\n%s", projectTitle, downloadURL)
	if !expiresAt.IsZero() {
		body += "\n\nThis archive is scheduled to expire on " + expiresAt.UTC().Format("2006-01-02 15:04 UTC") + "."
	}
	s.addSystemNoticeAndEmail(ctx, user, "warning", body, "Likeable project archive ready", body)
}

func (s *Server) notifyProjectDeletionScheduled(ctx context.Context, user *User, project *Project) {
	if user == nil || project == nil {
		return
	}
	body := fmt.Sprintf("Project deletion started: %q and its workspace resources are being removed.", project.Title)
	s.addSystemNoticeAndEmail(ctx, user, "warning", body, "Likeable project deletion started", body)
}

func (s *Server) profileURL() string {
	return strings.TrimRight(s.config.BaseURL, "/") + "/profile"
}
