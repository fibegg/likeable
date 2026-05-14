package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestTryAcquireProjectProvisioningLeasesProject(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "lease@example.com", "Lease", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-lease",
		UserID:         user.ID,
		Title:          "Lease",
		ConversationID: "conv-lease",
		Status:         "creating",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}

	acquired, err := store.TryAcquireProjectProvisioning(t.Context(), project.ID, user.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired {
		t.Fatal("first provisioning lease was not acquired")
	}
	acquired, err = store.TryAcquireProjectProvisioning(t.Context(), project.ID, user.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if acquired {
		t.Fatal("duplicate provisioning lease was acquired")
	}
	if err := store.ClearProjectProvisioningLease(t.Context(), project.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	acquired, err = store.TryAcquireProjectProvisioning(t.Context(), project.ID, user.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired {
		t.Fatal("provisioning lease was not acquired after clearing")
	}
}

func TestSaveProjectProvisioningSnapshotDoesNotResurrectDeletingProject(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "delete@example.com", "Delete", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-delete-race",
		UserID:         user.ID,
		Title:          "Delete Race",
		ConversationID: "conv-delete-race",
		Status:         "creating",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateProjectStatus(t.Context(), project.ID, user.ID, "deleting"); err != nil {
		t.Fatal(err)
	}

	project.PlaygroundID = "playground-1"
	project.PlaygroundName = "delete-race"
	project.PlayspecID = "playspec-1"
	project.PropID = "prop-1"
	project.RepoURL = "http://gitea.test/owner/repo.git"
	project.PreviewURL = "http://delete-race.test"
	project.Repositories = []ProjectRepository{
		{ProjectID: project.ID, Role: "app", PropID: "prop-1", RepoURL: project.RepoURL, ServiceNames: []string{"app"}},
	}
	project.Services = []ProjectService{
		{ProjectID: project.ID, Name: "app", URL: project.PreviewURL, Type: "dynamic", Visibility: "external"},
	}

	if err := store.SaveProjectProvisioningSnapshot(t.Context(), project, "ready"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SaveProjectProvisioningSnapshot error=%v, want sql.ErrNoRows", err)
	}
	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "deleting" {
		t.Fatalf("status=%q, want deleting", stored.Status)
	}
	if stored.PlaygroundID != "" || len(stored.Repositories) != 0 || len(stored.Services) != 0 {
		t.Fatalf("deleting project was resurrected: %+v", stored)
	}
}

func TestIdleProjectsForPlaygroundStopUsesDedicatedUsageTimestamp(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "idle@example.com", "Idle", "")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-9 * time.Hour).Format(time.RFC3339Nano)
	recent := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)

	oldProject := &Project{ID: "old-idle", UserID: user.ID, Title: "Old", ConversationID: "conv-old", PlaygroundID: "pg-old", Status: "ready", PlaygroundLastUsedAt: old}
	if err := store.CreateProject(t.Context(), oldProject); err != nil {
		t.Fatal(err)
	}
	recentProject := &Project{ID: "recent-idle", UserID: user.ID, Title: "Recent", ConversationID: "conv-recent", PlaygroundID: "pg-recent", Status: "ready", PlaygroundLastUsedAt: recent}
	if err := store.CreateProject(t.Context(), recentProject); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddMessageAt(t.Context(), recentProject.ID, "user", "old message", old); err != nil {
		t.Fatal(err)
	}
	missingProject := &Project{ID: "missing-idle", UserID: user.ID, Title: "Missing", ConversationID: "conv-missing", PlaygroundID: "pg-missing", Status: "ready", PlaygroundLastUsedAt: recent}
	if err := store.CreateProject(t.Context(), missingProject); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(t.Context(), `UPDATE projects SET playground_last_used_at = '' WHERE id = ?`, missingProject.ID); err != nil {
		t.Fatal(err)
	}

	projects, err := store.IdleProjectsForPlaygroundStop(t.Context(), time.Now().UTC().Add(-8*time.Hour), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].ID != oldProject.ID {
		t.Fatalf("idle projects=%+v, want only %s", projects, oldProject.ID)
	}
	if projects[0].PlaygroundIdleStopAt == "" {
		t.Fatalf("idle stop timestamp was not computed: %+v", projects[0])
	}
}

func TestBackfillProjectPlaygroundUsageProtectsReadyProjectsOnStartup(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "backfill@example.com", "Backfill", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{ID: "backfill-ready", UserID: user.ID, Title: "Ready", ConversationID: "conv-backfill", PlaygroundID: "pg-backfill", Status: "ready"}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(t.Context(), `UPDATE projects SET playground_last_used_at = '' WHERE id = ?`, project.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.backfillProjectPlaygroundUsage(t.Context()); err != nil {
		t.Fatal(err)
	}
	projects, err := store.IdleProjectsForPlaygroundStop(t.Context(), time.Now().UTC().Add(-8*time.Hour), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("idle projects after backfill=%+v, want none", projects)
	}
	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.PlaygroundLastUsedAt == "" {
		t.Fatal("playground_last_used_at was not backfilled")
	}
}

func TestProjectIdleForPlaygroundStopSkipsMissingOrInvalidUsageTimestamp(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "idle-invalid@example.com", "Idle", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{ID: "invalid-idle", UserID: user.ID, Title: "Invalid", ConversationID: "conv-invalid", PlaygroundID: "pg-invalid", Status: "ready", PlaygroundLastUsedAt: time.Now().UTC().Add(-9 * time.Hour).Format(time.RFC3339Nano)}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(t.Context(), `UPDATE projects SET playground_last_used_at = '' WHERE id = ?`, project.ID); err != nil {
		t.Fatal(err)
	}
	idle, reason, err := store.ProjectIdleForPlaygroundStop(t.Context(), project.ID, time.Now().UTC().Add(-8*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if idle || reason == "" {
		t.Fatalf("idle=%v reason=%q, want skipped missing timestamp", idle, reason)
	}
	if _, err := store.db.ExecContext(t.Context(), `UPDATE projects SET playground_last_used_at = 'not-a-time' WHERE id = ?`, project.ID); err != nil {
		t.Fatal(err)
	}
	idle, reason, err = store.ProjectIdleForPlaygroundStop(t.Context(), project.ID, time.Now().UTC().Add(-8*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if idle || reason == "" {
		t.Fatalf("idle=%v reason=%q, want skipped invalid timestamp", idle, reason)
	}
}

func TestPublicProjectErrorMessageExplainsLinkedFibePlaygroundError(t *testing.T) {
	got := publicProjectErrorMessage("The linked Fibe playground is in an error state.")
	want := "The linked Fibe playground is in an error state. Check it in Fibe, then restart the project playground from the project menu."
	if got != want {
		t.Fatalf("message=%q, want %q", got, want)
	}
}
