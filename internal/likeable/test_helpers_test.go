package likeable

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	base := http.DefaultTransport
	http.DefaultTransport = &fakeFibeTransport{base: base, cfg: fakeFibeTransportConfig{Mode: "default"}}
	code := m.Run()
	http.DefaultTransport = base
	os.Exit(code)
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
  *"playgrounds debug"*)
    echo '{"diagnostics":{"playground":{"id":123,"playspec_id":456,"status":"running"},"routes":[{"service":"app","type":"dynamic","visibility":"external","url":"http://lk-test.phoenix.test"}]}}'
    ;;
  *"playspecs get"*)
    echo '{"id":456,"source_template":{"id":321,"name":"delete-all-abc12345"},"source_template_version_id":654,"services":[{"name":"app","prop_id":789,"repo_url":"http://gitea.test/owner/repo.git","source_repo_url":"https://github.com/fibegg/go-fibe-app"}]}'
    ;;
  *"templates versions list"*)
    echo '{"Data":[{"id":654,"source":{"prop_id":789,"prop_repository_url":"http://gitea.test/owner/repo.git"}}]}'
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
  *"agents create-conversation"*|*"agents start-chat"*|*"agents delete-conversation"*|*"agents interrupt"*|*"agents messages"*|*"agents activity"*|*"playgrounds delete"*|*"playgrounds start"*|*"playgrounds stop"*|*"playgrounds hard-restart"*|*"playspecs delete"*|*"templates versions destroy"*|*"templates delete"*|*"props delete"*)
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
	installFakeFibeTransport(t, fakeFibeTransportConfig{
		Mode:      "default",
		LogPath:   logPath,
		StdinPath: stdinPath,
	})
	return path, logPath, stdinPath
}

func fakeTransformedFibeCLI(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fibe")
	stdinPath := filepath.Join(dir, "stdin.json")
	script := `#!/bin/sh
case "$*" in
  *"playgrounds debug 321"*)
    echo '{"diagnostics":{"playground":{"id":321,"playspec_id":654,"status":"running"},"routes":[{"service":"frontend","type":"dynamic","visibility":"external","url":"http://frontend.example.test"},{"service":"api","type":"dynamic","visibility":"external","url":"http://api.example.test"}]}}'
    ;;
  *"playspecs get 654"*)
    echo '{"id":654,"source_template":{"id":900,"name":"project-transform"},"source_template_version_id":901,"services":[{"name":"frontend","prop_id":81,"propID":81,"repo_url":"http://gitea.test/owner/frontend.git","repository_url":"http://gitea.test/owner/frontend.git","source_repo_url":"https://github.com/fibegg/custom-frontend"},{"name":"api","prop_id":82,"propID":82,"repo_url":"http://gitea.test/owner/api.git","repository_url":"http://gitea.test/owner/api.git","source_repo_url":"https://github.com/fibegg/custom-api"}]}'
    ;;
  *"agents messages"*|*"agents activity"*)
    echo '{"content":[]}'
    ;;
  *"agents live-state"*)
    echo '{"conversationId":"conv-trns","isProcessing":false,"streamText":"","queuedTurns":0}'
    ;;
  *"agents send-message"*)
    cat > "` + stdinPath + `"
    echo '{"ok":true}'
    ;;
  *"agents create-conversation"*)
    echo '{"ok":true}'
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
	installFakeFibeTransport(t, fakeFibeTransportConfig{
		Mode:      "transformed",
		StdinPath: stdinPath,
	})
	return path, stdinPath
}

func fakeProjectStateFibeCLI(t *testing.T, status, previewURL string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fibe")
	script := fmt.Sprintf(`#!/bin/sh
case "$*" in
  *"playgrounds get 321"*)
    echo '{"id":321,"status":%q}'
    ;;
  *"playgrounds debug 321"*)
    echo '{"diagnostics":{"playground":{"id":321,"playspec_id":654,"status":%q},"routes":[{"service":"app","type":"dynamic","visibility":"external","url":%q}]}}'
    ;;
  *"playspecs get 654"*)
    echo '{"id":654,"services":[{"name":"app","prop_id":81,"repo_url":"http://gitea.test/owner/app.git","source_repo_url":"https://github.com/fibegg/go-fibe-app"}]}'
    ;;
  *)
    echo "unexpected command: $*" >&2
    exit 64
    ;;
esac
`, status, status, previewURL)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	installFakeFibeTransport(t, fakeFibeTransportConfig{
		Mode:       "project-state",
		Status:     status,
		PreviewURL: previewURL,
	})
	return path
}

func fakeAlreadyStoppedFibeCLI(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fibe")
	logPath := filepath.Join(dir, "commands.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + logPath + `"
