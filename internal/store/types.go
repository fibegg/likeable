package store

import "github.com/fibegg/likeable/internal/domain"

type User = domain.User
type Project = domain.Project
type ProjectDomain = domain.ProjectDomain
type ProjectRepository = domain.ProjectRepository
type ProjectService = domain.ProjectService
type Message = domain.Message
type MessageAttachment = domain.MessageAttachment
type ProjectNotificationTiming = domain.ProjectNotificationTiming
type ProjectWorkSession = domain.ProjectWorkSession
type SocialConnection = domain.SocialConnection
type Subscription = domain.Subscription
type Payment = domain.Payment
type ProjectArchive = domain.ProjectArchive
type AgentPoolStat = domain.AgentPoolStat
type AgentAssignmentSummary = domain.AgentAssignmentSummary
type AgentPoolOption = domain.AgentPoolOption
type UserNotice = domain.UserNotice
type AdminUserSummary = domain.AdminUserSummary
type AdminProjectSummary = domain.AdminProjectSummary
type AdminBillingPayment = domain.AdminBillingPayment
type AdminHourCreditLedgerEntry = domain.AdminHourCreditLedgerEntry
type AdminProjectInternal = domain.AdminProjectInternal
type AdminProjectDiagnostics = domain.AdminProjectDiagnostics
type AdminUserDetail = domain.AdminUserDetail
type AdminUserFilters = domain.AdminUserFilters

const (
	ProjectDomainStatusPendingDNS = domain.ProjectDomainStatusPendingDNS
	ProjectDomainStatusActive     = domain.ProjectDomainStatusActive
)
