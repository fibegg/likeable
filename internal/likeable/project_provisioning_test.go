package likeable

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fibegg/likeable/internal/fibe"
	"github.com/fibegg/likeable/internal/store"
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
	server := &Server{store: store, http: previewServer.Client()}

	if err := server.provisionProject(t.Context(), user.ID, user.Email, project, ""); err != nil {
		t.Fatal(err)
	}

	log := readFile(t, logPath)
	if !strings.Contains(log, "agents start-chat agent-1 --marquee-id multipass") {
		t.Fatalf("log=%s, want provisioning to start assigned agent chat", log)
	}
}
