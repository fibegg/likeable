package workspace

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCreateGreenfieldCreatesLocalWorkspace(t *testing.T) {
	root := t.TempDir()
	client, err := NewClient(Config{
		BaseURL:       "https://likeable.example.test",
		WorkspaceRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	project := &Project{ID: "project-123", Title: "Kitchen Planner"}

	result, err := client.CreateGreenfield(t.Context(), project)
	if err != nil {
		t.Fatal(err)
	}

	if result.PlaygroundID == "" || result.RepoURL != "local://project-123" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.PreviewURL != "https://likeable.example.test/api/projects/project-123/preview/" {
		t.Fatalf("preview URL=%q", result.PreviewURL)
	}
	for _, name := range []string{"index.html", "styles.css", "app.js"} {
		if _, err := os.Stat(filepath.Join(root, "project-123", name)); err != nil {
			t.Fatalf("%s was not created: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "project-123", metadataDirName)); err != nil {
		t.Fatalf("metadata dir was not created: %v", err)
	}
}

func TestSendMessageAppliesOpenAIGeneratedFiles(t *testing.T) {
	root := t.TempDir()
	openAICalls := 0
	generated, err := json.Marshal(generatedAppResponse{
		Summary: "Built the first screen.",
		Files: []generatedFile{
			{Path: "index.html", Content: "<!doctype html><h1>Generated</h1>"},
			{Path: "styles.css", Content: "body { color: #123456; }"},
			{Path: "app.js", Content: "window.generated = true;\n"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	responseBody, err := json.Marshal(map[string]any{"output_text": string(generated)})
	if err != nil {
		t.Fatal(err)
	}
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		openAICalls++
		if req.URL.String() != openAIResponsesEndpoint {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer test-openai-key" {
			t.Fatalf("authorization header=%q", req.Header.Get("Authorization"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(responseBody))),
			Request:    req,
		}, nil
	})}
	project := &Project{ID: "project-generated", Title: "Starter App"}
	client, err := NewClient(Config{
		BaseURL:       "https://likeable.example.test",
		WorkspaceRoot: root,
		OpenAIAPIKey:  "test-openai-key",
		HTTP:          httpClient,
		Project:       project,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateGreenfield(t.Context(), project); err != nil {
		t.Fatal(err)
	}

	if err := client.SendMessage(t.Context(), "conversation-1", "Build it", nil, "queue"); err != nil {
		t.Fatal(err)
	}

	eventually(t, time.Second, func() bool {
		data, err := os.ReadFile(filepath.Join(root, "project-generated", "index.html"))
		return err == nil && strings.Contains(string(data), "<h1>Generated</h1>")
	})
	messages, err := client.Messages(t.Context(), "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	if openAICalls != 1 {
		t.Fatalf("OpenAI calls=%d, want 1", openAICalls)
	}
	if len(messages) == 0 {
		t.Fatal("assistant message was not recorded")
	}
}

func TestCleanGeneratedPathRejectsEscapesAndReservedDirs(t *testing.T) {
	for _, raw := range []string{"/abs.html", "../secret.txt", ".likeable/live.json", ".git/config"} {
		if _, err := cleanGeneratedPath(raw); err == nil {
			t.Fatalf("cleanGeneratedPath(%q) succeeded, want error", raw)
		}
	}
	if got, err := cleanGeneratedPath("assets/../index.html"); err != nil || got != "index.html" {
		t.Fatalf("cleanGeneratedPath normalized to %q, %v", got, err)
	}
}

func TestResponseOutputTextSupportsOutputArray(t *testing.T) {
	raw := []byte(`{"output":[{"content":[{"type":"output_text","text":"first"},{"type":"output_text","text":"second"}]}]}`)
	if got := responseOutputText(raw); got != "first\nsecond" {
		t.Fatalf("responseOutputText=%q", got)
	}
}

func eventually(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		if check() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("condition was not met before timeout")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
