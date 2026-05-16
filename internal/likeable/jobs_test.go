package likeable

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/fibegg/likeable/internal/store"
	"github.com/hibiken/asynq"
)

func TestProjectCleanupLimiterAllowsOnlyOneCleanup(t *testing.T) {
	server := &Server{}

	release, ok := server.tryAcquireProjectCleanupSlot()
	if !ok {
		t.Fatal("first cleanup slot was not acquired")
	}
	if _, ok := server.tryAcquireProjectCleanupSlot(); ok {
		t.Fatal("second cleanup slot was acquired")
	}
	release()
	release, ok = server.tryAcquireProjectCleanupSlot()
	if !ok {
		t.Fatal("cleanup slot was not acquired after release")
	}
	release()
}

func TestJobWorkerConcurrencyUsesConfigurableDefault(t *testing.T) {
	t.Setenv("LIKEABLE_JOB_CONCURRENCY", "")
	if got := jobWorkerConcurrency(); got != defaultJobWorkerConcurrency {
		t.Fatalf("jobWorkerConcurrency()=%d, want %d", got, defaultJobWorkerConcurrency)
	}

	t.Setenv("LIKEABLE_JOB_CONCURRENCY", "96")
	if got := jobWorkerConcurrency(); got != 96 {
		t.Fatalf("jobWorkerConcurrency()=%d, want configured value", got)
	}
}

func TestJobWorkerConcurrencyFallsBackForInvalidConfig(t *testing.T) {
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prev) })
	t.Setenv("LIKEABLE_JOB_CONCURRENCY", "0")

	if got := jobWorkerConcurrency(); got != defaultJobWorkerConcurrency {
		t.Fatalf("jobWorkerConcurrency()=%d, want %d", got, defaultJobWorkerConcurrency)
	}
	if logs.Len() == 0 {
		t.Fatal("invalid concurrency config was not logged")
	}
}

func TestDeleteProjectResourcesTaskYieldsWhenCleanupSlotBusy(t *testing.T) {
	store, user, project := testDeletingProject(t)
	server := &Server{store: store}
	release, ok := server.tryAcquireProjectCleanupSlot()
	if !ok {
		t.Fatal("cleanup slot was not acquired")
	}
	defer release()
	payload, err := json.Marshal(projectJobPayload{UserID: user.ID, UserEmail: user.Email, ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}

	err = server.handleDeleteProjectResourcesTask(t.Context(), asynq.NewTask(taskDeleteProjectResources, payload))

	if !errors.Is(err, errProjectCleanupConcurrencyLimit) {
		t.Fatalf("handleDeleteProjectResourcesTask error=%v, want cleanup concurrency limit", err)
	}
	acquired, err := store.TryAcquireProjectCleanup(t.Context(), project.ID, user.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired {
		t.Fatal("cleanup lease was not released after slot limit")
	}
}

func TestDeleteProjectResourcesTaskIgnoresDuplicateActiveCleanup(t *testing.T) {
	store, user, project := testDeletingProject(t)
	server := &Server{store: store}
	acquired, err := store.TryAcquireProjectCleanup(t.Context(), project.ID, user.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired {
		t.Fatal("cleanup lease was not acquired")
	}
	payload, err := json.Marshal(projectJobPayload{UserID: user.ID, UserEmail: user.Email, ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}

	if err := server.handleDeleteProjectResourcesTask(t.Context(), asynq.NewTask(taskDeleteProjectResources, payload)); err != nil {
		t.Fatalf("duplicate cleanup returned error: %v", err)
	}
}

func testDeletingProject(t *testing.T) (*store.Store, *User, *Project) {
	t.Helper()
	appStore, err := store.Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = appStore.Close() })
	user, err := appStore.UpsertUser(t.Context(), "cleanup@example.com", "Cleanup", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-cleanup",
		UserID:         user.ID,
		Title:          "Cleanup",
		ConversationID: "conv-cleanup",
		Status:         "deleting",
	}
	if err := appStore.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	return appStore, user, project
}
