package project

import (
	"testing"

	"github.com/fibegg/likeable/internal/domain"
)

func TestSourceNameForProjectIsDeterministic(t *testing.T) {
	project := &domain.Project{ID: "01234567-89ab-cdef-0123-456789abcdef", Title: "Test App"}
	first := SourceNameForProject(project)
	second := SourceNameForProject(project)
	if first != "test-app-0123456789abcdef" {
		t.Fatalf("source name=%q, want deterministic project-derived name", first)
	}
	if first != second {
		t.Fatalf("source name changed: %q then %q", first, second)
	}
	other := SourceNameForProject(&domain.Project{ID: "fedcba98-7654-3210-fedc-ba9876543210", Title: "Test App"})
	if other == first {
		t.Fatalf("different project IDs produced same source name %q", first)
	}
}
