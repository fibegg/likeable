package likeable

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fibegg/likeable/internal/fibe"
	projecttext "github.com/fibegg/likeable/internal/project"
	"github.com/fibegg/likeable/internal/store"
)

type captureEmailSender struct {
	ch chan emailMessage
}

func (c captureEmailSender) Send(_ context.Context, _ smtpSettings, message emailMessage) error {
	c.ch <- message
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testStripeSignature(secret string, body []byte) string {
	timestamp := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return "t=" + strconv.FormatInt(timestamp, 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestHealthzChecksSQLite(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("healthz returned %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBootstrapConfigRejectsMissingOrWrongToken(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	body := `{"google_client_id":"client","google_client_secret":"secret"}`
	for _, tc := range []struct {
		name   string
		token  string
		header string
		status int
	}{
		{name: "disabled", token: "", header: "Bearer deploy-token", status: http.StatusNotFound},
		{name: "placeholder", token: "placeholder", header: "Bearer placeholder", status: http.StatusNotFound},
		{name: "missing", token: "deploy-token", header: "", status: http.StatusUnauthorized},
		{name: "wrong", token: "deploy-token", header: "Bearer wrong-token", status: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test", BootstrapToken: tc.token}, http: http.DefaultClient}
			req := httptest.NewRequest(http.MethodPost, "/api/bootstrap/config", strings.NewReader(body))
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()

			server.routes().ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("bootstrap returned %d, want %d; body=%s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}

func TestBootstrapConfigWritesGoogleConfigWithoutExposingSecret(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test", BootstrapToken: "deploy-token"}, http: http.DefaultClient}
	req := httptest.NewRequest(http.MethodPost, "/api/bootstrap/config", strings.NewReader(`{"google_client_id":"client-id","google_client_secret":"client-secret","signup_mode":"all"}`))
	req.Header.Set("Authorization", "Bearer deploy-token")
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap returned %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "client-secret") || strings.Contains(rec.Body.String(), "google_client_secret") {
		t.Fatalf("bootstrap response exposed secret material: %s", rec.Body.String())
	}
	cfg, err := store.ConfigMap(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if cfg["google_client_id"] != "client-id" || cfg["google_client_secret"] != "client-secret" || cfg["signup_mode"] != "all" {
		t.Fatalf("stored config=%+v, want google config and signup mode", cfg)
	}
	public := publicAdminConfig(cfg)
	entry := public["google_client_secret"].(map[string]any)
	if !entry["secret"].(bool) || !entry["set"].(bool) || entry["value"].(string) != "" {
		t.Fatalf("google secret public entry=%+v, want write-only secret", entry)
	}
	meReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meRec := httptest.NewRecorder()
	server.routes().ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me returned %d: %s", meRec.Code, meRec.Body.String())
	}
	var me struct {
		Auth struct {
			GoogleConfigured bool   `json:"googleConfigured"`
			DevAuth          bool   `json:"devAuth"`
			SignupMode       string `json:"signupMode"`
		} `json:"auth"`
	}
	if err := json.NewDecoder(meRec.Body).Decode(&me); err != nil {
		t.Fatal(err)
	}
	if !me.Auth.GoogleConfigured || me.Auth.DevAuth || me.Auth.SignupMode != "all" {
		t.Fatalf("me auth=%+v, want configured google, no dev auth, signup all", me.Auth)
	}
}

func TestBootstrapConfigRejectsAfterUserExists(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.UpsertUser(t.Context(), "admin@example.com", "Admin", ""); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test", BootstrapToken: "deploy-token"}, http: http.DefaultClient}
	req := httptest.NewRequest(http.MethodPost, "/api/bootstrap/config", strings.NewReader(`{"google_client_id":"client-id","google_client_secret":"client-secret"}`))
	req.Header.Set("Authorization", "Bearer deploy-token")
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("bootstrap returned %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	cfg, err := store.ConfigMap(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if cfg["google_client_id"] != "" || cfg["google_client_secret"] != "" {
		t.Fatalf("bootstrap wrote config after user existed: %+v", cfg)
	}
}

func TestBootstrapConfigIsOneTime(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test", BootstrapToken: "deploy-token"}, http: http.DefaultClient}
	first := httptest.NewRequest(http.MethodPost, "/api/bootstrap/config", strings.NewReader(`{"google_client_id":"client-id","google_client_secret":"client-secret"}`))
	first.Header.Set("Authorization", "Bearer deploy-token")
	firstRec := httptest.NewRecorder()
	server.routes().ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first bootstrap returned %d, want 200; body=%s", firstRec.Code, firstRec.Body.String())
	}
	second := httptest.NewRequest(http.MethodPost, "/api/bootstrap/config", strings.NewReader(`{"google_client_id":"other","google_client_secret":"other-secret"}`))
	second.Header.Set("Authorization", "Bearer deploy-token")
	secondRec := httptest.NewRecorder()
	server.routes().ServeHTTP(secondRec, second)
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("second bootstrap returned %d, want 409; body=%s", secondRec.Code, secondRec.Body.String())
	}
	cfg, err := store.ConfigMap(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if cfg["google_client_id"] != "client-id" || cfg["google_client_secret"] != "client-secret" {
		t.Fatalf("bootstrap was overwritten: %+v", cfg)
	}
}

func TestProjectEndpointsEnforceUserOwnership(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test", AdminEmail: "admin@example.com"}, http: http.DefaultClient}

	userA, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	userB, _ := store.UpsertUser(t.Context(), "b@example.com", "B", "")
	if err := store.CreateSession(t.Context(), userA.ID, "token-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(t.Context(), userB.ID, "token-b", time.Hour); err != nil {
		t.Fatal(err)
	}
	projectB := &Project{
		ID:             "project-b",
		UserID:         userB.ID,
		Title:          "B project",
		ConversationID: "likeable-secret-conversation",
		Status:         "ready",
	}
	if err := store.CreateProject(t.Context(), projectB); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/projects/project-b", ""},
		{http.MethodPatch, "/api/projects/project-b", `{"title":"stolen"}`},
		{http.MethodGet, "/api/projects/project-b/feed", ""},
		{http.MethodGet, "/api/projects/project-b/preview-status", ""},
		{http.MethodPost, "/api/projects/project-b/messages", `{"text":"steal"}`},
		{http.MethodPost, "/api/projects/project-b/export", `{"repoName":"steal"}`},
		{http.MethodDelete, "/api/projects/project-b", ""},
		{http.MethodGet, "/api/projects/likeable-secret-conversation/feed", ""},
		{http.MethodGet, "/api/projects/likeable-secret-conversation/preview-status", ""},
		{http.MethodPost, "/api/projects/likeable-secret-conversation/messages", `{"text":"steal"}`},
		{http.MethodPost, "/api/projects/likeable-secret-conversation/export", `{"repoName":"steal"}`},
		{http.MethodPatch, "/api/projects/likeable-secret-conversation", `{"title":"stolen"}`},
		{http.MethodDelete, "/api/projects/likeable-secret-conversation", ""},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
		rec := httptest.NewRecorder()

		server.routes().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s returned %d, want 404; body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestProjectTitleCanBeUpdatedByOwner(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	user, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	if err := store.CreateSession(t.Context(), user.ID, "token-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-a",
		UserID:         user.ID,
		Title:          "Old name",
		ConversationID: "likeable-a",
		Status:         "ready",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/projects/project-a", strings.NewReader(`{"title":"  New project name  "}`))
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("rename returned %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Title != "New project name" {
		t.Fatalf("title=%q, want cleaned title", stored.Title)
	}
}

func TestProjectResponsesDoNotExposePlatformInternals(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	user, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	if err := store.CreateSession(t.Context(), user.ID, "token-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-a",
		UserID:         user.ID,
		Title:          "A project",
		ConversationID: "likeable-secret-conversation",
		AgentID:        "agent-1",
		MarqueeID:      "runner-1",
		PlaygroundID:   "workspace-1",
		PlayspecID:     "spec-1",
		PropID:         "prop-1",
		RepoURL:        "https://server.example.test/source/private.git",
		PreviewURL:     "https://preview.example.test",
		Status:         "ready",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/api/projects", "/api/projects/project-a"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
		rec := httptest.NewRecorder()

		server.routes().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s returned %d, want 200; body=%s", path, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, token := range []string{
			"conversationId",
			"agentId",
			"marqueeId",
			"playgroundId",
			"playspecId",
			"propId",
			"repoUrl",
			"UserID",
			"ConversationID",
			"server.example.test/source",
		} {
			if strings.Contains(body, token) {
				t.Fatalf("%s response leaks %q: %s", path, token, body)
			}
		}
		if !strings.Contains(body, "previewUrl") {
			t.Fatalf("%s response should still include public previewUrl: %s", path, body)
		}
	}
}

func TestDeletingProjectFreesProjectQuota(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	project := &Project{
		ID:             "project-a",
		UserID:         user.ID,
		Title:          "A project",
		ConversationID: "likeable-a",
		Status:         "ready",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateProjectStatus(t.Context(), project.ID, user.ID, "deleting"); err != nil {
		t.Fatal(err)
	}
	count, err := store.ProjectCountForUser(t.Context(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("count=%d, want 0", count)
	}
	projects, err := store.ProjectsForUser(t.Context(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("listed projects=%d, want 0", len(projects))
	}
	deleting, err := store.DeletingProjects(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleting) != 1 || deleting[0].ID != project.ID {
		t.Fatalf("deleting projects=%+v, want hidden deletion row", deleting)
	}
	if err := store.UpdateProjectProvisioning(t.Context(), project.ID, user.ID, "playground", "playspec", "prop", "repo", "preview", "ready"); err == nil {
		t.Fatal("provisioning update should not resurrect a deleting project")
	}
	if err := store.UpdateProjectStatus(t.Context(), project.ID, user.ID, "launching"); err == nil {
		t.Fatal("non-deleting status update should not resurrect a deleting project")
	}
	if err := store.UpdateProjectError(t.Context(), project.ID, user.ID, "failed"); err == nil {
		t.Fatal("error update should not resurrect a deleting project")
	}
	stillDeleting, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stillDeleting.Status != "deleting" {
		t.Fatalf("status=%q, want deleting", stillDeleting.Status)
	}
}

func TestProfileDeleteAllRequiresEmailConfirmation(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	user, err := store.UpsertUser(t.Context(), "pilot@example.com", "Pilot", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(t.Context(), user.ID, "delete-token", time.Hour); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/profile/delete-all", strings.NewReader(`{"email":"other@example.com"}`))
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "delete-token"})
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("delete-all returned %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if _, err := store.UserByID(t.Context(), user.ID); err != nil {
		t.Fatalf("user was deleted without matching email confirmation: %v", err)
	}
}

func TestProjectErrorsAreSanitized(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	project := &Project{
		ID:             "project-error",
		UserID:         user.ID,
		Title:          "A project",
		ConversationID: "likeable-error",
		Status:         "creating",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	raw := "Fibe server request failed: 404 Not Found"
	if err := store.UpdateProjectError(t.Context(), project.ID, user.ID, raw); err != nil {
		t.Fatal(err)
	}
	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	leaked := []string{"Fibe", "/api/", "conversations", "404", "Not Found"}
	for _, token := range leaked {
		if strings.Contains(stored.ErrorMessage, token) {
			t.Fatalf("stored error %q leaks %q", stored.ErrorMessage, token)
		}
	}
	if stored.ErrorMessage == "" || stored.ErrorMessage == raw {
		t.Fatalf("stored error=%q, want sanitized user-facing message", stored.ErrorMessage)
	}
}

func TestProjectPreviewStatusKeepsPlatform404BehindPlaceholder(t *testing.T) {
	previewServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not ready", http.StatusNotFound)
	}))
	defer previewServer.Close()
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: previewServer.Client()}
	user, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	if err := store.CreateSession(t.Context(), user.ID, "token-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-preview",
		UserID:         user.ID,
		Title:          "Preview",
		ConversationID: "conv-preview",
		PreviewURL:     previewServer.URL,
		Status:         "ready",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/project-preview/preview-status", nil)
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("preview-status returned %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Ready  bool   `json:"ready"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Ready {
		t.Fatal("404 preview must not be marked ready")
	}
	if body.Status != "starting" {
		t.Fatalf("status=%q, want sanitized starting", body.Status)
	}
}

func TestProjectPreviewStatusRecoversErroredProjectWithResources(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	user, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	if err := store.CreateSession(t.Context(), user.ID, "token-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-preview-recover",
		UserID:         user.ID,
		Title:          "Preview",
		ConversationID: "conv-preview",
		PlaygroundID:   "playground-1",
		PreviewURL:     "http://preview.example.test",
		Status:         "error",
		ErrorMessage:   "The canvas could not start.",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/project-preview-recover/preview-status", nil)
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("preview-status returned %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Ready  bool   `json:"ready"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Ready || body.Status != "starting" {
		t.Fatalf("body=%+v, want not ready and retryable starting state", body)
	}
}

func TestProjectPreviewStatusPromotesReachablePreviewWithoutPlatformConfig(t *testing.T) {
	previewServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><title>ready</title>"))
	}))
	defer previewServer.Close()
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: previewServer.Client()}
	user, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	if err := store.CreateSession(t.Context(), user.ID, "token-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-preview-reachable-no-platform",
		UserID:         user.ID,
		Title:          "Preview",
		ConversationID: "conv-preview",
		PlaygroundID:   "playground-1",
		PreviewURL:     previewServer.URL,
		Status:         "launching",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/project-preview-reachable-no-platform/preview-status", nil)
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("preview-status returned %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Ready  bool   `json:"ready"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Ready || body.Status != "200 OK" {
		t.Fatalf("body=%+v, want ready preview", body)
	}
	updated, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "ready" {
		t.Fatalf("status=%q, want ready", updated.Status)
	}
}

func TestProjectPreviewStatusMarksReachablePreviewReady(t *testing.T) {
	previewServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><title>ready</title>"))
	}))
	defer previewServer.Close()
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: previewServer.Client()}
	user, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	if err := store.CreateSession(t.Context(), user.ID, "token-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-preview-ready",
		UserID:         user.ID,
		Title:          "Preview",
		ConversationID: "conv-preview",
		PreviewURL:     previewServer.URL,
		Status:         "ready",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/project-preview-ready/preview-status", nil)
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("preview-status returned %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Ready  bool   `json:"ready"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Ready {
		t.Fatalf("ready=false status=%q", body.Status)
	}
	if body.Status != "200 OK" {
		t.Fatalf("status=%q, want 200 OK", body.Status)
	}
}

func TestProjectPreviewStatusPromotesLaunchingReadyWorkspace(t *testing.T) {
	previewServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><title>ready</title>"))
	}))
	defer previewServer.Close()
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fakeFibeCLI(t)
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"fibe_base_url": "server.test:3000",
		"fibe_api_key":  "test-key",
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: previewServer.Client()}
	user, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	if err := store.CreateSession(t.Context(), user.ID, "token-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-preview-promote",
		UserID:         user.ID,
		Title:          "Preview",
		ConversationID: "conv-preview",
		AgentID:        "agent-1",
		PlaygroundID:   "123",
		PreviewURL:     previewServer.URL,
		Status:         "launching",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/project-preview-promote/preview-status", nil)
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("preview-status returned %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Ready  bool   `json:"ready"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Ready || body.Status != "200 OK" {
		t.Fatalf("body=%+v, want ready preview", body)
	}
	updated, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "ready" {
		t.Fatalf("status=%q, want ready", updated.Status)
	}
}