case "$*" in
  *"playgrounds stop"*)
    echo '{"error":{"code":"INVALID_STATE","status":422,"message":"Cannot stop playground from current status"}}' >&2
    exit 1
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
	installFakeFibeTransport(t, fakeFibeTransportConfig{Mode: "already-stopped", LogPath: logPath})
	return path, logPath
}

func fakeMissingPlaygroundFibeCLI(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fibe")
	logPath := filepath.Join(dir, "commands.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + logPath + `"
case "$*" in
  *"playgrounds stop"*)
    echo '{"error":{"code":"INTERNAL_ERROR","status":404,"message":"unexpected status 404"}}' >&2
    exit 1
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
	installFakeFibeTransport(t, fakeFibeTransportConfig{Mode: "missing-playground", LogPath: logPath})
	return path, logPath
}

type fakeFibeTransportConfig struct {
	Mode        string
	LogPath     string
	StdinPath   string
	MarkerPath  string
	ReleasePath string
	Status      string
	PreviewURL  string
}

type fakeFibeTransport struct {
	base http.RoundTripper
	cfg  fakeFibeTransportConfig
}

func installFakeFibeTransport(t *testing.T, cfg fakeFibeTransportConfig) {
	t.Helper()
	base := http.DefaultTransport
	http.DefaultTransport = &fakeFibeTransport{base: base, cfg: cfg}
	t.Cleanup(func() {
		http.DefaultTransport = base
	})
}

func fakeFibeHTTPClient(base *http.Client, cfg fakeFibeTransportConfig) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	clone := *base
	transport := clone.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clone.Transport = &fakeFibeTransport{base: transport, cfg: cfg}
	return &clone
}

func (rt *fakeFibeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "gitea.test" && req.Method == http.MethodDelete {
		return fakeHTTPResponse(req, http.StatusNoContent, ""), nil
	}
	if strings.HasSuffix(req.URL.Hostname(), ".phoenix.test") || req.URL.Hostname() == "phoenix.test" {
		return fakeHTTPResponse(req, http.StatusOK, "<!doctype html><title>ready</title>"), nil
	}
	if req.URL.Host != "server.test:3000" {
		return rt.base.RoundTrip(req)
	}
	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return rt.roundTripWithBody(req, bodyBytes)
	}
	return rt.roundTripWithBody(req, nil)
}

func (rt *fakeFibeTransport) roundTripWithBody(req *http.Request, bodyBytes []byte) (*http.Response, error) {
	path := req.URL.Path
	methodPath := req.Method + " " + path
	switch rt.cfg.Mode {
	case "transformed":
		return rt.roundTripTransformed(req, methodPath, bodyBytes), nil
	case "project-state":
		return rt.roundTripProjectState(req, methodPath), nil
	case "already-stopped":
		return rt.roundTripAlreadyStopped(req, methodPath, bodyBytes), nil
	case "missing-playground":
		return rt.roundTripMissingPlayground(req, methodPath, bodyBytes), nil
	case "hydration-fail":
		if req.Method == http.MethodDelete && strings.Contains(req.URL.Path, "playground-bad") {
			return fakeJSONResponse(req, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "BAD_REQUEST", "message": "delete failed after debug failed"}}), nil
		}
		if strings.HasSuffix(req.URL.Path, "/debug") {
			return fakeJSONResponse(req, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "UNKNOWN_ERROR", "message": "debug failed"}}), nil
		}
		return rt.roundTripDefault(req, methodPath, bodyBytes), nil
	default:
		return rt.roundTripDefault(req, methodPath, bodyBytes), nil
	}
}

