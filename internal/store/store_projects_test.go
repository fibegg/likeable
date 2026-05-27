package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fibegg/likeable/internal/fibe"
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

func TestTryAcquireProjectCleanupLeasesDeletingProject(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "cleanup-lease@example.com", "Cleanup Lease", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-cleanup-lease",
		UserID:         user.ID,
		Title:          "Cleanup Lease",
		ConversationID: "conv-cleanup-lease",
		Status:         "deleting",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateProjectCleanupError(t.Context(), project.ID, user.ID, "previous cleanup failure"); err != nil {
		t.Fatal(err)
	}

	acquired, err := store.TryAcquireProjectCleanup(t.Context(), project.ID, user.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired {
		t.Fatal("first cleanup lease was not acquired")
	}
	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CleanupLastError != "" {
		t.Fatalf("cleanup_last_error=%q, want cleared on acquired retry", stored.CleanupLastError)
	}
	acquired, err = store.TryAcquireProjectCleanup(t.Context(), project.ID, user.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if acquired {
		t.Fatal("duplicate cleanup lease was acquired")
	}
	if err := store.ClearProjectCleanupLease(t.Context(), project.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	acquired, err = store.TryAcquireProjectCleanup(t.Context(), project.ID, user.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired {
		t.Fatal("cleanup lease was not acquired after clearing")
	}
}

func TestTryAcquireProjectCleanupSkipsActiveProject(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "active-cleanup@example.com", "Active Cleanup", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-active-cleanup",
		UserID:         user.ID,
		Title:          "Active Cleanup",
		ConversationID: "conv-active-cleanup",
		Status:         "ready",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}

	acquired, err := store.TryAcquireProjectCleanup(t.Context(), project.ID, user.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if acquired {
		t.Fatal("cleanup lease should not be acquired for active project")
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

func TestSaveProjectProvisioningSnapshotPreservesExistingUsageTimestamp(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "snapshot@example.com", "Snapshot", "")
	if err != nil {
		t.Fatal(err)
	}
	oldUsage := time.Now().UTC().Add(-9 * time.Hour).Format(time.RFC3339Nano)
	project := &Project{
		ID:                   "project-snapshot-usage",
		UserID:               user.ID,
		Title:                "Snapshot Usage",
		ConversationID:       "conv-snapshot-usage",
		Status:               "ready",
		PlaygroundID:         "playground-old",
		PlaygroundLastUsedAt: oldUsage,
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}

	project.PlaygroundName = "snapshot-usage"
	project.PlayspecID = "playspec-1"
	project.PropID = "prop-1"
	project.RepoURL = "http://gitea.test/owner/repo.git"
	project.PreviewURL = "http://snapshot.test"
	project.SelectedService = "app"
	project.PlaygroundLastUsedAt = time.Now().UTC().Format(time.RFC3339Nano)
	project.Repositories = []ProjectRepository{
		{ProjectID: project.ID, Role: "app", PropID: "prop-1", RepoURL: project.RepoURL, ServiceNames: []string{"app"}},
	}
	project.Services = []ProjectService{
		{ProjectID: project.ID, Name: "app", URL: project.PreviewURL, Type: "dynamic", Visibility: "external"},
	}
	if err := store.SaveProjectProvisioningSnapshot(t.Context(), project, "ready"); err != nil {
		t.Fatal(err)
	}
	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.PlaygroundLastUsedAt != oldUsage {
		t.Fatalf("playground_last_used_at changed from %q to %q", oldUsage, stored.PlaygroundLastUsedAt)
	}
	if stored.PreviewURL != project.PreviewURL {
		t.Fatalf("snapshot metadata was not saved: preview_url=%q", stored.PreviewURL)
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

func TestIdleProjectsForPlaygroundStopSkipsOnlyActiveProductionProjects(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "idle-production@example.com", "Idle Production", "")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-9 * time.Hour).Format(time.RFC3339Nano)
	activeProject := &Project{ID: "active-production-idle", UserID: user.ID, Title: "Active", ConversationID: "conv-active", PlaygroundID: "pg-active", Status: "ready", PlaygroundLastUsedAt: old}
	expiredProject := &Project{ID: "expired-production-idle", UserID: user.ID, Title: "Expired", ConversationID: "conv-expired", PlaygroundID: "pg-expired", Status: "ready", PlaygroundLastUsedAt: old}
	for _, project := range []*Project{activeProject, expiredProject} {
		if err := store.CreateProject(t.Context(), project); err != nil {
			t.Fatal(err)
		}
	}
	if granted, err := store.GrantProjectProduction(t.Context(), user.ID, activeProject.ID, "cs_active_production_idle", time.Now().UTC().Add(30*24*time.Hour)); err != nil || !granted {
		t.Fatalf("active production grant=%v err=%v, want granted", granted, err)
	}
	if granted, err := store.GrantProjectProduction(t.Context(), user.ID, expiredProject.ID, "cs_expired_production_idle", time.Now().UTC().Add(-time.Hour)); err != nil || !granted {
		t.Fatalf("expired production grant=%v err=%v, want granted", granted, err)
	}

	projects, err := store.IdleProjectsForPlaygroundStop(t.Context(), time.Now().UTC().Add(-8*time.Hour), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].ID != expiredProject.ID {
		t.Fatalf("idle projects=%+v, want only expired production project", projects)
	}
	idle, reason, err := store.ProjectIdleForPlaygroundStop(t.Context(), activeProject.ID, time.Now().UTC().Add(-8*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if idle || reason == "" {
		t.Fatalf("active production idle=%v reason=%q, want skipped with reason", idle, reason)
	}
	idle, reason, err = store.ProjectIdleForPlaygroundStop(t.Context(), expiredProject.ID, time.Now().UTC().Add(-8*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !idle || reason != "" {
		t.Fatalf("expired production idle=%v reason=%q, want eligible idle project", idle, reason)
	}
}

func TestGrantProjectProductionIgnoresInactiveProjects(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "production-inactive@example.com", "Inactive", "")
	if err != nil {
		t.Fatal(err)
	}
	archived := &Project{ID: "archived-production-grant", UserID: user.ID, Title: "Archived", ConversationID: "conv-archived", Status: "archived"}
	deleting := &Project{ID: "deleting-production-grant", UserID: user.ID, Title: "Deleting", ConversationID: "conv-deleting", Status: "deleting"}
	for _, project := range []*Project{archived, deleting} {
		if err := store.CreateProject(t.Context(), project); err != nil {
			t.Fatal(err)
		}
		granted, err := store.GrantProjectProduction(t.Context(), user.ID, project.ID, "cs_"+project.ID, time.Now().UTC().Add(30*24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if granted {
			t.Fatalf("production grant for %s was applied, want ignored for status %s", project.ID, project.Status)
		}
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

func TestPublicProjectErrorMessageExplainsRuntimeBilling(t *testing.T) {
	got := publicProjectErrorMessageFromError(&fibe.PlatformError{Code: "INTERNAL_ERROR", Status: 402, Message: "unexpected status 402"})
	want := "The workspace runtime is not funded. Ask an admin to fund the linked Fibe workspace, then retry starting the project."
	if got != want {
		t.Fatalf("message=%q, want %q", got, want)
	}
}

func TestAdminProjectDiagnosticsExposeInternalProjectError(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "likeable.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.UpsertUser(t.Context(), "project-internal-error@example.com", "Internal Error", "")
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{
		ID:             "project-internal-error",
		UserID:         user.ID,
		Title:          "Internal Error",
		ConversationID: "conv-internal-error",
		Status:         "creating",
	}
	if err := store.CreateProject(t.Context(), project); err != nil {
		t.Fatal(err)
	}
	rawErr := &fibe.PlatformError{Code: "INTERNAL_ERROR", Status: 422, Message: "unexpected status 422"}
	if err := store.UpdateProjectErrorFromError(t.Context(), project.ID, user.ID, rawErr); err != nil {
		t.Fatal(err)
	}

	stored, err := store.ProjectForUser(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored.ErrorMessage, "unexpected status 422") {
		t.Fatalf("public error_message=%q should stay sanitized", stored.ErrorMessage)
	}
	diagnostics, err := store.AdminProjectDiagnostics(t.Context(), user.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagnostics.Internal.InternalErrorMessage, "unexpected status 422") {
		t.Fatalf("internal_error_message=%q, want raw platform cause", diagnostics.Internal.InternalErrorMessage)
	}
}
