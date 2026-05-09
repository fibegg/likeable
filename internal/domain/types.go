package domain

import (
	"strings"
	"time"
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
	CreatedAt             string              `json:"createdAt"`
	UpdatedAt             string              `json:"updatedAt"`
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
	User               User        `json:"user"`
	ProjectCount       int         `json:"projectCount"`
	ProjectLimit       int         `json:"projectLimit"`
	PaidProjectSlots   int         `json:"paidProjectSlots"`
	ProjectSlotsExpire string      `json:"projectSlotsExpire,omitempty"`
	MessageCount       int         `json:"messageCount"`
	DailyMessageCount  int         `json:"dailyMessageCount"`
	FreeMessageLimit   int         `json:"freeMessageLimit"`
	PaidCreditBalance  int         `json:"paidCreditBalance"`
	GithubConnected    bool        `json:"githubConnected"`
	SubscriptionStatus string      `json:"subscriptionStatus"`
	PaidTotalCents     int64       `json:"paidTotalCents"`
	PaidCurrency       string      `json:"paidCurrency"`
	LastMessageAt      string      `json:"lastMessageAt,omitempty"`
	LastProjectAt      string      `json:"lastProjectAt,omitempty"`
	LatestNotice       *UserNotice `json:"latestNotice,omitempty"`
}

type AdminProjectSummary struct {
	Project      Project `json:"project"`
	MessageCount int     `json:"messageCount"`
}

type AdminUserDetail struct {
	Summary  AdminUserSummary      `json:"summary"`
	Projects []AdminProjectSummary `json:"projects"`
	Notices  []UserNotice          `json:"notices"`
}

type AdminUserFilters struct {
	Query   string
	Status  string
	Github  string
	Billing string
	Sort    string
	Page    int
	PerPage int
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