func TestProjectMessagePromotesLaunchingReadyWorkspace(t *testing.T) {
	previewServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><title>ready</title>"))
	}))
	defer previewServer.Close()
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, _, stdinPath := fakeFibeCLI(t)
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"fibe_base_url": "server.test:3000",
		"fibe_api_key":  "test-key",
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: previewServer.Client()}
	user, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	if err := store.CreateSession(t.Context(), user.ID, "token-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-message-promote",
		UserID:         user.ID,
		Title:          "Preview",
		ConversationID: "conv-preview",
		AgentID:        "agent-1",
		PlaygroundID:   "123",
		PreviewURL:     previewServer.URL,
		Status:         "launching",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/projects/project-message-promote/messages", strings.NewReader(`{"text":"change the heading"}`))
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("message returned %d: %s", rec.Code, rec.Body.String())
	}
	updated, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "ready" {
		t.Fatalf("status=%q, want ready", updated.Status)
	}
	if !strings.Contains(readFile(t, stdinPath), "change the heading") {
		t.Fatalf("agent prompt was not sent: %s", readFile(t, stdinPath))
	}
}

func TestProjectFeedTriggersReadinessRecovery(t *testing.T) {
	previewServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><title>ready</title>"))
	}))
	defer previewServer.Close()
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cliPath, _, _ := fakeFibeCLI(t)
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"fibe_base_url": "server.test:3000",
		"fibe_api_key":  "test-key",
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: previewServer.Client()}
	user, _ := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	if err := store.CreateSession(t.Context(), user.ID, "token-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-feed-recover",
		UserID:         user.ID,
		Title:          "Preview",
		ConversationID: "conv-preview",
		AgentID:        "agent-1",
		PlaygroundID:   "123",
		PreviewURL:     previewServer.URL,
		Status:         "launching",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(cliPath)+string(os.PathListSeparator)+os.Getenv("PATH"))

	req := httptest.NewRequest(http.MethodGet, "/api/projects/project-feed-recover/feed", nil)
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed returned %d: %s", rec.Code, rec.Body.String())
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		updated, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Status == "ready" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	updated, _ := store.ProjectForUser(t.Context(), user.ID, project.ID)
	t.Fatalf("project status=%q, want recovered ready", updated.Status)
}