func (rt *fakeFibeTransport) roundTripDefault(req *http.Request, methodPath string, body []byte) *http.Response {
	path := req.URL.Path
	agentID := agentIDFromPath(path)
	conversationID := firstNonEmpty(req.URL.Query().Get("conversation_id"), conversationIDFromBody(body))
	switch {
	case methodPath == "POST /api/greenfields":
		rt.log("greenfield")
		return fakeJSONResponse(req, http.StatusOK, map[string]any{
			"playground":   map[string]any{"id": 123},
			"playspec":     map[string]any{"id": 456},
			"prop":         map[string]any{"id": 789},
			"repo":         map[string]any{"repository_url": "http://gitea.test/owner/repo.git"},
			"service_urls": []map[string]any{{"name": "app", "type": "dynamic", "url": "http://lk-test.phoenix.test", "visibility": "external"}},
		})
	case methodPath == "GET /api/playgrounds/123/status":
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"id": 123, "status": "running"})
	case methodPath == "GET /api/playgrounds/123", strings.HasPrefix(methodPath, "GET /api/playgrounds/playground-"):
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"id": 123, "status": "running"})
	case methodPath == "GET /api/playgrounds/123/debug", strings.HasSuffix(methodPath, "/debug"):
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"diagnostics": map[string]any{
			"playground": map[string]any{"id": 123, "playspec_id": 456, "status": "running"},
			"routes":     []map[string]any{{"service": "app", "type": "dynamic", "visibility": "external", "url": "http://lk-test.phoenix.test"}},
		}})
	case methodPath == "GET /api/playspecs/456", strings.HasPrefix(methodPath, "GET /api/playspecs/playspec-"):
		return fakeJSONResponse(req, http.StatusOK, map[string]any{
			"id":                         456,
			"source_template":            map[string]any{"id": 321, "name": "delete-all-abc12345"},
			"source_template_version_id": 654,
			"services":                   []map[string]any{{"name": "app", "prop_id": 789, "repo_url": "http://gitea.test/owner/repo.git", "source_repo_url": "https://github.com/fibegg/go-fibe-app"}},
		})
	case methodPath == "GET /api/import_templates/321/versions":
		return fakeJSONResponse(req, http.StatusOK, map[string]any{
			"data": []map[string]any{{"id": 654, "source": map[string]any{"prop_id": 789, "prop_repository_url": "http://gitea.test/owner/repo.git"}}},
			"meta": map[string]any{"page": 1, "per_page": 25, "total": 1},
		})
	case strings.HasSuffix(path, "/uploads") && req.Method == http.MethodPost:
		lowerBody := bytes.ToLower(body)
		if bytes.Contains(lowerBody, []byte("mock.webp")) || bytes.Contains(lowerBody, []byte(".webp")) {
			return fakeJSONResponse(req, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "BAD_REQUEST", "message": "Unsupported or blocked file type"}})
		}
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"filename": "uploaded-file"})
	case strings.HasSuffix(path, "/messages") && req.Method == http.MethodPost:
		rt.blockIfConfigured(req)
		rt.writeStdin(body)
		rt.log(fmt.Sprintf("agents send-message %s --conversation-id %s", agentID, conversationID))
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"ok": true})
	case strings.HasSuffix(path, "/messages") && req.Method == http.MethodGet:
		rt.log(fmt.Sprintf("agents messages %s --conversation-id %s", agentID, conversationID))
		switch conversationID {
		case "conv-feed-rate-limit":
			return fakeJSONResponse(req, http.StatusTooManyRequests, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR", "message": "unexpected status 429"}})
		case "conv-feed-timeout":
			return fakeJSONResponse(req, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "UNKNOWN_ERROR", "message": `Get "https://next.fibe.live/api/agents/83/live_state": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`}})
		case "conv-clean":
			return fakeJSONResponse(req, http.StatusOK, map[string]any{"content": []map[string]any{
				{"role": "assistant", "body": "hidden prose [[LIKEABLE_NOTIFICATION_START]]Checking the preview[[LIKEABLE_NOTIFICATION_END]] more prose [[LIKEABLE_NOTIFICATION_START]]Canvas updated[[LIKEABLE_NOTIFICATION_END]]"},
				{"role": "user", "body": "keep user body"},
			}})
		case "conv-feed-activity-timeout":
			return fakeJSONResponse(req, http.StatusOK, map[string]any{"content": []map[string]any{{"role": "user", "body": "build it"}}})
		}
		if strings.HasPrefix(conversationID, "likeable-prompt-improve-") {
			return fakeJSONResponse(req, http.StatusOK, map[string]any{"content": []map[string]any{{"role": "assistant", "body": promptImproveStart + "\nImprove the existing car sharing webapp by adding a clear vehicle inventory section, more polished ride/request UX, responsive spacing, empty/loading states, and a quick visual verification pass. Keep the current car sharing product intact and do not replace it with another app.\n" + promptImproveEnd}}})
		}
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"content": []any{}})
	case strings.HasSuffix(path, "/activity"):
		rt.log(fmt.Sprintf("agents activity %s --conversation-id %s", agentID, conversationID))
		if conversationID == "conv-feed-activity-timeout" {
			return fakeJSONResponse(req, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "UNKNOWN_ERROR", "message": "signal: killed"}})
		}
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"content": []any{}})
	case strings.HasSuffix(path, "/live_state"):
		rt.log(fmt.Sprintf("agents live-state %s --conversation-id %s", agentID, conversationID))
		switch conversationID {
		case "conv-live-failure":
			return fakeJSONResponse(req, http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"code": "AGENT_COMMUNICATION_FAILED", "message": "Agent unreachable: connection refused"}})
		case "conv-clean":
			return fakeJSONResponse(req, http.StatusOK, map[string]any{"content": map[string]any{"conversationId": "conv-clean", "isProcessing": true, "streamText": "thinking [[LIKEABLE_NOTIFICATION_START]]Updating files[[LIKEABLE_NOTIFICATION_END]] noisy", "queuedTurns": 0}})
		case "conv-auth":
			return fakeJSONResponse(req, http.StatusOK, map[string]any{"content": map[string]any{"conversationId": "conv-auth", "isProcessing": true, "streamText": "Invalid API key - Fix external API key", "queuedTurns": 0}})
		case "conv-feed-activity-timeout":
			return fakeJSONResponse(req, http.StatusOK, map[string]any{"content": map[string]any{"conversationId": "conv-feed-activity-timeout", "isProcessing": false, "streamText": "", "queuedTurns": 0}})
		}
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"content": map[string]any{"conversationId": "conv-1", "isProcessing": true, "streamText": "[[LIKEABLE_NOTIFICATION_START]]Building[[LIKEABLE_NOTIFICATION_END]]", "queuedTurns": 1}})
	case strings.HasSuffix(path, "/gitea_token"):
		rt.log("agents gitea-token " + agentID)
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"token": "gitea-token", "username": "agent"})
	case strings.HasSuffix(path, "/conversations") && req.Method == http.MethodPost:
		rt.log(fmt.Sprintf("agents create-conversation %s --conversation-id %s", agentID, conversationID))
		if conversationID == "conv-offline-agent" {
			return fakeJSONResponse(req, http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"code": "UNPROCESSABLE_ENTITY", "message": "No running AgentChat for Agent#1"}})
		}
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"ok": true})
	case strings.HasSuffix(path, "/conversations") && req.Method == http.MethodDelete:
		rt.log(fmt.Sprintf("agents delete-conversation %s --conversation-id %s", agentID, conversationID))
		return fakeHTTPResponse(req, http.StatusNoContent, "")
	case strings.HasSuffix(path, "/chats"):
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		rt.log(fmt.Sprintf("agents start-chat %s --marquee-id %v", agentID, payload["marquee_id"]))
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"id": 1, "status": "running"})
	case strings.HasSuffix(path, "/interrupts"):
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"ok": true})
	case strings.HasSuffix(path, "/operations"):
		action := actionTypeFromBody(body)
		rt.log(fmt.Sprintf("playgrounds %s %s", strings.ReplaceAll(action, "_", "-"), resourceIDFromPath(path, "playgrounds")))
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"id": 123, "status": "running"})
	case req.Method == http.MethodDelete:
		rt.log(deleteCommandFromPath(path))
		return fakeHTTPResponse(req, http.StatusNoContent, "")
	default:
		return fakeJSONResponse(req, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "RESOURCE_NOT_FOUND", "message": "unexpected fake Fibe request: " + methodPath}})
	}
}

