package likeable

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fibegg/likeable/internal/fibe"
	"github.com/fibegg/likeable/internal/store"
	"github.com/hibiken/asynq"
)

func TestRetryProjectProvisionLaterRequiresProvisionedResources(t *testing.T) {
	project := &Project{
		ID:             "project-no-resources",
		ConversationID: "conv-no-resources",
	}
	if retryProjectProvisionLater(project, errors.New("greenfield failed")) {
		t.Fatal("conversation-only project should not stay launching after provisioning failure")
	}

	if !retryProjectProvisionLater(project, &fibe.PlatformError{Code: "INTERNAL_ERROR", Status: 422, Message: "unexpected status 422"}) {
		t.Fatal("transient platform failure before Greenfield should remain retryable")
	}

	project.PlaygroundID = "playground-1"
	if !retryProjectProvisionLater(project, errors.New("preview not ready")) {
		t.Fatal("project with created playground should stay retryable")
	}
}

func TestProjectHasFibeResourcesIgnoresSyntheticIdentifiers(t *testing.T) {
	project := &Project{
		ID:             "project-synthetic-only",
		ConversationID: "conv-synthetic-only",
		PlaygroundName: "project-synthetic-only",
	}
	if projectHasFibeResources(project) {
		t.Fatal("synthetic project name and conversation id should not count as remote resources")
	}

	project.PlaygroundID = "123"
	if !projectHasFibeResources(project) {
		t.Fatal("playground id should count as a remote resource")
	}
}

func TestProjectHasDeleteReadySnapshotRequiresDeletableResourceIds(t *testing.T) {
	project := &Project{PlaygroundID: "123", PreviewURL: "https://preview.example.test"}
	if projectHasDeleteReadySnapshot(project) {
		t.Fatal("preview URL without playspec or source ids should not skip snapshot recovery")
	}

	project.PlayspecID = "456"
	project.PropID = "789"
	if !projectHasDeleteReadySnapshot(project) {
		t.Fatal("playground, playspec, and source ids should be enough for deletion")
	}
}

func TestEnsureDefaultProjectSkipsRestrictedUser(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "pilot@example.com", "Pilot", "")
	if err != nil {
		t.Fatal(err)
	}
	user, err = store.UpdateUserAccess(t.Context(), user.ID, "restricted", "account deletion requested")
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store}

	server.ensureDefaultProject(t.Context(), user)

	count, err := store.ProjectCountForUser(t.Context(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("project count=%d, want 0", count)
	}
}

func TestRecordProjectProvisionFailureMarksPreGreenfieldFailureAsError(t *testing.T) {
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
		ID:             "project-pre-greenfield-failure",
		UserID:         user.ID,
		Title:          "Pre Greenfield Failure",
		ConversationID: "conv-pre-greenfield-failure",
		Status:         "creating",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store}

	server.recordProjectProvisionFailure(t.Context(), user.ID, project, errors.New("greenfield failed"), false)

	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "error" {
		t.Fatalf("status=%q, want error", stored.Status)
	}
	if stored.ErrorMessage == "" {
		t.Fatal("error message was not stored")
	}
}

func TestRecordProjectProvisionFailureKeepsTransientPreGreenfieldFailureCreating(t *testing.T) {
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
		ID:             "project-transient-pre-greenfield-failure",
		UserID:         user.ID,
		Title:          "Transient Pre Greenfield Failure",
		ConversationID: "conv-transient-pre-greenfield-failure",
		Status:         "creating",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store}

	server.recordProjectProvisionFailure(t.Context(), user.ID, project, &fibe.PlatformError{Code: "INTERNAL_ERROR", Status: 422, Message: "unexpected status 422"}, true)

	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "creating" {
		t.Fatalf("status=%q, want creating", stored.Status)
	}
	if stored.ErrorMessage != "" {
		t.Fatalf("error_message=%q, want empty", stored.ErrorMessage)
	}
	diagnostics, err := store.AdminProjectDiagnostics(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagnostics.Internal.InternalErrorMessage, "unexpected status 422") {
		t.Fatalf("internal_error_message=%q, want provisioning retry cause", diagnostics.Internal.InternalErrorMessage)
	}
}

func TestProjectNeedsProvisioningRecoveryWaitsForFreshOrLockedProject(t *testing.T) {
	now := time.Now().UTC()
	project := &Project{
		Status:    "creating",
		CreatedAt: now.Format(time.RFC3339Nano),
		UpdatedAt: now.Format(time.RFC3339Nano),
	}
	if projectNeedsProvisioningRecovery(project) {
		t.Fatal("freshly created project should let the original provisioning job run first")
	}

	project.CreatedAt = now.Add(-projectProvisioningRecoveryGrace - time.Second).Format(time.RFC3339Nano)
	project.UpdatedAt = project.CreatedAt
	if !projectNeedsProvisioningRecovery(project) {
		t.Fatal("stale project without playground should be recoverable")
	}

	project.ProvisioningLockUntil = now.Add(time.Minute).Format(time.RFC3339Nano)
	if projectNeedsProvisioningRecovery(project) {
		t.Fatal("active provisioning lease should suppress recovery")
	}
}