func TestProfileDeleteAllDeletesFibeResourcesAndLocalData(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cliPath, logPath, _ := fakeFibeCLI(t)
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"fibe_base_url":         "server.test:3000",
		"fibe_api_key":          "test-key",
		"fibe_cli_path":         cliPath,
		"signup_allowed_emails": "pilot@example.com\n@trusted.test",
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	user, err := store.UpsertUser(t.Context(), "pilot@example.com", "Pilot", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(t.Context(), user.ID, "delete-token", time.Hour); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-delete-all",
		UserID:         user.ID,
		Title:          "Delete all",
		ConversationID: "conv-delete-all",
		AgentID:        "agent-1",
		PlaygroundID:   "playground-1",
		PlayspecID:     "playspec-1",
		PropID:         "prop-1",
		Status:         "ready",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddMessage(t.Context(), project.ID, "user", "hello"); err != nil {
		t.Fatal(err)
	}
	attachmentDir := filepath.Join(store.DataDir(), "attachments", project.ID, "message-1")
	if err := os.MkdirAll(attachmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDir, "attachment.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/profile/delete-all", strings.NewReader(`{"email":"PILOT@example.com"}`))
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "delete-token"})
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("delete-all returned %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	log := readFile(t, logPath)
	for _, want := range []string{
		"playgrounds delete playground-1",
		"playspecs delete playspec-1",
		"props delete prop-1",
		"agents delete-conversation agent-1 --conversation-id conv-delete-all",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("missing CLI command %q; log=%s", want, log)
		}
	}
	if _, err := store.UserByID(t.Context(), user.ID); err == nil {
		t.Fatal("user still exists after delete-all")
	}
	if _, err := os.Stat(filepath.Join(store.DataDir(), "attachments", project.ID)); !os.IsNotExist(err) {
		t.Fatalf("attachment directory still exists or stat failed unexpectedly: %v", err)
	}
	projectRows, err := store.ProjectCountForUser(t.Context(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if projectRows != 0 {
		t.Fatalf("project rows=%d, want 0", projectRows)
	}
	if _, err := store.UserBySessionToken(t.Context(), "delete-token"); err == nil {
		t.Fatal("session still resolves after user deletion")
	}
	cfg, err := store.ConfigMap(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cfg["signup_allowed_emails"], "pilot@example.com") || !strings.Contains(cfg["signup_allowed_emails"], "@trusted.test") {
		t.Fatalf("allowlist=%q, want deleted email removed and other entries preserved", cfg["signup_allowed_emails"])
	}
}

func TestProfileDeleteAllKeepsLocalDataWhenFibeCleanupFails(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"fibe_base_url": "server.test:3000",
		"fibe_api_key":  "test-key",
		"fibe_cli_path": "/does/not/exist",
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	user, err := store.UpsertUser(t.Context(), "pilot@example.com", "Pilot", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(t.Context(), user.ID, "delete-token", time.Hour); err != nil {
		t.Fatal(err)
	}
	project := &Project{ID: "project-delete-all", UserID: user.ID, Title: "Delete all", ConversationID: "conv-delete-all", AgentID: "agent-1", PlaygroundID: "playground-1", Status: "ready"}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/profile/delete-all", strings.NewReader(`{"email":"pilot@example.com"}`))
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "delete-token"})
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("delete-all returned %d, want 502; body=%s", rec.Code, rec.Body.String())
	}
	if _, err := store.UserByID(t.Context(), user.ID); err != nil {
		t.Fatalf("user should remain when remote cleanup fails: %v", err)
	}
	if _, err := store.ProjectForUser(t.Context(), user.ID, project.ID); err != nil {
		t.Fatalf("project should remain when remote cleanup fails: %v", err)
	}
}

