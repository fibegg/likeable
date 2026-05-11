package likeable

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	fibegateway "github.com/fibegg/likeable/internal/fibe"
	projecttext "github.com/fibegg/likeable/internal/project"
)

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		projects, err := s.store.ProjectsForUser(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.recoverProjectsAsync(user.ID, user.Email, projects)
		projectQuota := s.projectQuota(r.Context(), user)
		writeJSON(w, http.StatusOK, map[string]any{"projects": projects, "projectCap": projectQuota["limit"], "projectQuota": projectQuota})
	case http.MethodPost:
		s.handleCreateProject(w, r, user)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request, user *User) {
	var body struct {
		Prompt  string `json:"prompt"`
		Title   string `json:"title"`
		Confirm bool   `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !body.Confirm {
		writeError(w, http.StatusPreconditionRequired, "new project requires confirmation")
		return
	}
	body.Prompt = strings.TrimSpace(body.Prompt)
	usesPaidCredit := false
	if body.Prompt != "" {
		allowed, paid, err := s.messageAllowance(r.Context(), user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !allowed {
			writeError(w, http.StatusPaymentRequired, "message pack required")
			return
		}
		usesPaidCredit = paid
	}
	count, err := s.store.ProjectCountForUser(r.Context(), user.ID)
	if err != nil {
		log.Printf("count projects for user %s: %v", user.ID, err)
		writeError(w, http.StatusInternalServerError, "could not create the project")
		return
	}
	cap := s.projectCapForUser(r.Context(), user)
	if count >= cap {
		writeError(w, http.StatusForbidden, fmt.Sprintf("project cap reached (%d)", cap))
		return
	}
	title := projecttext.CleanTitle(body.Title)
	if title == "" && body.Prompt != "" {
		title = projecttext.TitleFromPrompt(body.Prompt)
	}
	if title == "" {
		title = projecttext.DefaultTitle(count)
	}
	project, err := s.createProjectRecord(r.Context(), user, title)
	if err != nil {
		log.Printf("create project record for user %s: %v", user.ID, err)
		writeError(w, http.StatusInternalServerError, "workspace configuration needs attention")
		return
	}
	if err := s.provisionProjectAsync(user.ID, user.Email, project.ID, body.Prompt); err != nil {
		log.Printf("schedule project provisioning for user %s project %s: %v", user.ID, project.ID, err)
		_ = s.store.DeleteProject(r.Context(), project.ID, user.ID)
		writeError(w, http.StatusInternalServerError, "could not schedule workspace provisioning")
		return
	}
	if body.Prompt != "" {
		msg, _ := s.store.AddMessage(r.Context(), project.ID, "user", body.Prompt)
		if usesPaidCredit && msg != nil {
			_ = s.store.ConsumePaidMessageCredit(r.Context(), user.ID, msg.ID)
		}
		s.notifyMessageQuotaIfNeeded(r.Context(), user)
	}
	s.notifyProjectQuotaIfNeeded(r.Context(), user)
	created, _ := s.store.ProjectForUser(r.Context(), user.ID, project.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"project": created})
}

func (s *Server) handleProjectRoute(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	rest := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	project, err := s.store.ProjectForUser(r.Context(), user.ID, parts[0])
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.recoverProjectAsync(user.ID, user.Email, project)
			writeJSON(w, http.StatusOK, map[string]any{"project": project})
		case http.MethodPatch:
			s.handleProjectUpdate(w, r, user, project)
		case http.MethodDelete:
			s.handleProjectDelete(w, r, user, project)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	switch parts[1] {
	case "messages":
		s.handleProjectMessages(w, r, user, project)
	case "feed":
		s.handleProjectFeed(w, r, user, project)
	case "preview-status":
		s.handleProjectPreviewStatus(w, r, project)
	case "agent":
		if len(parts) == 3 && parts[2] == "interrupt" {
			s.handleProjectAgentInterrupt(w, r, user, project)
			return
		}
		writeError(w, http.StatusNotFound, "not found")
	case "attachments":
		if len(parts) != 3 {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		s.handleProjectAttachment(w, r, project, parts[2])
	case "export":
		s.handleProjectExport(w, r, user, project)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleProjectUpdate(w http.ResponseWriter, r *http.Request, user *User, project *Project) {
	var body struct {
		Title               string `json:"title"`
		SelectedServiceName string `json:"selectedServiceName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	title := projecttext.CleanTitle(body.Title)
	selectedService := strings.TrimSpace(body.SelectedServiceName)
	if title == "" && selectedService == "" {
		writeError(w, http.StatusBadRequest, "project title or service required")
		return
	}
	if title != "" {
		if err := s.store.UpdateProjectTitle(r.Context(), project.ID, user.ID, title); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if selectedService != "" {
		if err := s.store.UpdateProjectSelectedService(r.Context(), project.ID, user.ID, selectedService); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "project service not found")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	updated, err := s.store.ProjectForUser(r.Context(), user.ID, project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": updated})
}

func (s *Server) handleProjectDelete(w http.ResponseWriter, r *http.Request, user *User, project *Project) {
	if project.Status == "deleting" {
		s.deleteProjectResourcesAsync(user.ID, user.Email, project)
		writeJSON(w, http.StatusAccepted, map[string]any{"project": project})
		return
	}
	if err := s.store.UpdateProjectStatus(r.Context(), project.ID, user.ID, "deleting"); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	project.Status = "deleting"
	s.notifyProjectDeletionScheduled(r.Context(), user, project)
	s.deleteProjectResourcesAsync(user.ID, user.Email, project)
	writeJSON(w, http.StatusAccepted, map[string]any{"project": project})
}

func (s *Server) handleProjectAgentInterrupt(w http.ResponseWriter, r *http.Request, user *User, project *Project) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	fibe, err := s.fibeClientForProject(r.Context(), project, user.Email)
	if err != nil {
		log.Printf("interrupt workspace client for project %s: %v", project.ID, err)
		writeError(w, http.StatusServiceUnavailable, "workspace messaging is not configured")
		return
	}
	if err := fibe.Interrupt(r.Context(), project.ConversationID); err != nil {
		log.Printf("interrupt workspace message for project %s: %v", project.ID, err)
		writeError(w, http.StatusBadGateway, "could not stop the workspace agent")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

func (s *Server) handleProjectFeed(w http.ResponseWriter, r *http.Request, user *User, project *Project) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s.recoverProjectAsync(user.ID, user.Email, project)
	if project.Status == "ready" {
		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		if updated, err := s.refreshProjectResourcesIfDue(ctx, user, project); err == nil && updated != nil {
			project = updated
		} else if err != nil {
			log.Printf("refresh project resources for feed %s: %v", project.ID, err)
		}
		cancel()
	}
	local, _ := s.store.MessagesForProject(r.Context(), project.ID)
	fibe, err := s.fibeClientForProject(r.Context(), project, user.Email)
	if err != nil {
		log.Printf("load project feed workspace client for project %s: %v", project.ID, err)
		writeJSON(w, http.StatusOK, map[string]any{"project": project, "localMessages": local, "messages": []any{}, "activity": []any{}, "warning": "Live updates are temporarily unavailable."})
		return
	}
	messages, messagesErr := fibe.Messages(r.Context(), project.ConversationID)
	activity, activityErr := fibe.Activity(r.Context(), project.ConversationID)
	live, liveErr := fibe.ConversationLiveState(r.Context(), project.ConversationID)
	warnings := []string{}
	if messagesErr != nil && !fibegateway.IsConversationMissingError(messagesErr) {
		log.Printf("load project feed messages for project %s: %v", project.ID, messagesErr)
		warnings = append(warnings, "Workspace messages are temporarily unavailable.")
		messages = []any{}
	}
	if activityErr != nil && !fibegateway.IsConversationMissingError(activityErr) {
		log.Printf("load project feed activity for project %s: %v", project.ID, activityErr)
		warnings = append(warnings, "Workspace activity is temporarily unavailable.")
		activity = []any{}
	}
	if liveErr != nil {
		if !fibegateway.IsConversationMissingError(liveErr) {
			log.Printf("load project feed live state for project %s: %v", project.ID, liveErr)
			warnings = append(warnings, "Live workspace status is temporarily unavailable.")
		}
		live = &fibegateway.ConversationLiveState{ConversationID: project.ConversationID, IsProcessing: false, StreamText: "", QueuedTurns: 0}
	}
	if messages == nil {
		messages = []any{}
	}
	if activity == nil {
		activity = []any{}
	}
	messages = sanitizeAgentProtocolMessages(messages)
	sanitizeAgentProtocolLiveState(live)
	response := map[string]any{"project": project, "localMessages": local, "messages": messages, "activity": activity, "live": live}
	if len(warnings) > 0 {
		response["warning"] = strings.Join(warnings, " ")
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleProjectPreviewStatus(w http.ResponseWriter, r *http.Request, project *Project) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user := userFromContext(r.Context())
	if projectNeedsReadinessRecovery(project) && user != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		updated, err := s.refreshProjectReadiness(ctx, user, project)
		cancel()
		if err == nil && updated != nil {
			project = updated
		} else {
			log.Printf("preview status recovery for project %s is still pending: %v", project.ID, err)
			s.recoverProjectAsync(user.ID, user.Email, project)
		}
	}
	if strings.TrimSpace(project.PreviewURL) == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ready":     false,
			"status":    publicPreviewProbeStatus(project.Status),
			"checkedAt": nowString(),
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_, ready, status, maintenance, err := s.promoteProjectFromReachablePreview(ctx, project.UserID, project)
	if err != nil {
		log.Printf("preview status probe for project %s failed: %v", project.ID, err)
		ready = false
		maintenance = false
		status = "starting"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ready":       ready,
		"maintenance": maintenance,
		"status":      publicPreviewProbeStatus(status),
		"checkedAt":   nowString(),
	})
}

func publicPreviewProbeStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "starting"
	}
	code := strings.Fields(status)
	if len(code) == 0 || len(code[0]) != 3 || code[0][0] < '0' || code[0][0] > '9' || code[0][1] < '0' || code[0][1] > '9' || code[0][2] < '0' || code[0][2] > '9' {
		return "starting"
	}
	if code[0] == "404" {
		return "starting"
	}
	return status
}
