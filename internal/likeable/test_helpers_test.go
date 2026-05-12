package likeable

import (
	"os"
	"path/filepath"
	"testing"
)

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
	return path, stdinPath
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
