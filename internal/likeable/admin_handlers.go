package likeable

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fibegg/likeable/internal/fibe"
	"github.com/google/uuid"
)

const adminMaxHourGrant = 100
const adminMaxProductionGrantDays = maxProductionProjectDays

type AdminReadinessCheck struct {
	Key      string `json:"key"`
	OK       bool   `json:"ok"`
	Severity string `json:"severity"`
	Detail   string `json:"detail,omitempty"`
}

type AdminReadiness struct {
	CheckedAt    string                `json:"checkedAt"`
	Ready        bool                  `json:"ready"`
	BlockerCount int                   `json:"blockerCount"`
	WarningCount int                   `json:"warningCount"`
	Checks       []AdminReadinessCheck `json:"checks"`
}

var secretConfigKeys = map[string]bool{
	"fibe_api_key":          true,
	"stripe_secret_key":     true,
	"stripe_webhook_secret": true,
	"github_client_secret":  true,
	"github_token":          true,
	"google_client_secret":  true,
	"smtp_password":         true,
	"agent_artefacts":       true,
}

func (s *Server) handleAdminConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := s.store.ConfigMap(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		stats, err := s.store.AgentPoolStats(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		pool, err := adminAgentPoolOptionsFromConfig(cfg)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": publicAdminConfig(cfg), "adminEmail": s.config.AdminEmail, "agentPoolStats": stats, "agentPool": pool, "agentPoolHealth": s.adminAgentPoolHealth(r.Context(), cfg, pool)})
	case http.MethodPut:
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		normalized, err := normalizeAdminConfigValues(body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.store.UpsertConfig(r.Context(), normalized, secretConfigKeys); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAdminAgentPoolRetire(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		AgentID       string `json:"agent_id"`
		AgentIDAlias  string `json:"agentId"`
		ServerID      string `json:"server_id"`
		ServerIDAlias string `json:"serverId"`
		MarqueeID     string `json:"marquee_id"`
		MarqueeAlias  string `json:"marqueeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	agentID := firstNonEmptyString(body.AgentID, body.AgentIDAlias)
	serverID := firstNonEmptyString(body.ServerID, body.ServerIDAlias, body.MarqueeID, body.MarqueeAlias)
	if agentID == "" || serverID == "" {
		writeError(w, http.StatusBadRequest, "agent_id and server_id are required")
		return
	}
	result, err := s.retireAgentPoolPair(r.Context(), agentID, serverID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "agent/server pair not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(result.Errors) > 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "archive failed: " + strings.Join(result.Errors, "; "), "result": result})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAdminRecovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	projects, err := s.store.DeletingProjects(r.Context(), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	users, err := s.store.PendingAccountDeletionUsers(r.Context(), accountDeletionAccessNote, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	projectRows := make([]adminRecoveryProject, 0, len(projects))
	for i := range projects {
		projectRows = append(projectRows, adminRecoveryProjectFromProject(&projects[i]))
	}
	accountRows := make([]adminRecoveryAccount, 0, len(users))
	for i := range users {
		user := &users[i]
		projects, err := s.store.AllProjectsForUser(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		accountRows = append(accountRows, adminRecoveryAccount{
			UserID:       user.ID,
			Email:        user.Email,
			ProjectCount: len(projects),
			Ready:        len(projects) == 0,
			CreatedAt:    user.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"checkedAt":                   time.Now().UTC().Format(time.RFC3339Nano),
		"deletingProjects":            projectRows,
		"pendingAccountDeletions":     accountRows,
		"deletingProjectCount":        len(projectRows),
		"pendingAccountDeletionCount": len(accountRows),
		"sweepIntervalSeconds":        int(projectDeletionSweepInterval.Seconds()),
	})
}

func (s *Server) handleAdminReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg, err := s.store.ConfigMap(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pool, err := adminAgentPoolOptionsFromConfig(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	poolHealth := s.adminAgentPoolHealth(r.Context(), cfg, activeAdminPoolOptions(pool))
	writeJSON(w, http.StatusOK, map[string]any{"readiness": s.adminReadiness(cfg, pool, poolHealth)})
}

func (s *Server) handleAdminBillingHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg, err := s.store.ConfigMap(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stripeCfg := stripeConfigFromMap(cfg)
	payments, err := s.store.RecentPayments(r.Context(), 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	priceStatus := stripePriceStatus(stripeCfg)
	issues := stripeBillingIssues(stripeCfg, priceStatus)
	products := billingProductsFromConfig(stripeCfg)
	products["projectQuotaDays"] = s.projectQuotaDays(r.Context())
	products["productionProjectDays"] = s.productionProjectDays(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"health": map[string]any{
			"checkedAt": time.Now().UTC().Format(time.RFC3339Nano),
			"configured": map[string]bool{
				"publishableKey": strings.TrimSpace(cfg["stripe_publishable_key"]) != "",
				"secretKey":      strings.TrimSpace(stripeCfg["secret"]) != "",
				"webhookSecret":  strings.TrimSpace(stripeCfg["webhook"]) != "",
			},
			"products":       products,
			"prices":         priceStatus,
			"free":           map[string]any{"minutes": s.freeBuildLimitMinutes(r.Context()), "windowHours": s.freeHourWindowHours(r.Context())},
			"issues":         issues,
			"recentPayments": payments,
		},
	})
}

func stripePriceStatus(stripeCfg map[string]string) map[string]bool {
	return map[string]bool{
		"oneHour":           strings.TrimSpace(stripeCfg["price_1_hour"]) != "",
		"tenHours":          strings.TrimSpace(stripeCfg["price_10_hours"]) != "",
		"hundredHours":      strings.TrimSpace(stripeCfg["price_100_hours"]) != "",
		"projectQuota":      strings.TrimSpace(stripeCfg["project_quota_price"]) != "",
		"productionProject": strings.TrimSpace(stripeCfg["production_project_price"]) != "",
	}
}

func stripeBillingIssues(stripeCfg map[string]string, priceStatus map[string]bool) []string {
	issues := []string{}
	if strings.TrimSpace(stripeCfg["secret"]) == "" {
		issues = append(issues, "stripe_secret_missing")
	}
	if strings.TrimSpace(stripeCfg["webhook"]) == "" {
		issues = append(issues, "stripe_webhook_missing")
	}
	if !priceStatus["oneHour"] && !priceStatus["tenHours"] && !priceStatus["hundredHours"] {
		issues = append(issues, "stripe_hour_prices_missing")
	}
	if !priceStatus["projectQuota"] {
		issues = append(issues, "stripe_project_quota_price_missing")
	}
	if !priceStatus["productionProject"] {
		issues = append(issues, "stripe_production_project_price_missing")
	}
	return issues
}

func (s *Server) adminReadiness(cfg map[string]string, pool []AgentPoolOption, poolHealth []AgentPoolHealth) AdminReadiness {
	stripeCfg := stripeConfigFromMap(cfg)
	priceStatus := stripePriceStatus(stripeCfg)
	checks := []AdminReadinessCheck{}
	addAdminReadinessCheck(&checks, "stripe_secret", "blocker", strings.TrimSpace(stripeCfg["secret"]) != "", "")
	addAdminReadinessCheck(&checks, "stripe_webhook", "blocker", strings.TrimSpace(stripeCfg["webhook"]) != "", "")
	addAdminReadinessCheck(&checks, "stripe_hour_prices", "blocker", priceStatus["oneHour"] || priceStatus["tenHours"] || priceStatus["hundredHours"], "")
	addAdminReadinessCheck(&checks, "stripe_project_quota_price", "blocker", priceStatus["projectQuota"], "")
	addAdminReadinessCheck(&checks, "stripe_production_project_price", "blocker", priceStatus["productionProject"], "")
	greenfieldReady, greenfieldDetail := greenfieldTemplateReadiness(cfg)
	addAdminReadinessCheck(&checks, "fibe_template_version", "blocker", greenfieldReady, greenfieldDetail)
	activePoolCount := 0
	for _, option := range pool {
		if strings.TrimSpace(option.Status) == fibe.AssignmentStatusActive {
			activePoolCount++
		}
	}
	addAdminReadinessCheck(&checks, "fibe_active_pool", "blocker", activePoolCount > 0, "")
	healthyActivePool, activePoolDetail := activePoolReadiness(poolHealth, activePoolCount)
	addAdminReadinessCheck(&checks, "fibe_active_pool_health", "blocker", healthyActivePool, activePoolDetail)
	addAdminReadinessCheck(&checks, "google_oauth", "blocker", strings.TrimSpace(cfg["google_client_id"]) != "" && strings.TrimSpace(cfg["google_client_secret"]) != "", "")
	addAdminReadinessCheck(&checks, "smtp_delivery", "warning", strings.TrimSpace(cfg["smtp_host"]) != "" && strings.TrimSpace(cfg["smtp_from_email"]) != "", "")
	signupMode := strings.TrimSpace(cfg["signup_mode"])
	if signupMode == "" {
		signupMode = "forbidden"
	}
	addAdminReadinessCheck(&checks, "signup_enabled", "warning", signupMode != "forbidden", signupMode)

	readiness := AdminReadiness{
		CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Checks:    checks,
		Ready:     true,
	}
	for _, check := range checks {
		if check.OK {
			continue
		}
		if check.Severity == "warning" {
			readiness.WarningCount++
			continue
		}
		readiness.BlockerCount++
		readiness.Ready = false
	}
	return readiness
}

func addAdminReadinessCheck(checks *[]AdminReadinessCheck, key, severity string, ok bool, detail string) {
	*checks = append(*checks, AdminReadinessCheck{
		Key:      key,
		OK:       ok,
		Severity: severity,
		Detail:   strings.TrimSpace(detail),
	})
}

func greenfieldTemplateReadiness(cfg map[string]string) (bool, string) {
	if strings.TrimSpace(cfg["fibe_template_version_id"]) != "" {
		return true, "template version id"
	}
	body, err := fibe.GreenfieldTemplateBody()
	if err != nil {
		return false, err.Error()
	}
	if strings.TrimSpace(body) != "" {
		return true, "bundled template body"
	}
	return false, "no template version id or bundled template body"
}

func activeAdminPoolOptions(pool []AgentPoolOption) []AgentPoolOption {
	active := []AgentPoolOption{}
	for _, option := range pool {
		if strings.TrimSpace(option.Status) == fibe.AssignmentStatusActive {
			active = append(active, option)
		}
	}
	return active
}

func activePoolReadiness(poolHealth []AgentPoolHealth, activePoolCount int) (bool, string) {
	if activePoolCount == 0 {
		return false, "no active agent/server pairs configured"
	}
	failures := []string{}
	for _, health := range poolHealth {
		if strings.TrimSpace(health.Status) != fibe.AssignmentStatusActive {
			continue
		}
		pair := readinessPoolPairLabel(health.Label, health.AgentID, health.ServerID)
		if health.OK {
			return true, pair
		}
		if len(health.Problems) > 0 {
			failures = append(failures, pair+": "+strings.Join(health.Problems, "; "))
		} else {
			failures = append(failures, pair)
		}
	}
	if len(failures) == 0 {
		return false, "active agent/server pairs did not return health"
	}
	return false, strings.Join(failures, " | ")
}

func readinessPoolPairLabel(label, agentID, serverID string) string {
	if strings.TrimSpace(label) != "" {
		return strings.TrimSpace(label)
	}
	return strings.TrimSpace(agentID) + "/" + strings.TrimSpace(serverID)
}

type adminRecoveryProject struct {
	ID               string `json:"id"`
	UserID           string `json:"userId"`
	Title            string `json:"title"`
	Status           string `json:"status"`
	CleanupLastError string `json:"cleanupLastError,omitempty"`
	PlaygroundID     string `json:"playgroundId,omitempty"`
	PlayspecID       string `json:"playspecId,omitempty"`
	PropID           string `json:"propId,omitempty"`
	UpdatedAt        string `json:"updatedAt"`
}

type adminRecoveryAccount struct {
	UserID       string `json:"userId"`
	Email        string `json:"email"`
	ProjectCount int    `json:"projectCount"`
	Ready        bool   `json:"ready"`
	CreatedAt    string `json:"createdAt"`
}

func adminRecoveryProjectFromProject(project *Project) adminRecoveryProject {
	if project == nil {
		return adminRecoveryProject{}
	}
	return adminRecoveryProject{
		ID:               project.ID,
		UserID:           project.UserID,
		Title:            project.Title,
		Status:           project.Status,
		CleanupLastError: project.CleanupLastError,
		PlaygroundID:     project.PlaygroundID,
		PlayspecID:       project.PlayspecID,
		PropID:           project.PropID,
		UpdatedAt:        project.UpdatedAt,
	}
}

type agentPoolRetirementResult struct {
	AgentID       string   `json:"agentId"`
	ServerID      string   `json:"serverId"`
	Status        string   `json:"status"`
	ProjectCount  int      `json:"projectCount"`
	ArchivedCount int      `json:"archivedCount"`
	Errors        []string `json:"errors,omitempty"`
}

func (s *Server) retireAgentPoolPair(ctx context.Context, agentID, serverID string) (agentPoolRetirementResult, error) {
	result := agentPoolRetirementResult{AgentID: agentID, ServerID: serverID, Status: fibe.AssignmentStatusRetiring}
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return result, err
	}
	pool, err := fibe.AssignmentPoolFromConfig(cfg)
	if err != nil {
		return result, err
	}
	index := -1
	for i := range pool {
		if strings.TrimSpace(pool[i].AgentID) == agentID && strings.TrimSpace(pool[i].MarqueeID) == serverID {
			index = i
			break
		}
	}
	if index < 0 {
		return result, sql.ErrNoRows
	}
	pool[index].Status = fibe.AssignmentStatusRetiring
	if err := s.store.UpsertConfig(ctx, map[string]string{"fibe_agent_server_pool": fibe.EncodeAssignmentPool(pool)}, secretConfigKeys); err != nil {
		return result, err
	}
	projects, err := s.store.ProjectsForAssignment(ctx, agentID, serverID)
	if err != nil {
		return result, err
	}
	result.ProjectCount = len(projects)
	for i := range projects {
		project := projects[i]
		user, err := s.store.UserByID(ctx, project.UserID)
		if err != nil {
			result.Errors = append(result.Errors, project.ID+": "+err.Error())
			continue
		}
		if project.Status == "archived" {
			if _, err := s.store.LatestProjectArchive(ctx, user.ID, project.ID); err == nil {
				result.ArchivedCount++
				continue
			}
		}
		if _, err := s.archiveProjectSource(ctx, user, &project); err != nil {
			result.Errors = append(result.Errors, project.ID+": "+err.Error())
			continue
		}
		if err := s.markProjectArchived(ctx, user.ID, &project); err != nil {
			result.Errors = append(result.Errors, project.ID+": "+err.Error())
			continue
		}
		result.ArchivedCount++
	}
	if len(result.Errors) > 0 {
		return result, nil
	}
	pool[index].Status = fibe.AssignmentStatusRetired
	if err := s.store.UpsertConfig(ctx, map[string]string{"fibe_agent_server_pool": fibe.EncodeAssignmentPool(pool)}, secretConfigKeys); err != nil {
		return result, err
	}
	result.Status = fibe.AssignmentStatusRetired
	return result, nil
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/admin/users")
	if rest == "" || rest == "/" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleAdminUsersIndex(w, r)
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	userID := parts[0]
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		s.handleAdminUserShow(w, r, userID)
	case len(parts) == 1 && r.Method == http.MethodPatch:
		s.handleAdminUserAccess(w, r, userID)
	case len(parts) == 2 && parts[1] == "notices" && r.Method == http.MethodPost:
		s.handleAdminUserNotice(w, r, userID)
	case len(parts) == 3 && parts[1] == "notices" && r.Method == http.MethodDelete:
		s.handleAdminUserNoticeUnsend(w, r, userID, parts[2])
	case len(parts) == 3 && parts[1] == "billing" && parts[2] == "hours" && r.Method == http.MethodPost:
		s.handleAdminUserGrantHours(w, r, userID)
	case len(parts) == 3 && parts[1] == "projects" && r.Method == http.MethodDelete:
		s.handleAdminUserProjectDelete(w, r, userID, parts[2])
	case len(parts) == 4 && parts[1] == "projects" && parts[3] == "diagnostics" && r.Method == http.MethodGet:
		s.handleAdminUserProjectDiagnostics(w, r, userID, parts[2])
	case len(parts) == 4 && parts[1] == "projects" && parts[3] == "production" && r.Method == http.MethodPost:
		s.handleAdminUserProjectProductionGrant(w, r, userID, parts[2])
	case len(parts) == 5 && parts[1] == "projects" && parts[3] == "production" && parts[4] == "start" && r.Method == http.MethodPost:
		s.handleAdminUserProjectProductionStart(w, r, userID, parts[2])
	case len(parts) == 4 && parts[1] == "projects" && parts[3] == "assignment" && r.Method == http.MethodPatch:
		s.handleAdminUserProjectAssignment(w, r, userID, parts[2])
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleAdminUsersIndex(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	windowStart, windowEnd := s.freeHourWindow(time.Now(), r.Context())
	filters := AdminUserFilters{
		Query:            query.Get("q"),
		Status:           query.Get("status"),
		Github:           query.Get("github"),
		Billing:          query.Get("billing"),
		Sort:             query.Get("sort"),
		Page:             boundedQueryInt(query.Get("page"), 1, 1, 100000),
		PerPage:          boundedQueryInt(query.Get("per_page"), 25, 1, 100),
		UsageWindowStart: windowStart,
		UsageWindowEnd:   windowEnd,
	}
	users, total, err := s.store.AdminUsers(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pool, err := s.adminAgentPoolOptions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	freeLimitMs := s.freeHourLimitMs(r.Context())
	for i := range users {
		users[i].FreeHourLimitMs = freeLimitMs
		users[i].ProjectLimit = s.baseProjectCap(r.Context()) + users[i].PaidProjectSlots
		decorateAdminUserAssignmentStatuses(&users[i], pool)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users":     users,
		"agentPool": pool,
		"pagination": map[string]any{
			"page":    filters.Page,
			"perPage": filters.PerPage,
			"total":   total,
		},
	})
}

func (s *Server) handleAdminUserShow(w http.ResponseWriter, r *http.Request, userID string) {
	windowStart, windowEnd := s.freeHourWindow(time.Now(), r.Context())
	detail, err := s.store.AdminUserDetail(r.Context(), userID, s.freeHourLimitMs(r.Context()), windowStart, windowEnd)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail.Summary.ProjectLimit = s.baseProjectCap(r.Context()) + detail.Summary.PaidProjectSlots
	pool, err := s.adminAgentPoolOptions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	decorateAdminDetailAssignmentStatuses(detail, pool)
	detail.AgentPool = pool
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminUserAccess(w http.ResponseWriter, r *http.Request, userID string) {
	var body struct {
		AccessStatus string `json:"accessStatus"`
		AccessNote   string `json:"accessNote"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	target, err := s.store.UserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if normalizeEmail(target.Email) == s.config.AdminEmail && strings.EqualFold(body.AccessStatus, "restricted") {
		writeError(w, http.StatusBadRequest, "cannot restrict the configured admin")
		return
	}
	user, err := s.store.UpdateUserAccess(r.Context(), userID, body.AccessStatus, body.AccessNote)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleAdminUserNotice(w http.ResponseWriter, r *http.Request, userID string) {
	var body struct {
		Severity string `json:"severity"`
		Body     string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	target, err := s.store.UserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	notice, err := s.store.AddUserNotice(r.Context(), UserNotice{UserID: userID, Sender: "admin", Severity: body.Severity, Body: body.Body})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.sendUserEmailAsync(target.Email, "New Likeable message", s.systemMessageEmailBody(target, notice.Body))
	writeJSON(w, http.StatusCreated, map[string]any{"notice": notice})
}

func (s *Server) handleAdminUserGrantHours(w http.ResponseWriter, r *http.Request, userID string) {
	var body struct {
		Hours int `json:"hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Hours <= 0 || body.Hours > adminMaxHourGrant {
		writeError(w, http.StatusBadRequest, "hours must be between 1 and "+strconv.Itoa(adminMaxHourGrant))
		return
	}
	if _, err := s.store.UserByID(r.Context(), userID); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	granted, err := s.store.GrantHourCredits(r.Context(), userID, "admin_grant_"+uuid.NewString(), body.Hours)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if granted {
		hourLabel := "hours"
		if body.Hours == 1 {
			hourLabel = "hour"
		}
		_, _ = s.store.AddUserNotice(r.Context(), UserNotice{
			UserID:   userID,
			Sender:   "system",
			Severity: "info",
			Body:     "Support added " + strconv.Itoa(body.Hours) + " build " + hourLabel + " to your account.",
		})
	}
	windowStart, windowEnd := s.freeHourWindow(time.Now(), r.Context())
	detail, err := s.store.AdminUserDetail(r.Context(), userID, s.freeHourLimitMs(r.Context()), windowStart, windowEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail.Summary.ProjectLimit = s.baseProjectCap(r.Context()) + detail.Summary.PaidProjectSlots
	pool, err := s.adminAgentPoolOptions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	decorateAdminDetailAssignmentStatuses(detail, pool)
	detail.AgentPool = pool
	writeJSON(w, http.StatusOK, map[string]any{"detail": detail, "granted": granted, "hours": body.Hours})
}

func (s *Server) handleAdminUserNoticeUnsend(w http.ResponseWriter, r *http.Request, userID, noticeID string) {
	if _, err := s.store.UserByID(r.Context(), userID); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	notice, err := s.store.UnsendUserNotice(r.Context(), userID, noticeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notice": notice})
}

func (s *Server) handleAdminUserProjectDelete(w http.ResponseWriter, r *http.Request, userID, projectID string) {
	target, err := s.store.UserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	project, err := s.store.ProjectForUser(r.Context(), userID, projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if project.Status != "deleting" {
		if err := s.store.UpdateProjectStatus(r.Context(), project.ID, userID, "deleting"); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		project.Status = "deleting"
		s.notifyProjectDeletionScheduled(r.Context(), target, project)
		s.deleteProjectResourcesAsync(userID, target.Email, project)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"project": project})
}

func (s *Server) handleAdminUserProjectDiagnostics(w http.ResponseWriter, r *http.Request, userID, projectID string) {
	diagnostics, err := s.store.AdminProjectDiagnostics(r.Context(), userID, projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.decorateAdminProjectRuntimeDiagnostics(r.Context(), diagnostics)
	writeJSON(w, http.StatusOK, map[string]any{"diagnostics": diagnostics})
}

func (s *Server) handleAdminUserProjectProductionGrant(w http.ResponseWriter, r *http.Request, userID, projectID string) {
	var body struct {
		Days int `json:"days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	days := body.Days
	if days == 0 {
		days = s.productionProjectDays(r.Context())
	}
	if days <= 0 || days > adminMaxProductionGrantDays {
		writeError(w, http.StatusBadRequest, "days must be between 1 and "+strconv.Itoa(adminMaxProductionGrantDays))
		return
	}
	if _, err := s.store.UserByID(r.Context(), userID); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	project, err := s.store.ProjectForUser(r.Context(), userID, projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if project.Status == "archived" || project.Status == "deleting" {
		writeError(w, http.StatusConflict, "production grant requires an active project")
		return
	}
	expiresAt := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
	granted, err := s.store.GrantProjectProduction(r.Context(), userID, projectID, "admin_production_"+uuid.NewString(), expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if granted {
		s.startProductionProjectIfStopped(r.Context(), userID, projectID)
		s.notifyProductionProjectPurchased(r.Context(), userID, projectID, expiresAt)
	}
	updated, err := s.store.ProjectForUser(r.Context(), userID, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	windowStart, windowEnd := s.freeHourWindow(time.Now(), r.Context())
	detail, err := s.store.AdminUserDetail(r.Context(), userID, s.freeHourLimitMs(r.Context()), windowStart, windowEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail.Summary.ProjectLimit = s.baseProjectCap(r.Context()) + detail.Summary.PaidProjectSlots
	pool, err := s.adminAgentPoolOptions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	decorateAdminDetailAssignmentStatuses(detail, pool)
	detail.AgentPool = pool
	writeJSON(w, http.StatusOK, map[string]any{"detail": detail, "project": updated, "granted": granted, "days": days})
}

func (s *Server) handleAdminUserProjectProductionStart(w http.ResponseWriter, r *http.Request, userID, projectID string) {
	target, err := s.store.UserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	project, err := s.store.ProjectForUser(r.Context(), userID, projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if strings.TrimSpace(project.ProductionExpiresAt) == "" {
		writeError(w, http.StatusConflict, "production start retry requires an active production grant")
		return
	}
	if project.Status == "archived" || project.Status == "deleting" {
		writeError(w, http.StatusConflict, "production start retry requires an active project")
		return
	}

	started := false
	blockedCode := ""
	warning := ""
	updated := project
	if project.Status == "stopped" {
		updated, err = s.controlProjectPlayground(r.Context(), target, project, "start")
		if err != nil {
			if fibe.IsRuntimeBillingRequiredError(err) {
				blockedCode = "runtime_billing_required"
				warning = productionRuntimeBillingRequiredMessage
				s.notifyProductionProjectStartBlocked(r.Context(), target, project)
				updated = project
			} else if isPlatformRateLimited(err) {
				writeError(w, http.StatusServiceUnavailable, "workspace platform is rate limited; try again shortly")
				return
			} else {
				writeError(w, http.StatusBadGateway, "could not start production playground")
				return
			}
		} else {
			started = true
		}
	}

	windowStart, windowEnd := s.freeHourWindow(time.Now(), r.Context())
	detail, err := s.store.AdminUserDetail(r.Context(), userID, s.freeHourLimitMs(r.Context()), windowStart, windowEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail.Summary.ProjectLimit = s.baseProjectCap(r.Context()) + detail.Summary.PaidProjectSlots
	pool, err := s.adminAgentPoolOptions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	decorateAdminDetailAssignmentStatuses(detail, pool)
	detail.AgentPool = pool
	writeJSON(w, http.StatusAccepted, map[string]any{"detail": detail, "project": updated, "started": started, "blockedCode": blockedCode, "warning": warning})
}

func (s *Server) decorateAdminProjectRuntimeDiagnostics(ctx context.Context, diagnostics *AdminProjectDiagnostics) {
	if diagnostics == nil || strings.TrimSpace(diagnostics.Project.ProductionExpiresAt) == "" {
		return
	}
	switch diagnostics.Project.Status {
	case "ready":
		diagnostics.Internal.ProductionRuntimeStatus = "running"
	case "creating", "launching":
		diagnostics.Internal.ProductionRuntimeStatus = "starting"
	case "stopped":
		diagnostics.Internal.ProductionRuntimeStatus = "stopped"
	default:
		diagnostics.Internal.ProductionRuntimeStatus = diagnostics.Project.Status
	}
	if diagnostics.Project.Status != "stopped" {
		return
	}
	notices, err := s.store.NoticesForUser(ctx, diagnostics.Project.UserID, 50)
	if err != nil {
		return
	}
	prefix := "Production runtime paused: " + strconv.Quote(diagnostics.Project.Title)
	for _, notice := range notices {
		if notice.Sender == "system" && strings.HasPrefix(notice.Body, prefix) {
			diagnostics.Internal.ProductionRuntimeStatus = "runtime_billing_required"
			diagnostics.Internal.ProductionRuntimeMessage = notice.Body
			diagnostics.Internal.ProductionRuntimeBlockedAt = notice.CreatedAt
			return
		}
	}
}

func (s *Server) handleAdminUserProjectAssignment(w http.ResponseWriter, r *http.Request, userID, projectID string) {
	var body struct {
		AgentID       string `json:"agent_id"`
		AgentIDAlias  string `json:"agentId"`
		ServerID      string `json:"server_id"`
		ServerIDAlias string `json:"serverId"`
		MarqueeID     string `json:"marquee_id"`
		MarqueeAlias  string `json:"marqueeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	agentID := firstNonEmptyString(body.AgentID, body.AgentIDAlias)
	serverID := firstNonEmptyString(body.ServerID, body.ServerIDAlias, body.MarqueeID, body.MarqueeAlias)
	if agentID == "" || serverID == "" {
		writeError(w, http.StatusBadRequest, "agent_id and server_id are required")
		return
	}
	target, err := s.store.UserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	project, err := s.store.ProjectForUser(r.Context(), userID, projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if project.Status == "deleting" || project.Status == "archived" {
		writeError(w, http.StatusConflict, "project cannot be reassigned")
		return
	}
	pool, err := s.adminAgentPoolOptions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status, found := assignmentStatusForPairInPool(pool, agentID, serverID)
	if !found {
		writeError(w, http.StatusNotFound, "agent/server pair not found")
		return
	}
	if status != fibe.AssignmentStatusActive {
		writeError(w, http.StatusBadRequest, "agent/server pair is not active")
		return
	}
	if err := s.store.UpdateProjectAssignment(r.Context(), projectID, userID, agentID, serverID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.invalidateProjectFeedCache(projectID)
	updated, err := s.store.ProjectForUser(r.Context(), userID, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	warning := s.warmProjectAssignmentWarning(r.Context(), target.Email, updated)
	windowStart, windowEnd := s.freeHourWindow(time.Now(), r.Context())
	detail, err := s.store.AdminUserDetail(r.Context(), userID, s.freeHourLimitMs(r.Context()), windowStart, windowEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail.Summary.ProjectLimit = s.baseProjectCap(r.Context()) + detail.Summary.PaidProjectSlots
	decorateAdminDetailAssignmentStatuses(detail, pool)
	detail.AgentPool = pool
	writeJSON(w, http.StatusOK, map[string]any{"detail": detail, "project": updated, "warning": warning})
}

func (s *Server) warmProjectAssignmentWarning(ctx context.Context, userEmail string, project *Project) string {
	if project == nil || strings.TrimSpace(project.ConversationID) == "" {
		return ""
	}
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return "assignment saved, but the new agent could not be warmed: " + err.Error()
	}
	if strings.TrimSpace(cfg["fibe_base_url"]) == "" || strings.TrimSpace(cfg["fibe_api_key"]) == "" {
		return ""
	}
	client, err := s.fibeClientForProject(ctx, project, userEmail)
	if err != nil {
		return "assignment saved, but the new agent could not be warmed: " + err.Error()
	}
	warmCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	if err := client.StartAgentChat(warmCtx); err != nil {
		return "assignment saved, but the new agent could not be warmed: " + err.Error()
	}
	if err := client.EnsureConversation(warmCtx, project.ConversationID, project.Title); err != nil {
		return "assignment saved, but the project conversation could not be prepared on the new agent: " + err.Error()
	}
	return ""
}

func (s *Server) adminAgentPoolOptions(ctx context.Context) ([]AgentPoolOption, error) {
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return nil, err
	}
	return adminAgentPoolOptionsFromConfig(cfg)
}

func adminAgentPoolOptionsFromConfig(cfg map[string]string) ([]AgentPoolOption, error) {
	pool, err := fibe.AssignmentPoolFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	options := make([]AgentPoolOption, 0, len(pool))
	for _, assignment := range pool {
		options = append(options, AgentPoolOption{
			Label:    strings.TrimSpace(assignment.Label),
			AgentID:  strings.TrimSpace(assignment.AgentID),
			ServerID: strings.TrimSpace(assignment.MarqueeID),
			Status:   fibe.AssignmentStatus(assignment),
			Capacity: assignment.Capacity,
		})
	}
	return options, nil
}

func (s *Server) adminAgentPoolHealth(ctx context.Context, cfg map[string]string, pool []AgentPoolOption) []AgentPoolHealth {
	if len(pool) == 0 {
		return []AgentPoolHealth{}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out := make([]AgentPoolHealth, 0, len(pool))
	for _, option := range pool {
		health := AgentPoolHealth{
			Label:    strings.TrimSpace(option.Label),
			AgentID:  strings.TrimSpace(option.AgentID),
			ServerID: strings.TrimSpace(option.ServerID),
			Status:   fibe.AssignmentStatus(fibe.Assignment{Status: option.Status}),
		}
		client, err := s.fibeClientFromConfig(cfg, fibe.Assignment{AgentID: health.AgentID, MarqueeID: health.ServerID})
		if err != nil {
			health.Problems = []string{err.Error()}
			out = append(out, health)
			continue
		}
		checked := client.AssignmentHealth(ctx)
		health.AgentStatus = checked.AgentStatus
		health.AgentAuthenticated = checked.AgentAuthenticated
		health.ServerStatus = checked.MarqueeStatus
		health.ServerBillingRuntimeActive = checked.MarqueeBillingRuntimeActive
		health.ServerChatLaunchable = checked.MarqueeChatLaunchable
		health.Problems = checked.Problems
		health.OK = checked.OK
		out = append(out, health)
		if ctx.Err() != nil {
			break
		}
	}
	return out
}

func decorateAdminDetailAssignmentStatuses(detail *AdminUserDetail, pool []AgentPoolOption) {
	if detail == nil {
		return
	}
	decorateAdminUserAssignmentStatuses(&detail.Summary, pool)
	for i := range detail.Projects {
		detail.Projects[i].Assignment.Status = assignmentStatusForPair(pool, detail.Projects[i].Assignment.AgentID, detail.Projects[i].Assignment.ServerID)
	}
}

func decorateAdminUserAssignmentStatuses(summary *AdminUserSummary, pool []AgentPoolOption) {
	if summary == nil {
		return
	}
	for i := range summary.AgentPairs {
		summary.AgentPairs[i].Status = assignmentStatusForPair(pool, summary.AgentPairs[i].AgentID, summary.AgentPairs[i].ServerID)
	}
}

func assignmentStatusForPair(pool []AgentPoolOption, agentID, serverID string) string {
	agentID = strings.TrimSpace(agentID)
	serverID = strings.TrimSpace(serverID)
	if agentID == "" && serverID == "" {
		return ""
	}
	if status, found := assignmentStatusForPairInPool(pool, agentID, serverID); found {
		return status
	}
	return fibe.AssignmentStatusRetired
}

func assignmentStatusForPairInPool(pool []AgentPoolOption, agentID, serverID string) (string, bool) {
	agentID = strings.TrimSpace(agentID)
	serverID = strings.TrimSpace(serverID)
	for _, option := range pool {
		if strings.TrimSpace(option.AgentID) == agentID && strings.TrimSpace(option.ServerID) == serverID {
			return strings.TrimSpace(option.Status), true
		}
	}
	return "", false
}

func boundedQueryInt(raw string, fallback, min, max int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "<nil>" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func publicAdminConfig(cfg map[string]string) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"fibe_base_url", "fibe_agent_server_pool", "fibe_template_version_id", "free_minutes", "free_hour_window_hours", "prompt_improve_charge_minutes", "project_cap", "project_quota_days", "production_project_days", "signup_mode", "signup_allowed_emails", "stripe_publishable_key", "stripe_price_id_1_hour", "stripe_price_id_10_hours", "stripe_price_id_100_hours", "stripe_project_quota_price_id", "stripe_production_project_price_id", "github_client_id", "github_username", "google_client_id", "smtp_host", "smtp_port", "smtp_username", "smtp_from_email", "smtp_from_name", "smtp_tls_mode"} {
		value := cfg[key]
		set := strings.TrimSpace(cfg[key]) != ""
		if key == "free_minutes" && strings.TrimSpace(value) == "" {
			if legacyHours := strings.TrimSpace(cfg["free_hours"]); legacyHours != "" {
				if n, err := strconv.Atoi(legacyHours); err == nil && n >= 0 {
					minutes := n * 60
					if minutes > maxFreeBuildMinutes {
						minutes = maxFreeBuildMinutes
					}
					value = strconv.Itoa(minutes)
					set = true
				}
			}
		}
		if key == "fibe_agent_server_pool" && strings.TrimSpace(value) == "" {
			value = cfg["fibe_agent_marquee_pool"]
			set = strings.TrimSpace(value) != ""
		}
		if strings.TrimSpace(value) == "" {
			value = publicConfigDefault(key)
		}
		if key == "signup_allowed_emails" {
			value = normalizeEmailListConfig(value)
		}
		out[key] = map[string]any{"value": value, "secret": false, "set": set}
	}
	for key := range secretConfigKeys {
		out[key] = map[string]any{"value": "", "secret": true, "set": strings.TrimSpace(cfg[key]) != ""}
	}
	return out
}

func publicConfigDefault(key string) string {
	switch key {
	case "free_minutes":
		return strconv.Itoa(defaultFreeBuildMinutes)
	case "free_hour_window_hours":
		return strconv.Itoa(defaultFreeHourWindowHours)
	case "prompt_improve_charge_minutes":
		return "0"
	case "project_cap":
		return "3"
	case "project_quota_days":
		return strconv.Itoa(defaultProjectQuotaDays)
	case "production_project_days":
		return strconv.Itoa(defaultProductionProjectDays)
	case "signup_mode":
		return "forbidden"
	case "fibe_agent_server_pool":
		return "[]"
	case "smtp_port":
		return "587"
	case "smtp_from_name":
		return "Likeable"
	case "smtp_tls_mode":
		return "auto"
	default:
		return ""
	}
}

func normalizeAdminConfigValues(values map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(values))
	for key, value := range values {
		switch key {
		case "signup_allowed_emails":
			out[key] = normalizeEmailListConfig(value)
		case "fibe_agent_server_pool", "fibe_agent_marquee_pool":
			pool, err := fibe.ParseAssignmentPool(value)
			if err != nil {
				return nil, err
			}
			if len(pool) == 0 {
				out["fibe_agent_server_pool"] = ""
				out["fibe_agent_marquee_pool"] = ""
			} else {
				encoded := fibe.EncodeAssignmentPool(pool)
				out["fibe_agent_server_pool"] = encoded
				out["fibe_agent_marquee_pool"] = ""
			}
		case "smtp_tls_mode":
			out[key] = normalizeSMTPTLSMode(value)
		case "free_minutes":
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				out[key] = ""
				continue
			}
			n, err := strconv.Atoi(trimmed)
			if err != nil || n < 0 || n > maxFreeBuildMinutes {
				return nil, errors.New("free_minutes must be between 0 and 1440")
			}
			out[key] = strconv.Itoa(n)
		case "free_hour_window_hours":
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				out[key] = ""
				continue
			}
			n, err := strconv.Atoi(trimmed)
			if err != nil || n <= 0 || n > maxFreeHourWindowHours {
				return nil, errors.New("free_hour_window_hours must be between 1 and 24")
			}
			out[key] = strconv.Itoa(n)
		case "prompt_improve_charge_minutes":
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				out[key] = ""
				continue
			}
			n, err := strconv.Atoi(trimmed)
			if err != nil || n < 0 || n > maxPromptImproveChargeMin {
				return nil, errors.New("prompt_improve_charge_minutes must be between 0 and 60")
			}
			out[key] = strconv.Itoa(n)
		case "project_quota_days":
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				out[key] = ""
				continue
			}
			n, err := strconv.Atoi(trimmed)
			if err != nil || n <= 0 || n > maxProjectQuotaDays {
				return nil, errors.New("project_quota_days must be between 1 and 365")
			}
			out[key] = strconv.Itoa(n)
		case "production_project_days":
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				out[key] = ""
				continue
			}
			n, err := strconv.Atoi(trimmed)
			if err != nil || n <= 0 || n > maxProductionProjectDays {
				return nil, errors.New("production_project_days must be between 1 and 365")
			}
			out[key] = strconv.Itoa(n)
		default:
			out[key] = strings.TrimSpace(value)
		}
	}
	return out, nil
}

func normalizeEmailListConfig(raw string) string {
	var out []string
	seen := map[string]bool{}
	for _, item := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		item = normalizeEmail(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return strings.Join(out, "\n")
}