func TestReserveProjectRecoveryDeduplicatesUntilTTL(t *testing.T) {
	server := &Server{}
	if !server.reserveProjectRecovery("project:recovery", 25*time.Millisecond) {
		t.Fatal("first recovery reservation should be accepted")
	}
	if server.reserveProjectRecovery("project:recovery", 25*time.Millisecond) {
		t.Fatal("duplicate recovery reservation should be suppressed")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if server.reserveProjectRecovery("project:recovery", 25*time.Millisecond) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("recovery reservation should expire after ttl")
}

func TestProvisionProjectTaskDoesNotRetryDefaultTemplateConfigurationFailure(t *testing.T) {
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fibeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/greenfields" {
			writeJSONStatus(t, w, http.StatusUnprocessableEntity, map[string]any{
				"error": map[string]any{
					"code":    "GREENFIELD_DEFAULT_TEMPLATE_VERSION_UNAVAILABLE",
					"message": "Default greenfield template version is configured but is not available",
				},
			})
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/api/playgrounds" {
			writeJSONResponse(t, w, map[string]any{"data": []any{}, "meta": map[string]any{"page": 1, "per_page": 100, "total": 0}})
			return
		}
		writeJSONStatus(t, w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "RESOURCE_NOT_FOUND", "message": "not found"}})
	}))
	defer fibeAPI.Close()
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"fibe_base_url": fibeAPI.URL,
		"fibe_api_key":  "test-key",
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	user, err := store.UpsertUser(t.Context(), "pilot@example.com", "Pilot", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-default-template-unavailable",
		UserID:         user.ID,
		Title:          "Default Template Unavailable",
		ConversationID: "conv-default-template-unavailable",
		AgentID:        "agent-1",
		MarqueeID:      "marquee-1",
		Status:         "creating",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store, http: fibeAPI.Client()}
	payload, err := json.Marshal(projectJobPayload{UserID: user.ID, UserEmail: user.Email, ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}

	if err := server.handleProvisionProjectTask(t.Context(), asynq.NewTask(taskProvisionProject, payload)); err != nil {
		t.Fatalf("handleProvisionProjectTask returned retryable error: %v", err)
	}

	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "error" {
		t.Fatalf("status=%q, want error", stored.Status)
	}
	if !strings.Contains(stored.ErrorMessage, "Workspace settings are incomplete") {
		t.Fatalf("error_message=%q, want configuration guidance", stored.ErrorMessage)
	}
}

func TestProvisionProjectSkipsAlreadyReadyProject(t *testing.T) {
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
		ID:             "project-ready",
		UserID:         user.ID,
		Title:          "Ready",
		ConversationID: "conv-ready",
		PlaygroundID:   "206",
		Status:         "ready",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store}

	if err := server.provisionProject(t.Context(), user.ID, user.Email, project, ""); err != nil {
		t.Fatal(err)
	}

	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "ready" {
		t.Fatalf("status=%q, want ready", stored.Status)
	}
}

func TestProvisionProjectStartsAssignedAgentChat(t *testing.T) {
	previewServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!doctype html><title>ready</title>"))
	}))
	defer previewServer.Close()
	store, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cliPath, logPath, _ := fakeFibeCLI(t)
	if err := store.UpsertConfig(t.Context(), map[string]string{
		"fibe_base_url": "server.test:3000",
		"fibe_api_key":  "test-key",
		"fibe_cli_path": cliPath,
	}, secretConfigKeys); err != nil {
		t.Fatal(err)
	}
	user, err := store.UpsertUser(t.Context(), "pilot@example.com", "Pilot", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-start-agent",
		UserID:         user.ID,
		Title:          "Start Agent",
		ConversationID: "conv-start-agent",
		AgentID:        "agent-1",
		MarqueeID:      "multipass",
		PlaygroundID:   "123",
		PreviewURL:     previewServer.URL,
		Status:         "creating",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store, http: fakeFibeHTTPClient(previewServer.Client(), fakeFibeTransportConfig{Mode: "default", LogPath: logPath})}

	if err := server.provisionProject(t.Context(), user.ID, user.Email, project, ""); err != nil {
		t.Fatal(err)
	}

	log := readFile(t, logPath)
	if !strings.Contains(log, "agents start-chat agent-1 --marquee-id multipass") {
		t.Fatalf("log=%s, want provisioning to start assigned agent chat", log)
	}
}