func (rt *fakeFibeTransport) roundTripTransformed(req *http.Request, methodPath string, body []byte) *http.Response {
	switch methodPath {
	case "GET /api/playgrounds/321/debug":
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"diagnostics": map[string]any{
			"playground": map[string]any{"id": 321, "playspec_id": 654, "status": "running"},
			"routes": []map[string]any{
				{"service": "frontend", "type": "dynamic", "visibility": "external", "url": "http://frontend.example.test"},
				{"service": "api", "type": "dynamic", "visibility": "external", "url": "http://api.example.test"},
			},
		}})
	case "GET /api/playspecs/654":
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"id": 654, "services": []map[string]any{
			{"name": "frontend", "prop_id": 81, "repo_url": "http://gitea.test/owner/frontend.git", "repository_url": "http://gitea.test/owner/frontend.git", "source_repo_url": "https://github.com/fibegg/custom-frontend"},
			{"name": "api", "prop_id": 82, "repo_url": "http://gitea.test/owner/api.git", "repository_url": "http://gitea.test/owner/api.git", "source_repo_url": "https://github.com/fibegg/custom-api"},
		}})
	default:
		return rt.roundTripDefault(req, methodPath, body)
	}
}

func (rt *fakeFibeTransport) roundTripProjectState(req *http.Request, methodPath string) *http.Response {
	status := rt.cfg.Status
	if status == "" {
		status = "running"
	}
	switch methodPath {
	case "GET /api/playgrounds/321/status", "GET /api/playgrounds/321":
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"id": 321, "status": status})
	case "GET /api/playgrounds/321/debug":
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"diagnostics": map[string]any{
			"playground": map[string]any{"id": 321, "playspec_id": 654, "status": status},
			"routes":     []map[string]any{{"service": "app", "type": "dynamic", "visibility": "external", "url": rt.cfg.PreviewURL}},
		}})
	case "GET /api/playspecs/654":
		return fakeJSONResponse(req, http.StatusOK, map[string]any{"id": 654, "services": []map[string]any{{"name": "app", "prop_id": 81, "repo_url": "http://gitea.test/owner/app.git", "source_repo_url": "https://github.com/fibegg/go-fibe-app"}}})
	default:
		return fakeJSONResponse(req, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "RESOURCE_NOT_FOUND", "message": "unexpected fake Fibe request: " + methodPath}})
	}
}

