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
