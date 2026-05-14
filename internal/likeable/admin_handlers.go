package likeable

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fibegg/likeable/internal/fibe"
)

var secretConfigKeys = map[string]bool{
	"fibe_api_key":          true,
	"stripe_secret_key":     true,
	"stripe_webhook_secret": true,
	"github_client_secret":  true,
	"google_client_secret":  true,
	"smtp_password":         true,
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
		writeJSON(w, http.StatusOK, map[string]any{"config": publicAdminConfig(cfg), "adminEmail": s.config.AdminEmail, "agentPoolStats": stats})
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
	case len(parts) == 3 && parts[1] == "projects" && r.Method == http.MethodDelete:
		s.handleAdminUserProjectDelete(w, r, userID, parts[2])
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleAdminUsersIndex(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filters := AdminUserFilters{
		Query:   query.Get("q"),
		Status:  query.Get("status"),
		Github:  query.Get("github"),
		Billing: query.Get("billing"),
		Sort:    query.Get("sort"),
		Page:    boundedQueryInt(query.Get("page"), 1, 1, 100000),
		PerPage: boundedQueryInt(query.Get("per_page"), 25, 1, 100),
	}
	users, total, err := s.store.AdminUsers(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	freeLimit := s.freeMessageLimit(r.Context())
	for i := range users {
		users[i].FreeMessageLimit = freeLimit
		users[i].ProjectLimit = s.baseProjectCap(r.Context()) + users[i].PaidProjectSlots
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
	detail, err := s.store.AdminUserDetail(r.Context(), userID, s.freeMessageLimit(r.Context()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail.Summary.ProjectLimit = s.baseProjectCap(r.Context()) + detail.Summary.PaidProjectSlots
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
	for _, key := range []string{"fibe_base_url", "fibe_agent_server_pool", "fibe_template_version_id", "free_messages", "project_cap", "signup_mode", "signup_allowed_emails", "stripe_publishable_key", "stripe_price_id_10", "stripe_price_id_100", "stripe_price_id_1000", "stripe_project_quota_price_id", "github_client_id", "google_client_id", "smtp_host", "smtp_port", "smtp_username", "smtp_from_email", "smtp_from_name", "smtp_tls_mode"} {
		value := cfg[key]
		set := strings.TrimSpace(cfg[key]) != ""
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
	case "free_messages":
		return "5"
	case "project_cap":
		return "3"
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