func (rt *fakeFibeTransport) roundTripAlreadyStopped(req *http.Request, methodPath string, body []byte) *http.Response {
	if strings.HasSuffix(req.URL.Path, "/operations") {
		rt.log("playgrounds stop " + resourceIDFromPath(req.URL.Path, "playgrounds"))
		return fakeJSONResponse(req, http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"code": "INVALID_STATE", "message": "Cannot stop playground from current status"}})
	}
	return rt.roundTripDefault(req, methodPath, body)
}

func (rt *fakeFibeTransport) roundTripMissingPlayground(req *http.Request, methodPath string, body []byte) *http.Response {
	if strings.HasSuffix(req.URL.Path, "/operations") {
		rt.log("playgrounds stop " + resourceIDFromPath(req.URL.Path, "playgrounds"))
		return fakeJSONResponse(req, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR", "message": "unexpected status 404"}})
	}
	return rt.roundTripDefault(req, methodPath, body)
}

func agentIDFromPath(path string) string {
	return resourceIDFromPath(path, "agents")
}

func resourceIDFromPath(path, resource string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == resource {
			return parts[i+1]
		}
	}
	return ""
}

func conversationIDFromBody(body []byte) string {
	var payload map[string]any
	if len(body) == 0 || json.Unmarshal(body, &payload) != nil {
		return ""
	}
	if value, ok := payload["conversation_id"]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func deleteCommandFromPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/playgrounds/"):
		return "playgrounds delete " + resourceIDFromPath(path, "playgrounds")
	case strings.HasPrefix(path, "/api/playspecs/"):
		return "playspecs delete " + resourceIDFromPath(path, "playspecs")
	case strings.HasPrefix(path, "/api/import_templates/") && strings.Contains(path, "/versions/"):
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 5 {
			return fmt.Sprintf("templates versions destroy %s %s", parts[2], parts[4])
		}
	case strings.HasPrefix(path, "/api/import_templates/"):
		return "templates delete " + resourceIDFromPath(path, "import_templates")
	case strings.HasPrefix(path, "/api/props/"):
		return "props delete " + resourceIDFromPath(path, "props")
	}
	return strings.TrimPrefix(path, "/api/") + " delete"
}

func actionTypeFromBody(body []byte) string {
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)
	return fmt.Sprint(payload["action_type"])
}

func (rt *fakeFibeTransport) log(line string) {
	if rt.cfg.LogPath == "" {
		return
	}
	f, err := os.OpenFile(rt.cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line + "\n")
}

func (rt *fakeFibeTransport) writeStdin(body []byte) {
	if rt.cfg.StdinPath == "" {
		return
	}
	_ = os.WriteFile(rt.cfg.StdinPath, body, 0o600)
}

func (rt *fakeFibeTransport) blockIfConfigured(req *http.Request) {
	if rt.cfg.MarkerPath == "" || rt.cfg.ReleasePath == "" {
		return
	}
	_ = os.WriteFile(rt.cfg.MarkerPath, []byte("started"), 0o600)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(rt.cfg.ReleasePath); err == nil {
			return
		}
		select {
		case <-req.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func fakeJSONResponse(req *http.Request, status int, value any) *http.Response {
	data, _ := json.Marshal(value)
	return fakeHTTPResponse(req, status, string(data))
}

func fakeHTTPResponse(req *http.Request, status int, body string) *http.Response {
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     header,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Request:    req,
	}
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func writeJSONStatus(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
