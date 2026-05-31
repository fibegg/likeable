package likeable

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fibegg/likeable/internal/store"
)

func TestProjectPreviewServesLocalWorkspaceAndBlocksReservedPaths(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "pilot@example.com", "Pilot", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-preview",
		UserID:         user.ID,
		Title:          "Preview",
		ConversationID: "conversation-preview",
		PlaygroundID:   "local-project-preview",
		PlaygroundName: "preview",
		Status:         "ready",
		PreviewURL:     "http://example.test/api/projects/project-preview/preview/",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	client, err := server.workspaceClientForProject(t.Context(), project, user.Email)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateGreenfield(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(client.WorkspaceDir(project.ID), ".likeable", "private.txt"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/project-preview/preview/", nil)
	rec := httptest.NewRecorder()
	server.handleProjectRoute(rec, req.WithContext(context.WithValue(req.Context(), userContextKey, user)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Likeable standalone") {
		t.Fatalf("preview returned %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/projects/project-preview/preview/.likeable/private.txt", nil)
	rec = httptest.NewRecorder()
	server.handleProjectRoute(rec, req.WithContext(context.WithValue(req.Context(), userContextKey, user)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("reserved preview path returned %d, want %d", rec.Code, http.StatusBadRequest)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/projects/project-preview/previewish", nil)
	rec = httptest.NewRecorder()
	server.handleProjectRoute(rec, req.WithContext(context.WithValue(req.Context(), userContextKey, user)))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("previewish returned %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestZipDirectorySkipsWorkspaceMetadata(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "index.html"), []byte("<h1>ok</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(source, ".likeable"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".likeable", "messages.jsonl"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "archive.zip")
	if err := zipDirectory(target, source); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(target)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	foundIndex := false
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, ".likeable/") {
			t.Fatalf("metadata file leaked into archive: %s", file.Name)
		}
		if file.Name == "index.html" {
			foundIndex = true
		}
	}
	if !foundIndex {
		t.Fatal("index.html missing from archive")
	}
}
