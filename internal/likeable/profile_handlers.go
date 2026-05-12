package likeable

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Server) handleProfileDeleteAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user := userFromContext(r.Context())
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if normalizeEmail(body.Email) == "" || normalizeEmail(body.Email) != normalizeEmail(user.Email) {
		writeError(w, http.StatusBadRequest, "type your signed-in email to confirm deletion")
		return
	}
	projects, err := s.store.AllProjectsForUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cleanupCtx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	for i := range projects {
		project := &projects[i]
		if !projectHasFibeResources(project) {
			continue
		}
		fibe, err := s.completeProjectResourceSnapshot(cleanupCtx, user.Email, project)
		if err != nil {
			log.Printf("delete all configure workspace cleanup for project %s: %v", project.ID, err)
			writeError(w, http.StatusBadGateway, "could not configure workspace cleanup")
			return
		}
		if err := fibe.DeleteProjectResources(cleanupCtx, project); err != nil {
			log.Printf("delete all workspace cleanup for project %s: %v", project.ID, err)
			writeError(w, http.StatusBadGateway, "could not delete all workspace resources")
			return
		}
	}
	if err := s.deleteLocalProjectAttachmentDirs(projects); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.store.RemoveEmailFromSignupAllowlist(r.Context(), user.Email); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.store.DeleteUser(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "likeable_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) handleProfileArchives(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	rest := strings.TrimPrefix(r.URL.Path, "/api/profile/archives")
	if rest == "" || rest == "/" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		archives, err := s.store.ArchivesForUser(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for i := range archives {
			archives[i].DownloadURL = s.archiveDownloadURL(archives[i].ID)
		}
		writeJSON(w, http.StatusOK, map[string]any{"archives": archives})
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[1] != "download" || r.Method != http.MethodGet {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	archive, err := s.store.ArchiveForUser(r.Context(), user.ID, parts[0])
	if err != nil {
		writeError(w, http.StatusNotFound, "archive not found")
		return
	}
	fullPath := filepath.Join(s.store.DataDir(), archive.StoragePath)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", cleanAttachmentFilename(archive.ProjectTitle)+".zip"))
	http.ServeFile(w, r, fullPath)
}

func projectHasFibeResources(project *Project) bool {
	return strings.TrimSpace(project.PlaygroundID) != "" ||
		strings.TrimSpace(project.PlaygroundName) != "" ||
		strings.TrimSpace(project.PlayspecID) != "" ||
		strings.TrimSpace(project.PropID) != "" ||
		strings.TrimSpace(project.RepoURL) != "" ||
		strings.TrimSpace(project.PreviewURL) != "" ||
		strings.TrimSpace(project.ConversationID) != "" ||
		len(project.Repositories) > 0 ||
		len(project.Services) > 0
}

func (s *Server) deleteLocalProjectAttachmentDirs(projects []Project) error {
	base := filepath.Clean(s.store.DataDir())
	attachmentRoot := filepath.Join(base, "attachments")
	for _, project := range projects {
		id := strings.TrimSpace(project.ID)
		if id == "" || filepath.IsAbs(id) || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
			return fmt.Errorf("unsafe project attachment path for %s", project.ID)
		}
		target := filepath.Clean(filepath.Join(attachmentRoot, id))
		if !strings.HasPrefix(target, attachmentRoot+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe project attachment path for %s", project.ID)
		}
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("delete local attachments for project %s: %w", project.ID, err)
		}
	}
	return nil
}