func TestAgentProjectPromptIncludesTargetContext(t *testing.T) {
	project := &Project{
		ID:             "project-1",
		Title:          "Starter",
		ConversationID: "likeable-project-1",
		PlaygroundID:   "10",
		RepoURL:        "http://gitea.test/owner/repo",
		PreviewURL:     "http://starter.test",
	}
	prompt := projecttext.AgentPrompt(project, "Change the heading")
	for _, want := range []string{
		"target Fibe playground_id: 10",
		"target private source repo: http://gitea.test/owner/repo",
		"target preview_url: http://starter.test",
		"target app subdomain: lk-a33e35d302125bbd",
		"Target playground_id 10 only",
		"fibe_templates_develop with target_type=\"playground\", mode=\"apply\", post_apply=\"rollout_target\", and wait=true",
		"prefer direct Brownfield changes on the live playground workspace for playground_id 10",
		"Do not use rollout_all, do not update default/global Import Templates",
		"User request:\nChange the heading",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestSignupPolicyDefaultsClosedButAllowsAdminExistingAndAllowlist(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test", AdminEmail: "admin@example.com"}, http: http.DefaultClient}

	allowed, err := server.canSignInEmail(t.Context(), "new@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("new non-admin user should be rejected when signup mode is unset")
	}

	allowed, err = server.canSignInEmail(t.Context(), "admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("admin should be allowed even when signup is closed")
	}

	if _, err := store.UpsertUser(t.Context(), "existing@example.com", "Existing", ""); err != nil {
		t.Fatal(err)
	}
	allowed, err = server.canSignInEmail(t.Context(), "existing@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("existing users should be allowed to sign back in")
	}

	if err := store.UpsertConfig(t.Context(), map[string]string{
		"signup_mode":           "allowlist",
		"signup_allowed_emails": "pilot@gmail.com, @trusted.test",
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"pilot@gmail.com", "crew@trusted.test"} {
		allowed, err = server.canSignInEmail(t.Context(), email)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed {
			t.Fatalf("%s should be allowed by allowlist", email)
		}
	}
	allowed, err = server.canSignInEmail(t.Context(), "stranger@gmail.com")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("unlisted user should be rejected in allowlist mode")
	}
}

func TestNormalizeAdminConfigValuesFormatsAllowlistAndPool(t *testing.T) {
	values, err := normalizeAdminConfigValues(map[string]string{
		"signup_allowed_emails": "Pilot@Gmail.com, @Trusted.test\npilot@gmail.com",
		"fibe_agent_server_pool": `[{"label":"Main","agentId":" agent-1 ","serverId":" server-1 "},
			{"label":"","agent_id":"","server_id":""}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if values["signup_allowed_emails"] != "pilot@gmail.com\n@trusted.test" {
		t.Fatalf("allowlist=%q, want newline-normalized emails", values["signup_allowed_emails"])
	}
	pool, err := fibe.ParseAssignmentPool(values["fibe_agent_server_pool"])
	if err != nil {
		t.Fatal(err)
	}
	if len(pool) != 1 || pool[0].AgentID != "agent-1" || pool[0].MarqueeID != "server-1" {
		t.Fatalf("pool=%+v, want normalized single pair", pool)
	}
}

func TestNormalizeAdminConfigRejectsIncompletePoolRows(t *testing.T) {
	_, err := normalizeAdminConfigValues(map[string]string{
		"fibe_agent_server_pool": `[{"agent_id":"agent-only"}]`,
	})
	if err == nil || !strings.Contains(err.Error(), "requires both") {
		t.Fatalf("err=%v, want incomplete pool row error", err)
	}
}

func TestPublicAdminConfigExposesSMTPSettings(t *testing.T) {
	cfg := publicAdminConfig(map[string]string{
		"smtp_host":       "smtp.example.com",
		"smtp_port":       "2525",
		"smtp_from_email": "noreply@example.com",
		"smtp_password":   "secret",
	})
	if entry := cfg["smtp_host"].(map[string]any); entry["value"] != "smtp.example.com" || entry["secret"].(bool) {
		t.Fatalf("smtp_host entry=%+v, want public value", entry)
	}
	if entry := cfg["smtp_password"].(map[string]any); !entry["secret"].(bool) || !entry["set"].(bool) || entry["value"] != "" {
		t.Fatalf("smtp_password entry=%+v, want write-only secret", entry)
	}
}

func TestCreateProjectRecordStoresAssignedFibePair(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	user, err := store.UpsertUser(t.Context(), "pilot@example.com", "Pilot", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"fibe_agent_server_pool": `[{"label":"A","agent_id":"agent-a","server_id":"server-a"},{"label":"B","agent_id":"agent-b","server_id":"server-b"}]`,
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	cfg, err := store.ConfigMap(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want, err := fibe.AssignmentForNewProject(cfg, user.Email)
	if err != nil {
		t.Fatal(err)
	}
	project, err := server.createProjectRecord(t.Context(), user, "Assigned app")
	if err != nil {
		t.Fatal(err)
	}
	if project.AgentID != want.AgentID || project.MarqueeID != want.MarqueeID {
		t.Fatalf("project assignment=%s/%s, want %s/%s", project.AgentID, project.MarqueeID, want.AgentID, want.MarqueeID)
	}
	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.AgentID != want.AgentID || stored.MarqueeID != want.MarqueeID {
		t.Fatalf("stored assignment=%s/%s, want %s/%s", stored.AgentID, stored.MarqueeID, want.AgentID, want.MarqueeID)
	}
}

func TestFibeClientForProjectUsesStoredAssignment(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"fibe_base_url":   "server.test:3000",
		"fibe_api_key":    "secret",
		"fibe_agent_id":   "global-agent",
		"fibe_marquee_id": "global-marquee",
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	client, err := server.fibeClientForProject(t.Context(), &Project{AgentID: "stored-agent", MarqueeID: "stored-marquee"}, "pilot@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if client.AgentID() != "stored-agent" || client.MarqueeID() != "stored-marquee" {
		t.Fatalf("client pair=%s/%s, want stored pair", client.AgentID(), client.MarqueeID())
	}
	if client.BaseURL() != "http://server.test:3000" {
		t.Fatalf("baseURL=%q, want normalized local URL", client.BaseURL())
	}
}

func TestAdminUserListingAndRestrictionControls(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test", AdminEmail: "admin@example.com"}, http: http.DefaultClient}

	user, err := store.UpsertUser(t.Context(), "customer@example.com", "Customer", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{ID: "project-customer", UserID: user.ID, Title: "Customer app", ConversationID: "conv-customer", Status: "ready"}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddMessage(t.Context(), project.ID, "user", "build"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertSocialConnection(t.Context(), SocialConnection{UserID: user.ID, Provider: "github", ProviderUserID: "gh-customer", AccessToken: "secret"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPayment(t.Context(), Payment{UserID: user.ID, ProviderPaymentID: "cs_test", AmountCents: 2500, Currency: "usd", Status: "paid"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddUserNotice(t.Context(), UserNotice{UserID: user.ID, Severity: "warning", Body: "Please reduce usage."}); err != nil {
		t.Fatal(err)
	}

	users, total, err := store.AdminUsers(t.Context(), AdminUserFilters{Github: "connected", Billing: "paid", Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(users) != 1 {
		t.Fatalf("users=%d total=%d, want single paid github-connected user", len(users), total)
	}
	got := users[0]
	if got.MessageCount != 1 || got.ProjectCount != 1 || !got.GithubConnected || got.PaidTotalCents != 2500 || got.LatestNotice == nil {
		t.Fatalf("summary=%+v, want usage/github/payment/notice populated", got)
	}

	if _, err := store.UpdateUserAccess(t.Context(), user.ID, "restricted", "abuse review"); err != nil {
		t.Fatal(err)
	}
	allowed, err := server.canSignInEmail(t.Context(), user.Email)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("restricted user should not be allowed to sign in")
	}
	if err := store.CreateSession(t.Context(), user.ID, "restricted-token", time.Hour); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "restricted-token"})
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("restricted projects request returned %d, want 403", rec.Code)
	}
}

func TestAdminNoticeSendsEmailWhenSMTPConfigured(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"smtp_host":       "smtp.example.test",
		"smtp_port":       "2525",
		"smtp_from_email": "noreply@example.test",
		"smtp_tls_mode":   "none",
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	ch := make(chan emailMessage, 1)
	server := &Server{
		store:  store,
		config: RuntimeConfig{BaseURL: "http://example.test", AdminEmail: "admin@example.com"},
		http:   http.DefaultClient,
		email:  captureEmailSender{ch: ch},
	}
	admin, err := store.UpsertUser(t.Context(), "admin@example.com", "Admin", "")
	if err != nil {
		t.Fatal(err)
	}
	target, err := store.UpsertUser(t.Context(), "customer@example.com", "Customer", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(t.Context(), admin.ID, "admin-token", time.Hour); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+target.ID+"/notices", strings.NewReader(`{"severity":"warning","body":"Please reduce usage."}`))
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "admin-token"})
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("notice returned %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	select {
	case message := <-ch:
		if message.To != "customer@example.com" || message.Subject != "New Likeable message" || !strings.Contains(message.Body, "Please reduce usage.") {
			t.Fatalf("email=%+v, want customer notice email", message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notice email")
	}
}

func TestDailyFreeMessagesAndPaidCredits(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	user, err := store.UpsertUser(t.Context(), "customer@example.com", "Customer", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{ID: "project-quota", UserID: user.ID, Title: "Quota app", ConversationID: "conv-quota", Status: "ready"}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertConfig(t.Context(), map[string]string{"free_messages": "1"}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	oldTime := dailyMessageWindowStart().Add(-time.Hour).Format(time.RFC3339Nano)
	if _, err := store.AddMessageAt(t.Context(), project.ID, "user", "yesterday", oldTime); err != nil {
		t.Fatal(err)
	}
	today, err := store.AddMessage(t.Context(), project.ID, "user", "today")
	if err != nil {
		t.Fatal(err)
	}
	quota := server.messageQuota(t.Context(), user)
	if quota["used"] != 1 || quota["remaining"] != 0 || quota["lifetimeUsed"] != 2 {
		t.Fatalf("quota=%+v, want daily used 1, remaining 0, lifetime 2", quota)
	}
	allowed, paid, err := server.messageAllowance(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	if allowed || !paid {
		t.Fatalf("allowed=%v paid=%v, want paid allowance required but unavailable", allowed, paid)
	}
	if _, err := store.GrantMessageCredits(t.Context(), user.ID, "cs_pack", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GrantMessageCredits(t.Context(), user.ID, "cs_pack", 10); err != nil {
		t.Fatal(err)
	}
	balance, err := store.PaidMessageCreditBalance(t.Context(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 10 {
		t.Fatalf("balance=%d, want idempotent grant of 10", balance)
	}
	allowed, paid, err = server.messageAllowance(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || !paid {
		t.Fatalf("allowed=%v paid=%v, want paid allowance", allowed, paid)
	}
	if err := store.ConsumePaidMessageCredit(t.Context(), user.ID, today.ID); err != nil {
		t.Fatal(err)
	}
	balance, err = store.PaidMessageCreditBalance(t.Context(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 9 {
		t.Fatalf("balance=%d, want 9 after consume", balance)
	}
}

func TestPaidProjectQuotaExtendsProjectCap(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	user, err := store.UpsertUser(t.Context(), "projects@example.com", "Projects", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertConfig(t.Context(), map[string]string{"project_cap": "1"}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	if cap := server.projectCapForUser(t.Context(), user); cap != 1 {
		t.Fatalf("cap=%d, want base cap 1", cap)
	}
	granted, err := store.GrantProjectQuota(t.Context(), user.ID, "cs_project_slot", 1, time.Now().UTC().Add(30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !granted {
		t.Fatal("first project quota grant should be applied")
	}
	granted, err = store.GrantProjectQuota(t.Context(), user.ID, "cs_project_slot", 1, time.Now().UTC().Add(30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if granted {
		t.Fatal("duplicate project quota payment should be idempotent")
	}
	if cap := server.projectCapForUser(t.Context(), user); cap != 2 {
		t.Fatalf("cap=%d, want base plus paid slot", cap)
	}
	quota := server.projectQuota(t.Context(), user)
	if quota["baseLimit"] != 1 || quota["paidSlots"] != 1 || quota["limit"] != 2 {
		t.Fatalf("quota=%+v, want paid project quota reflected", quota)
	}
}

func TestProjectQuotaCheckoutBuildsStripeMetadata(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"stripe_secret_key":             "sk_test",
		"stripe_project_quota_price_id": "price_project_slot",
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	user, err := store.UpsertUser(t.Context(), "buyer@example.com", "Buyer", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(t.Context(), user.ID, "buyer-token", time.Hour); err != nil {
		t.Fatal(err)
	}
	var form url.Values
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		form, err = url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"url":"https://checkout.stripe.test/session"}`)),
		}, nil
	})}
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: client}
	req := httptest.NewRequest(http.MethodPost, "/api/billing/checkout", strings.NewReader(`{"product":"project_quota","slots":1}`))
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "buyer-token"})
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("checkout returned %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if form.Get("line_items[0][price]") != "price_project_slot" ||
		form.Get("metadata[purchase_kind]") != "project_quota" ||
		form.Get("metadata[project_slots]") != "1" ||
		form.Get("metadata[project_quota_days]") != "30" {
		t.Fatalf("stripe form=%v, want project quota metadata", form)
	}
}

