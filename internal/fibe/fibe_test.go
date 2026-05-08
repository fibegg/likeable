package fibe

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	projecttext "github.com/fibegg/likeable/internal/project"
)

func testFibeBaseURL() string {
	return firstNonEmpty(os.Getenv("FIBE_URL"), "server.test:3000")
}

func testFibeNormalizedBaseURL() string {
	return normalizeFibeBaseURL(testFibeBaseURL())
}

func testFibeCLIDomain() string {
	return fibeCLIDomain(testFibeNormalizedBaseURL())
}

func TestCreateGreenfieldUsesTemplateVersionIDOnlyWhenConfigured(t *testing.T) {
	for _, tc := range []struct {
		name              string
		templateVersionID string
		wantPresent       bool
	}{
		{name: "omitted", templateVersionID: "", wantPresent: false},
		{name: "present", templateVersionID: "42", wantPresent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cliPath, logPath, _ := fakeFibeCLI(t)

			client := &Client{
				apiKey:            "test",
				agentID:           "agent",
				templateVersionID: tc.templateVersionID,
				cliPath:           cliPath,
				cliDomain:         testFibeCLIDomain(),
				http:              http.DefaultClient,
			}
			project := &Project{
				ID:             "01234567-89ab-cdef-0123-456789abcdef",
				Title:          "Test app",
				ConversationID: "likeable-0123456789abcdef0123456789abcdef",
			}
			result, err := client.CreateGreenfield(t.Context(), project)
			if err != nil {
				t.Fatal(err)
			}
			if result.PlaygroundID != "123" || result.PlayspecID != "456" || result.PropID != "789" {
				t.Fatalf("parsed ids=%+v, want playground=123 playspec=456 prop=789", result)
			}
			log := readFile(t, logPath)
			if strings.Contains(log, "--template-id") {
				t.Fatal("greenfield CLI must not include --template-id")
			}
			hasTemplateVersionID := strings.Contains(log, "--template-version-id")
			if hasTemplateVersionID != tc.wantPresent {
				t.Fatalf("--template-version-id present=%v, want %v; log=%s", hasTemplateVersionID, tc.wantPresent, log)
			}
			if tc.wantPresent && !strings.Contains(log, "--template-version-id 42") {
				t.Fatalf("log=%s, want --template-version-id 42", log)
			}
			if !strings.Contains(log, "--private") {
				t.Fatalf("log=%s, want private greenfield", log)
			}
			if !strings.Contains(log, "--var subdomain=lk-0123456789abcdef") {
				t.Fatalf("log=%s, want subdomain variable", log)
			}
			if !strings.Contains(log, "--var ws_subdomain=ws-lk-0123456789abcdef") {
				t.Fatalf("log=%s, want websocket subdomain variable", log)
			}
		})
	}
}

func TestGreenfieldSubdomainsAreStableDNSLabels(t *testing.T) {
	a := projecttext.PreviewSubdomain(&Project{ID: "9447f6b8-3d10-47d6-a946-0afc6bdd4406"})
	b := projecttext.PreviewSubdomain(&Project{ID: "f57dc08b-5e22-4b84-bdf4-e77ca7b94c99"})
	if a != "lk-9447f6b83d1047d6" {
		t.Fatalf("subdomain=%q, want stable UUID-derived value", a)
	}
	if a == b {
		t.Fatalf("different project IDs produced same subdomain %q", a)
	}
	for _, subdomain := range []string{a, "ws-" + a, b, "ws-" + b} {
		if len(subdomain) > 63 {
			t.Fatalf("subdomain %q length=%d exceeds DNS label limit", subdomain, len(subdomain))
		}
		for _, r := range subdomain {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				t.Fatalf("subdomain %q contains non-DNS-safe rune %q", subdomain, r)
			}
		}
	}
}

func TestFibeAssignmentPoolMapsEmailDeterministically(t *testing.T) {
	pool := []Assignment{
		{Label: "A", AgentID: "agent-a", MarqueeID: "marquee-a"},
		{Label: "B", AgentID: "agent-b", MarqueeID: "marquee-b"},
	}
	reversed := []Assignment{pool[1], pool[0]}

	first, ok := selectAssignment("Pilot@Example.COM", pool)
	if !ok {
		t.Fatal("expected assignment")
	}
	second, ok := selectAssignment("pilot@example.com", reversed)
	if !ok {
		t.Fatal("expected assignment from reversed pool")
	}
	if first != second {
		t.Fatalf("assignment changed with case/order: first=%+v second=%+v", first, second)
	}
}

func TestFibeAssignmentFallsBackToGlobalConfig(t *testing.T) {
	assignment, err := AssignmentForNewProject(map[string]string{
		"fibe_agent_id":   "global-agent",
		"fibe_marquee_id": "global-marquee",
	}, "pilot@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if assignment.AgentID != "global-agent" || assignment.MarqueeID != "global-marquee" {
		t.Fatalf("assignment=%+v, want global pair", assignment)
	}
}

func TestParseGreenfieldStatusPrefersAppPreviewURL(t *testing.T) {
	result := parseGreenfieldStatus(map[string]any{
		"status": "success",
		"service_urls": []any{
			map[string]any{"name": "ws", "type": "static", "url": "http://ws-starter.phoenix.test", "visibility": "external"},
			map[string]any{"name": "app", "type": "dynamic", "url": "http://starter.phoenix.test", "visibility": "external"},
		},
	})
	if result.PreviewURL != "http://starter.phoenix.test" {
		t.Fatalf("PreviewURL=%q, want app URL", result.PreviewURL)
	}
}

