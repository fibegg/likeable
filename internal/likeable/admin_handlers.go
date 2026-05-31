package likeable

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const adminMaxHourGrant = 100

var secretConfigKeys = map[string]bool{
	"openai_api_key":        true,
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
		writeJSON(w, http.StatusOK, map[string]any{"config": publicAdminConfig(cfg), "adminEmail": s.config.AdminEmail})
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

type adminRecoveryProject struct {
	ID               string `json:"id"`
	UserID           string `json:"userId"`
	Title            string `json:"title"`
	Status           string `json:"status"`
	CleanupLastError string `json:"cleanupLastError,omitempty"`
	PlaygroundID     string `json:"workspaceId,omitempty"`
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
	freeLimitMs := s.freeHourLimitMs(r.Context())
	for i := range users {
		users[i].FreeHourLimitMs = freeLimitMs
		users[i].ProjectLimit = s.baseProjectCap(r.Context()) + users[i].PaidProjectSlots
		users[i].AgentPairs = nil
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": users,
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
	clearAdminAssignments(detail)
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
	clearAdminAssignments(detail)
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

func clearAdminAssignments(detail *AdminUserDetail) {
	if detail == nil {
		return
	}
	detail.Summary.AgentPairs = nil
	detail.AgentPool = nil
	for i := range detail.Projects {
		detail.Projects[i].Assignment = AgentAssignmentSummary{}
	}
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
	for _, key := range []string{"openai_model", "workspace_root", "free_hours", "free_hour_window_hours", "prompt_improve_charge_minutes", "project_cap", "signup_mode", "signup_allowed_emails", "stripe_publishable_key", "stripe_price_id_1_hour", "stripe_price_id_10_hours", "stripe_price_id_100_hours", "stripe_project_quota_price_id", "github_client_id", "github_username", "google_client_id", "smtp_host", "smtp_port", "smtp_username", "smtp_from_email", "smtp_from_name", "smtp_tls_mode"} {
		value := cfg[key]
		set := strings.TrimSpace(cfg[key]) != ""
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
	case "free_hours":
		return "5"
	case "free_hour_window_hours":
		return strconv.Itoa(defaultFreeHourWindowHours)
	case "prompt_improve_charge_minutes":
		return "0"
	case "project_cap":
		return "3"
	case "signup_mode":
		return "forbidden"
	case "openai_model":
		return "gpt-5-mini"
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
		case "workspace_agent_server_pool":
			continue
		case "smtp_tls_mode":
			out[key] = normalizeSMTPTLSMode(value)
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
