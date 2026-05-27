package domain

import (
	"strings"
	"time"
)

const PlaygroundIdleStopAfter = 8 * time.Hour

const (
	ProjectDomainStatusPendingDNS = "pending_dns"
	ProjectDomainStatusActive     = "active"
)

type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	AvatarURL    string `json:"avatarUrl"`
	AccessStatus string `json:"accessStatus"`
	AccessNote   string `json:"accessNote,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

type Project struct {
	ID                    string              `json:"id"`
	UserID                string              `json:"-"`
	Title                 string              `json:"title"`
	ConversationID        string              `json:"-"`
	AgentID               string              `json:"-"`
	MarqueeID             string              `json:"-"`
	PlaygroundID          string              `json:"-"`
	PlaygroundName        string              `json:"-"`
	PlayspecID            string              `json:"-"`
	PropID                string              `json:"-"`
	RepoURL               string              `json:"-"`
	PreviewURL            string              `json:"previewUrl,omitempty"`
	SelectedService       string              `json:"selectedServiceName,omitempty"`
	Repositories          []ProjectRepository `json:"repositories,omitempty"`
	Services              []ProjectService    `json:"services,omitempty"`
	Status                string              `json:"status"`
	ErrorMessage          string              `json:"errorMessage,omitempty"`
	ProvisioningLockUntil string              `json:"-"`
	CleanupLastError      string              `json:"-"`
	PlaygroundLastUsedAt  string              `json:"playgroundLastUsedAt,omitempty"`
	PlaygroundIdleStopAt  string              `json:"playgroundIdleStopAt,omitempty"`
	ProductionExpiresAt   string              `json:"productionExpiresAt,omitempty"`
	CustomDomain          string              `json:"customDomain,omitempty"`
	CustomDomainStatus    string              `json:"customDomainStatus,omitempty"`
	CustomDomainTarget    string              `json:"customDomainTarget,omitempty"`
	CustomDomainUpdatedAt string              `json:"customDomainUpdatedAt,omitempty"`
	CreatedAt             string              `json:"createdAt"`
	UpdatedAt             string              `json:"updatedAt"`
}

type ProjectDomain struct {
	ProjectID string
	UserID    string
	Domain    string
	Target    string
	Status    string
	UpdatedAt string
}

func (p *Project) RefreshComputedFields() {
	p.PlaygroundIdleStopAt = ""
	if strings.TrimSpace(p.ProductionExpiresAt) != "" {
		return
	}
	if p.Status != "ready" || strings.TrimSpace(p.PlaygroundLastUsedAt) == "" {
		return
	}
	lastUsedAt, err := time.Parse(time.RFC3339Nano, p.PlaygroundLastUsedAt)
	if err != nil {
		return
	}
	p.PlaygroundIdleStopAt = lastUsedAt.UTC().Add(PlaygroundIdleStopAfter).Format(time.RFC3339Nano)
}

type ProjectRepository struct {
	ID            string   `json:"id"`
	ProjectID     string   `json:"projectId,omitempty"`
	Role          string   `json:"role"`
	PropID        string   `json:"-"`
	RepoURL       string   `json:"-"`
	SourceRepoURL string   `json:"sourceRepoUrl,omitempty"`
	Provider      string   `json:"provider,omitempty"`
	ServiceNames  []string `json:"serviceNames,omitempty"`
	CreatedAt     string   `json:"createdAt,omitempty"`
}

type ProjectService struct {
	ID           string `json:"id"`
	ProjectID    string `json:"projectId,omitempty"`
	Name         string `json:"name"`
	URL          string `json:"url"`
	Type         string `json:"type,omitempty"`
	Visibility   string `json:"visibility,omitempty"`
	AuthRequired bool   `json:"authRequired"`
	CreatedAt    string `json:"createdAt,omitempty"`
}

type Message struct {
	ID          string              `json:"id"`
	ProjectID   string              `json:"projectId"`
	Role        string              `json:"role"`
	Body        string              `json:"body"`
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	CreatedAt   string              `json:"createdAt"`
}

type MessageAttachment struct {
	ID          string `json:"id"`
	MessageID   string `json:"messageId,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	URL         string `json:"url,omitempty"`
	StoragePath string `json:"-"`
	CreatedAt   string `json:"createdAt,omitempty"`
}

