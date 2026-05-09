package likeable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	fibegateway "github.com/fibegg/likeable/internal/fibe"
	projecttext "github.com/fibegg/likeable/internal/project"
	"github.com/google/uuid"
)

func (s *Server) handleProjectMessages(w http.ResponseWriter, r *http.Request, user *User, project *Project) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	text, attachmentHeaders, busyPolicy, err := parseProjectMessageRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	text = strings.TrimSpace(text)
	if text == "" && len(attachmentHeaders) == 0 {
		writeError(w, http.StatusBadRequest, "text or attachment is required")
		return
	}
	if len(attachmentHeaders) > maxMessageAttachments {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("too many attachments; max %d", maxMessageAttachments))
		return
	}
	if project.Status != "ready" && projectNeedsReadinessRecovery(project) {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		updated, err := s.refreshProjectReadiness(ctx, user, project)
		cancel()
		if err == nil && updated != nil {
			project = updated
		} else {
			log.Printf("message readiness recovery for project %s is still pending: %v", project.ID, err)
			s.recoverProjectAsync(user.ID, user.Email, project)
		}
	}
	if project.Status != "ready" && strings.TrimSpace(project.PreviewURL) != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		updated, ready, _, err := s.promoteProjectFromReachablePreview(ctx, user.ID, project)
		cancel()
		if err == nil && ready && updated != nil {
			project = updated
		}
	}
	if project.Status != "ready" || project.PreviewURL == "" {
		writeError(w, http.StatusConflict, "canvas is still starting")
		return
	}
	if strings.TrimSpace(project.PlaygroundID) != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		if updated, err := s.refreshProjectResourcesNow(ctx, user, project); err == nil && updated != nil {
			project = updated
		} else if err != nil {
			log.Printf("refresh project resources before message %s: %v", project.ID, err)
		}
		cancel()
	}
	allowed, usesPaidCredit, err := s.messageAllowance(r.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !allowed {
		writeError(w, http.StatusPaymentRequired, "message pack required")
		return
	}
	fibe, err := s.fibeClientForProject(r.Context(), project, user.Email)
	if err != nil {
		log.Printf("message workspace client for project %s: %v", project.ID, err)
		writeError(w, http.StatusServiceUnavailable, "workspace messaging is not configured")
		return
	}
	messageID := uuid.NewString()
	localAttachments, cleanupLocalAttachments, err := saveLocalMessageAttachments(s.store.DataDir(), project.ID, messageID, attachmentHeaders)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	attachmentPaths, err := messageAttachmentPaths(s.store.DataDir(), localAttachments)
	if err != nil {
		cleanupLocalAttachments()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	agentText := text
	if agentText == "" {
		agentText = "Review the attached file(s) and update the playground accordingly."
	}
	msg, err := s.store.AddMessageWithAttachments(r.Context(), &Message{
		ID:        messageID,
		ProjectID: project.ID,
		Role:      "user",
		Body:      text,
		CreatedAt: nowString(),
	}, localAttachments)
	if err != nil {
		cleanupLocalAttachments()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := fibe.SendMessage(r.Context(), project.ConversationID, projecttext.AgentPrompt(project, agentText), attachmentPaths, busyPolicy); err != nil {
		cleanupLocalAttachments()
		_ = s.store.DeleteMessage(context.Background(), project.ID, messageID)
		log.Printf("send workspace message for project %s: %v", project.ID, err)
		status, message := workspaceSendFailureResponse(localAttachments, err)
		writeError(w, status, message)
		return
	}
	if usesPaidCredit {
		if err := s.store.ConsumePaidMessageCredit(r.Context(), user.ID, msg.ID); err != nil {
			writeError(w, http.StatusPaymentRequired, err.Error())
			return
		}
	}
	s.notifyMessageQuotaIfNeeded(r.Context(), user)
	writeJSON(w, http.StatusAccepted, map[string]any{"message": msg})
}

func parseProjectMessageRequest(w http.ResponseWriter, r *http.Request) (string, []*multipart.FileHeader, string, error) {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxMessageUploadBytes)
		if err := r.ParseMultipartForm(maxMessageUploadBytes); err != nil {
			return "", nil, "", fmt.Errorf("invalid multipart form: %w", err)
		}
		var headers []*multipart.FileHeader
		if r.MultipartForm != nil {
			headers = append(headers, r.MultipartForm.File["attachments"]...)
			headers = append(headers, r.MultipartForm.File["file"]...)
		}
		return r.FormValue("text"), headers, normalizeBusyPolicy(r.FormValue("busy_policy")), nil
	}
	var body struct {
		Text            string `json:"text"`
		BusyPolicy      string `json:"busy_policy"`
		BusyPolicyCamel string `json:"busyPolicy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return "", nil, "", errors.New("invalid json")
	}
	return body.Text, nil, normalizeBusyPolicy(firstNonEmpty(body.BusyPolicy, body.BusyPolicyCamel)), nil
}

func normalizeBusyPolicy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "reject", "queue", "steer":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "queue"
	}
}

func workspaceSendFailureResponse(attachments []MessageAttachment, err error) (int, string) {
	if len(attachments) > 0 && workspaceAttachmentFailure(err) {
		return http.StatusBadRequest, unsupportedAttachmentMessage(attachments)
	}
	return http.StatusBadGateway, "could not send the request to the workspace"
}

func workspaceAttachmentFailure(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	var platformErr *fibegateway.PlatformError
	if errors.As(err, &platformErr) {
		text = strings.ToLower(strings.Join([]string{
			platformErr.Code,
			platformErr.Message,
			platformErr.Stderr,
			err.Error(),
		}, " "))
	}
	for _, needle := range []string{
		"unsupported or blocked file type",
		"unsupported file type",
		"blocked file type",
		"upload attachment",
		"/uploads",
	} {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func unsupportedAttachmentMessage(attachments []MessageAttachment) string {
	for _, attachment := range attachments {
		if strings.EqualFold(cleanAttachmentExtension(attachment.Filename), ".webp") {
			return "The workspace rejected this WEBP attachment. Convert the image to PNG or JPG and try again."
		}
	}
	return "One of the attached files is not supported by this workspace. Try PNG, JPG, GIF, PDF, ZIP, text, CSV, Markdown, JSON, Word, or Excel."
}

func messageAttachmentPaths(dataDir string, attachments []MessageAttachment) ([]string, error) {
	paths := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		path := filepath.Join(dataDir, attachment.StoragePath)
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("attachment %s is not available", attachment.Filename)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func saveLocalMessageAttachments(dataDir, projectID, messageID string, headers []*multipart.FileHeader) ([]MessageAttachment, func(), error) {
	if len(headers) == 0 {
		return nil, func() {}, nil
	}
	baseDir := filepath.Join(dataDir, "attachments", projectID, messageID)
	cleanup := func() { _ = os.RemoveAll(baseDir) }
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, cleanup, fmt.Errorf("create attachment directory: %w", err)
	}
	attachments := make([]MessageAttachment, 0, len(headers))
	for _, header := range headers {
		if header == nil {
			continue
		}
		attachmentID := uuid.NewString()
		displayName := cleanAttachmentFilename(header.Filename)
		ext := cleanAttachmentExtension(displayName)
		relativePath := filepath.Join("attachments", projectID, messageID, attachmentID+ext)
		fullPath := filepath.Join(dataDir, relativePath)
		src, err := header.Open()
		if err != nil {
			cleanup()
			return nil, cleanup, fmt.Errorf("open attachment %s: %w", displayName, err)
		}
		dst, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			_ = src.Close()
			cleanup()
			return nil, cleanup, fmt.Errorf("save attachment %s: %w", displayName, err)
		}
		size, copyErr := io.Copy(dst, src)
		closeDstErr := dst.Close()
		closeSrcErr := src.Close()
		if copyErr != nil {
			cleanup()
			return nil, cleanup, fmt.Errorf("save attachment %s: %w", displayName, copyErr)
		}
		if closeDstErr != nil {
			cleanup()
			return nil, cleanup, fmt.Errorf("close attachment %s: %w", displayName, closeDstErr)
		}
		if closeSrcErr != nil {
			cleanup()
			return nil, cleanup, fmt.Errorf("close attachment %s: %w", displayName, closeSrcErr)
		}
		attachments = append(attachments, MessageAttachment{
			ID:          attachmentID,
			MessageID:   messageID,
			ProjectID:   projectID,
			Filename:    displayName,
			ContentType: strings.TrimSpace(header.Header.Get("Content-Type")),
			Size:        size,
			StoragePath: relativePath,
			CreatedAt:   nowString(),
		})
	}
	return attachments, cleanup, nil
}

func cleanAttachmentFilename(filename string) string {
	name := strings.TrimSpace(filepath.Base(filename))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "attachment"
	}
	return name
}

func cleanAttachmentExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if len(ext) > 16 || strings.ContainsAny(ext, `/\`) {
		return ""
	}
	return ext
}

func (s *Server) handleProjectAttachment(w http.ResponseWriter, r *http.Request, project *Project, attachmentID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	attachment, err := s.store.AttachmentForProject(r.Context(), project.ID, attachmentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}
	fullPath := filepath.Join(s.store.DataDir(), attachment.StoragePath)
	if attachment.ContentType != "" {
		w.Header().Set("Content-Type", attachment.ContentType)
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", attachment.Filename))
	http.ServeFile(w, r, fullPath)
}