func TestStripeWebhookGrantsProjectQuota(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"stripe_secret_key":             "sk_test",
		"stripe_webhook_secret":         "whsec_test",
		"stripe_project_quota_price_id": "price_project_slot",
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	user, err := store.UpsertUser(t.Context(), "buyer@example.com", "Buyer", "")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "/line_items") {
			t.Fatalf("unexpected stripe request %s", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"price":{"id":"price_project_slot"}}]}`)),
		}, nil
	})}
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: client}
	event := map[string]any{
		"type": "checkout.session.completed",
		"data": map[string]any{"object": map[string]any{
			"id":                  "cs_project_quota",
			"client_reference_id": user.ID,
			"amount_total":        1200,
			"currency":            "usd",
			"payment_status":      "paid",
			"status":              "complete",
			"metadata": map[string]any{
				"purchase_kind": "project_quota",
				"project_slots": "1",
			},
		}},
	}
	payload, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", testStripeSignature("whsec_test", payload))
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("webhook returned %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	slots, expiresAt, err := store.ActiveProjectQuota(t.Context(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if slots != 1 || expiresAt == "" {
		t.Fatalf("slots=%d expires=%q, want active paid project slot", slots, expiresAt)
	}
	notices, err := store.NoticesForUser(t.Context(), user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(notices) == 0 || !strings.Contains(notices[0].Body, "Project quota purchased") {
		t.Fatalf("notices=%+v, want project quota purchase notice", notices)
	}
}

func TestFreeQuotaIsConsumedBeforePaidCredits(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	user, err := store.UpsertUser(t.Context(), "paid@example.com", "Paid", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{ID: "project-free-first", UserID: user.ID, Title: "Free first", ConversationID: "conv-free-first", Status: "ready"}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertConfig(t.Context(), map[string]string{"free_messages": "2"}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GrantMessageCredits(t.Context(), user.ID, "cs_paid_pack", 10); err != nil {
		t.Fatal(err)
	}

	allowed, paid, err := server.messageAllowance(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || paid {
		t.Fatalf("first allowance allowed=%v paid=%v, want free allowance even with paid credits", allowed, paid)
	}
	first, err := store.AddMessage(t.Context(), project.ID, "user", "free one")
	if err != nil {
		t.Fatal(err)
	}
	if paid {
		if err := store.ConsumePaidMessageCredit(t.Context(), user.ID, first.ID); err != nil {
			t.Fatal(err)
		}
	}
	balance, err := store.PaidMessageCreditBalance(t.Context(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 10 {
		t.Fatalf("balance=%d, want paid credits untouched while free quota remains", balance)
	}

	allowed, paid, err = server.messageAllowance(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || paid {
		t.Fatalf("second allowance allowed=%v paid=%v, want second free allowance", allowed, paid)
	}
	second, err := store.AddMessage(t.Context(), project.ID, "user", "free two")
	if err != nil {
		t.Fatal(err)
	}
	if paid {
		if err := store.ConsumePaidMessageCredit(t.Context(), user.ID, second.ID); err != nil {
			t.Fatal(err)
		}
	}
	balance, err = store.PaidMessageCreditBalance(t.Context(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 10 {
		t.Fatalf("balance=%d, want paid credits untouched after free quota is consumed exactly", balance)
	}

	allowed, paid, err = server.messageAllowance(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || !paid {
		t.Fatalf("third allowance allowed=%v paid=%v, want paid allowance after free quota is exhausted", allowed, paid)
	}
	third, err := store.AddMessage(t.Context(), project.ID, "user", "paid one")
	if err != nil {
		t.Fatal(err)
	}
	if paid {
		if err := store.ConsumePaidMessageCredit(t.Context(), user.ID, third.ID); err != nil {
			t.Fatal(err)
		}
	}
	balance, err = store.PaidMessageCreditBalance(t.Context(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 9 {
		t.Fatalf("balance=%d, want one paid credit consumed after free quota is exhausted", balance)
	}
}

func TestMessageQuotaNotificationIsDedupedPerDay(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{store: store, config: RuntimeConfig{BaseURL: "http://example.test"}, http: http.DefaultClient}
	user, err := store.UpsertUser(t.Context(), "quota@example.com", "Quota", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{ID: "project-quota-notice", UserID: user.ID, Title: "Quota notice", ConversationID: "conv-quota-notice", Status: "ready"}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertConfig(t.Context(), map[string]string{"free_messages": "1"}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddMessage(t.Context(), project.ID, "user", "first"); err != nil {
		t.Fatal(err)
	}
	server.notifyMessageQuotaIfNeeded(t.Context(), user)
	server.notifyMessageQuotaIfNeeded(t.Context(), user)
	notices, err := store.NoticesForUser(t.Context(), user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	var quotaNotices int
	for _, notice := range notices {
		if strings.HasPrefix(notice.Body, "Message quota:") {
			quotaNotices++
		}
	}
	if quotaNotices != 1 {
		t.Fatalf("quota notices=%d, want 1", quotaNotices)
	}
}

func TestMailboxDismissAndAdminUnsend(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "customer@example.com", "Customer", "")
	if err != nil {
		t.Fatal(err)
	}
	notice, err := store.AddUserNotice(t.Context(), UserNotice{UserID: user.ID, Sender: "admin", Severity: "warning", Body: "Please reduce usage."})
	if err != nil {
		t.Fatal(err)
	}
	active, err := store.ActiveNoticesForUser(t.Context(), user.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Fatalf("active=%d, want 1", len(active))
	}
	dismissed, err := store.DismissUserNotice(t.Context(), user.ID, notice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dismissed.DismissedAt == "" || dismissed.ReadAt == "" {
		t.Fatalf("dismissed notice=%+v, want read and dismissed timestamps", dismissed)
	}
	active, err = store.ActiveNoticesForUser(t.Context(), user.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("active=%d, want dismissed notice hidden", len(active))
	}
	if _, err := store.AddUserNotice(t.Context(), UserNotice{UserID: user.ID, Sender: "user", Body: "I need help."}); err != nil {
		t.Fatal(err)
	}
	history, err := store.NoticesForUser(t.Context(), user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("history=%d, want admin and user messages", len(history))
	}
	if _, err := store.UnsendUserNotice(t.Context(), user.ID, notice.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UserNotice(t.Context(), user.ID, notice.ID); err == nil {
		t.Fatal("unsent notice should not be visible to user history")
	}
	history, err = store.NoticesForUser(t.Context(), user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Sender != "user" {
		t.Fatalf("history=%+v, want only user-to-admin message after unsend", history)
	}
}

func TestAnonymousRateLimitUsesIPAddress(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := &Server{
		store:  store,
		config: RuntimeConfig{BaseURL: "http://example.test"},
		http:   http.DefaultClient,
		limiter: newRateLimiter(rateLimitConfig{
			anonymousLimit:      2,
			anonymousWindow:     time.Minute,
			authenticatedLimit:  100,
			authenticatedWindow: time.Hour,
		}),
	}

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.RemoteAddr = "203.0.113.10:45123"
		rec := httptest.NewRecorder()
		server.routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d returned %d, want 200; body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.RemoteAddr = "203.0.113.10:45123"
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("limited request returned %d, want 429; body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" || rec.Header().Get("X-RateLimit-Limit") != "2" {
		t.Fatalf("rate headers missing: %+v", rec.Header())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.RemoteAddr = "203.0.113.11:45123"
	rec = httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("different IP returned %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthenticatedRateLimitUsesUserID(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userA, err := store.UpsertUser(t.Context(), "a@example.com", "A", "")
	if err != nil {
		t.Fatal(err)
	}
	userB, err := store.UpsertUser(t.Context(), "b@example.com", "B", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(t.Context(), userA.ID, "token-a", time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(t.Context(), userB.ID, "token-b", time.Hour); err != nil {
		t.Fatal(err)
	}
	server := &Server{
		store:  store,
		config: RuntimeConfig{BaseURL: "http://example.test"},
		http:   http.DefaultClient,
		limiter: newRateLimiter(rateLimitConfig{
			anonymousLimit:      100,
			anonymousWindow:     time.Minute,
			authenticatedLimit:  2,
			authenticatedWindow: time.Hour,
		}),
	}

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.RemoteAddr = "203.0.113.20:45123"
		req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
		rec := httptest.NewRecorder()
		server.routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("authenticated request %d returned %d, want 200; body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.RemoteAddr = "203.0.113.20:45123"
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-a"})
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("limited authenticated request returned %d, want 429; body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-RateLimit-Limit") != "2" {
		t.Fatalf("authenticated rate headers missing: %+v", rec.Header())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.RemoteAddr = "203.0.113.20:45123"
	req.AddCookie(&http.Cookie{Name: "likeable_session", Value: "token-b"})
	rec = httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("different authenticated user returned %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}