type ProjectNotificationTiming struct {
	ProjectID      string `json:"-"`
	NotificationID string `json:"-"`
	Body           string `json:"body,omitempty"`
	StartedAt      string `json:"startedAt,omitempty"`
	CompletedAt    string `json:"completedAt,omitempty"`
	ElapsedMs      int64  `json:"elapsedMs,omitempty"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
}

type ProjectWorkSession struct {
	ProjectID    string `json:"projectId"`
	UserID       string `json:"userId"`
	SessionKey   string `json:"sessionKey"`
	StartedAt    string `json:"startedAt"`
	CompletedAt  string `json:"completedAt,omitempty"`
	ElapsedMs    int64  `json:"elapsedMs"`
	FreeBilledMs int64  `json:"freeBilledMs"`
	PaidBilledMs int64  `json:"paidBilledMs"`
	BilledAt     string `json:"billedAt,omitempty"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type SocialConnection struct {
	ID             string
	UserID         string
	Provider       string
	ProviderUserID string
	AccessToken    string
	Scope          string
}

type Subscription struct {
	UserID               string
	Status               string
	StripeCustomerID     string
	StripeSubscriptionID string
	CurrentPeriodEnd     time.Time
}

type Payment struct {
	ID                string `json:"id"`
	UserID            string `json:"userId"`
	ProviderPaymentID string `json:"providerPaymentId"`
	AmountCents       int64  `json:"amountCents"`
	Currency          string `json:"currency"`
	Status            string `json:"status"`
	CreatedAt         string `json:"createdAt"`
}

type ProjectQuotaGrant struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	PaymentID string `json:"paymentId"`
	Slots     int    `json:"slots"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
}

type ProjectArchive struct {
	ID            string `json:"id"`
	UserID        string `json:"userId"`
	ProjectID     string `json:"projectId"`
	ProjectTitle  string `json:"projectTitle"`
	StoragePath   string `json:"-"`
	DownloadURL   string `json:"downloadUrl,omitempty"`
	GithubRepoURL string `json:"githubRepoUrl,omitempty"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
	ExpiresAt     string `json:"expiresAt"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type AgentPoolStat struct {
	AgentID            string `json:"agentId"`
	ServerID           string `json:"serverId"`
	ProjectCount       int    `json:"projectCount"`
	ActiveProjectCount int    `json:"activeProjectCount"`
	ArchivedCount      int    `json:"archivedCount"`
	ReadyArchiveCount  int    `json:"readyArchiveCount"`
}

type AgentAssignmentSummary struct {
	AgentID      string `json:"agentId"`
	ServerID     string `json:"serverId"`
	Status       string `json:"status,omitempty"`
	ProjectCount int    `json:"projectCount,omitempty"`
}

type AgentPoolOption struct {
	Label    string `json:"label,omitempty"`
	AgentID  string `json:"agentId"`
	ServerID string `json:"serverId"`
	Status   string `json:"status"`
	Capacity int    `json:"capacity,omitempty"`
}

type UserNotice struct {
	ID          string `json:"id"`
	UserID      string `json:"userId,omitempty"`
	Sender      string `json:"sender"`
	Severity    string `json:"severity"`
	Body        string `json:"body"`
	ReadAt      string `json:"readAt,omitempty"`
	DismissedAt string `json:"dismissedAt,omitempty"`
	UnsentAt    string `json:"unsentAt,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

type AdminUserSummary struct {
	User               User                     `json:"user"`
	ProjectCount       int                      `json:"projectCount"`
	ProjectLimit       int                      `json:"projectLimit"`
	PaidProjectSlots   int                      `json:"paidProjectSlots"`
	ProjectSlotsExpire string                   `json:"projectSlotsExpire,omitempty"`
	LifetimeWorkMs     int64                    `json:"lifetimeWorkMs"`
	WindowWorkMs       int64                    `json:"windowWorkMs"`
	FreeHourLimitMs    int64                    `json:"freeHourLimitMs"`
	PaidHourBalanceMs  int64                    `json:"paidHourBalanceMs"`
	GithubConnected    bool                     `json:"githubConnected"`
	SubscriptionStatus string                   `json:"subscriptionStatus"`
	PaidTotalCents     int64                    `json:"paidTotalCents"`
	PaidCurrency       string                   `json:"paidCurrency"`
	LastMessageAt      string                   `json:"lastMessageAt,omitempty"`
	LastProjectAt      string                   `json:"lastProjectAt,omitempty"`
	LatestNotice       *UserNotice              `json:"latestNotice,omitempty"`
	AgentPairs         []AgentAssignmentSummary `json:"agentPairs,omitempty"`
}

type AdminProjectSummary struct {
	Project    Project                `json:"project"`
	WorkMs     int64                  `json:"workMs"`
	Assignment AgentAssignmentSummary `json:"assignment"`
}

type AdminBillingPayment struct {
	ID                string `json:"id"`
	UserID            string `json:"userId"`
	UserEmail         string `json:"userEmail"`
	ProviderPaymentID string `json:"providerPaymentId"`
	AmountCents       int64  `json:"amountCents"`
	Currency          string `json:"currency"`
	Status            string `json:"status"`
	CreatedAt         string `json:"createdAt"`
}

type AdminHourCreditLedgerEntry struct {
	ID             string `json:"id"`
	UserID         string `json:"userId"`
	DeltaMs        int64  `json:"deltaMs"`
	Reason         string `json:"reason"`
	PaymentID      string `json:"paymentId,omitempty"`
	WorkSessionKey string `json:"workSessionKey,omitempty"`
	CreatedAt      string `json:"createdAt"`
}

type AdminProjectInternal struct {
	UserID                string `json:"userId"`
	ConversationID        string `json:"conversationId,omitempty"`
	AgentID               string `json:"agentId,omitempty"`
	ServerID              string `json:"serverId,omitempty"`
	PlaygroundID          string `json:"playgroundId,omitempty"`
	PlaygroundName        string `json:"playgroundName,omitempty"`
	PlayspecID            string `json:"playspecId,omitempty"`
	PropID                string `json:"propId,omitempty"`
	RepoURL               string `json:"repoUrl,omitempty"`
	ProvisioningLockUntil string `json:"provisioningLockUntil,omitempty"`
	CleanupLastError      string `json:"cleanupLastError,omitempty"`
}

type AdminProjectDiagnostics struct {
	Project      Project                      `json:"project"`
	Internal     AdminProjectInternal         `json:"internal"`
	WorkSessions []ProjectWorkSession         `json:"workSessions"`
	HourLedger   []AdminHourCreditLedgerEntry `json:"hourLedger"`
	Payments     []AdminBillingPayment        `json:"payments"`
}

type AdminUserDetail struct {
	Summary   AdminUserSummary      `json:"summary"`
	Projects  []AdminProjectSummary `json:"projects"`
	Notices   []UserNotice          `json:"notices"`
	AgentPool []AgentPoolOption     `json:"agentPool,omitempty"`
}

type AdminUserFilters struct {
	Query            string
	Status           string
	Github           string
	Billing          string
	Sort             string
	Page             int
	PerPage          int
	UsageWindowStart time.Time
	UsageWindowEnd   time.Time
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
