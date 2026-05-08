package likeable

import "github.com/fibegg/likeable/internal/domain"

type User = domain.User
type Project = domain.Project
type Message = domain.Message
type MessageAttachment = domain.MessageAttachment
type SocialConnection = domain.SocialConnection
type Subscription = domain.Subscription
type Payment = domain.Payment
type ProjectQuotaGrant = domain.ProjectQuotaGrant
type ProjectArchive = domain.ProjectArchive
type UserNotice = domain.UserNotice
type AdminUserSummary = domain.AdminUserSummary
type AdminProjectSummary = domain.AdminProjectSummary
type AdminUserDetail = domain.AdminUserDetail
type AdminUserFilters = domain.AdminUserFilters

type RuntimeConfig struct {
	Addr         string
	BaseURL      string
	DatabasePath string
	AdminEmail   string
	RedisURL     string
	DevAuth      bool
	WebDir       string
}

func normalizeEmail(email string) string {
	return domain.NormalizeEmail(email)
}