func TestFrameBlockingHeaderDetectsIframeBlockers(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Frame-Options", "SAMEORIGIN")
	if got := frameBlockingHeader(headers); got == "" {
		t.Fatal("expected X-Frame-Options to be treated as frame-blocking")
	}
}

func TestSendMessagePassesConversationAttachmentsThroughCLI(t *testing.T) {
	cliPath, logPath, stdinPath := fakeFibeCLI(t)
	attachmentPath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(attachmentPath, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &Client{
		apiKey:    "test",
		agentID:   "agent",
		cliPath:   cliPath,
		cliDomain: testFibeCLIDomain(),
		http:      http.DefaultClient,
	}
	if err := client.SendMessage(t.Context(), "conv-1", "Use attachment", []string{attachmentPath}, "steer"); err != nil {
		t.Fatal(err)
	}
	log := readFile(t, logPath)
	if !strings.Contains(log, "agents send-message agent --conversation-id conv-1 --busy-policy steer --attach "+attachmentPath+" -f -") {
		t.Fatalf("unexpected CLI args: %s", log)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(readFile(t, stdinPath)), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["text"] != "Use attachment" {
		t.Fatalf("payload=%#v, want text", payload)
	}
}

func TestConversationLiveStateFetchesRuntimeStreamState(t *testing.T) {
	cliPath, logPath, _ := fakeFibeCLI(t)

	client := &Client{
		apiKey:    "test",
		agentID:   "agent",
		cliPath:   cliPath,
		cliDomain: testFibeCLIDomain(),
		http:      http.DefaultClient,
	}
	live, err := client.ConversationLiveState(t.Context(), "conv-1")
	if err != nil {
		t.Fatal(err)
	}
	if !live.IsProcessing || live.StreamText == "" || live.QueuedTurns != 1 {
		t.Fatalf("live=%+v, want processing stream state", live)
	}
	if !strings.Contains(readFile(t, logPath), "agents live-state agent --conversation-id conv-1") {
		t.Fatalf("live-state command was not scoped to conversation: %s", readFile(t, logPath))
	}
}

func TestDeleteProjectResourcesDeletesFibeAndGiteaResources(t *testing.T) {
	var paths []string
	cliPath, logPath, _ := fakeFibeCLI(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch r.Method + " " + r.URL.Path {
		case http.MethodDelete + " /api/v1/repos/owner/repo":
			if got := r.Header.Get("Authorization"); got != "token gitea-token" {
				t.Fatalf("Authorization=%q, want token gitea-token", got)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := &Client{
		baseURL:   server.URL,
		apiKey:    "test",
		agentID:   "agent",
		cliPath:   cliPath,
		cliDomain: testFibeCLIDomain(),
		http:      server.Client(),
	}
	err := client.DeleteProjectResources(t.Context(), &Project{
		PlaygroundID:   "123",
		PlayspecID:     "456",
		PropID:         "789",
		RepoURL:        server.URL + "/owner/repo.git",
		ConversationID: "likeable-123",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"DELETE /api/v1/repos/owner/repo",
	} {
		if !containsString(paths, path) {
			t.Fatalf("missing request %s; got %v", path, paths)
		}
	}
	log := readFile(t, logPath)
	for _, want := range []string{
		"agents gitea-token agent",
		"playgrounds delete 123",
		"playspecs delete 456",
		"props delete 789",
		"agents delete-conversation agent --conversation-id likeable-123",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("missing CLI command %q; log=%s", want, log)
		}
	}
}

func fakeFibeCLI(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fibe")
	logPath := filepath.Join(dir, "commands.log")
	stdinPath := filepath.Join(dir, "stdin.json")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + logPath + `"
case "$*" in
  *"greenfield"*)
    echo '{"status":"success","playground":{"id":123},"playspec":{"id":456},"prop":{"id":789},"repo":{"repository_url":"http://gitea.test/owner/repo.git"},"service_urls":[{"name":"app","type":"dynamic","url":"http://lk-test.phoenix.test","visibility":"external"}]}'
    ;;
  *"playgrounds get"*)
    echo '{"id":123,"status":"running"}'
    ;;
  *"wait playground"*)
    echo '{"status":"running"}'
    ;;
  *"agents send-message"*)
    cat > "` + stdinPath + `"
    echo '{"ok":true}'
    ;;
  *"agents live-state"*)
    echo '{"conversationId":"conv-1","isProcessing":true,"streamText":"[[LIKEABLE_NOTIFICATION_START]]Building[[LIKEABLE_NOTIFICATION_END]]","queuedTurns":1}'
    ;;
  *"agents gitea-token"*)
    echo '{"token":"gitea-token","username":"agent"}'
    ;;
  *"agents create-conversation"*|*"agents delete-conversation"*|*"agents interrupt"*|*"agents messages"*|*"agents activity"*|*"playgrounds delete"*|*"playspecs delete"*|*"props delete"*)
    echo '{"ok":true,"content":[]}'
    ;;
  *)
    echo "unexpected command: $*" >&2
    exit 64
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return path, logPath, stdinPath
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
