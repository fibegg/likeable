package workspace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	metadataDirName          = ".likeable"
	workspaceMessagesFile    = "messages.jsonl"
	workspaceActivityFile    = "activity.jsonl"
	workspaceLiveFile        = "live.json"
	localProviderName        = "local"
	defaultPreviewService    = "app"
	openAIResponsesEndpoint  = "https://api.openai.com/v1/responses"
	maxWorkspaceFileBytes    = 512 << 10
	maxWorkspacePromptBytes  = 160 << 10
	maxGeneratedFileBytes    = 2 << 20
	likeableNotificationOpen = "[[LIKEABLE_NOTIFICATION_START]]"
	likeableNotificationDone = "[[LIKEABLE_NOTIFICATION_END]]"
)

type generatedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type generatedAppResponse struct {
	Files   []generatedFile `json:"files"`
	Summary string          `json:"summary"`
}

func (c *Client) AgentID() string {
	if c.project != nil && strings.TrimSpace(c.project.AgentID) != "" {
		return c.project.AgentID
	}
	return "local-agent"
}

func (c *Client) MarqueeID() string {
	if c.project != nil && strings.TrimSpace(c.project.MarqueeID) != "" {
		return c.project.MarqueeID
	}
	return "local"
}

func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) CreateGreenfield(ctx context.Context, project *Project) (*GreenfieldResult, error) {
	if project == nil {
		return nil, errors.New("project is required")
	}
	c.project = project
	if err := c.ensureWorkspace(ctx, project); err != nil {
		return nil, err
	}
	return c.resultForProject(project), nil
}

func (c *Client) GreenfieldByPlaygroundID(ctx context.Context, playgroundID string) (*GreenfieldResult, error) {
	if c.project == nil {
		return nil, errors.New("project context is required")
	}
	if strings.TrimSpace(playgroundID) == "" {
		return nil, errors.New("workspace id is not available")
	}
	if _, err := os.Stat(c.workspaceDir(c.project.ID)); err != nil {
		return nil, err
	}
	return c.resultForProject(c.project), nil
}

func (c *Client) GreenfieldByPlaygroundName(ctx context.Context, playgroundName string) (*GreenfieldResult, error) {
	if c.project == nil {
		return nil, errors.New("project context is required")
	}
	return c.resultForProject(c.project), nil
}

func (c *Client) FindGreenfieldBySubdomain(ctx context.Context, subdomain string) (*GreenfieldResult, error) {
	if c.project == nil {
		return nil, errors.New("project context is required")
	}
	return c.resultForProject(c.project), nil
}

func (c *Client) EnsureConversation(ctx context.Context, conversationID, title string) error {
	if c.project == nil {
		return nil
	}
	return c.ensureMetadataDir(c.project.ID)
}

func (c *Client) CreateConversation(ctx context.Context, conversationID, title string) error {
	return c.EnsureConversation(ctx, conversationID, title)
}

func (c *Client) DeleteConversation(ctx context.Context, conversationID string) error {
	return nil
}

func (c *Client) StartAgentChat(ctx context.Context) error {
	return nil
}

func (c *Client) SendMessage(ctx context.Context, conversationID, text string, attachmentPaths []string, busyPolicy string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if strings.TrimSpace(c.openAIAPIKey) == "" {
		return &PlatformError{Code: "OPENAI_API_KEY_MISSING", Status: http.StatusServiceUnavailable, Message: "OpenAI API key is not configured"}
	}
	if c.project == nil {
		return errors.New("project context is required")
	}
	if err := c.ensureMetadataDir(c.project.ID); err != nil {
		return err
	}
	projectSnapshot := *c.project
	clientSnapshot := *c
	if strings.Contains(text, "LIKEABLE_PROMPT_IMPROVE_START") {
		return clientSnapshot.runPromptImprove(ctx, conversationID, &projectSnapshot, text)
	}
	go func() {
		runCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if err := clientSnapshot.runProjectGeneration(runCtx, conversationID, &projectSnapshot, text, attachmentPaths); err != nil {
			log.Printf("local workspace generation failed project=%s: %v", projectSnapshot.ID, err)
		}
	}()
	return nil
}

func (c *Client) Interrupt(ctx context.Context, conversationID string) error {
	if c.project == nil {
		return nil
	}
	live := &ConversationLiveState{ConversationID: conversationID, IsProcessing: false, StreamText: "", QueuedTurns: 0}
	return c.writeLive(c.project.ID, live)
}

func (c *Client) StartPlayground(ctx context.Context, playgroundID string) error {
	if c.project == nil {
		return nil
	}
	err := os.Remove(filepath.Join(c.workspaceDir(c.project.ID), metadataDirName, "stopped"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (c *Client) StopPlayground(ctx context.Context, playgroundID string) error {
	if c.project == nil {
		return nil
	}
	if err := c.ensureMetadataDir(c.project.ID); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.workspaceDir(c.project.ID), metadataDirName, "stopped"), []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0o600)
}

func (c *Client) RestartPlayground(ctx context.Context, playgroundID string) error {
	return c.StartPlayground(ctx, playgroundID)
}

func (c *Client) Messages(ctx context.Context, conversationID string) ([]any, error) {
	if c.project == nil {
		return []any{}, nil
	}
	return c.readJSONL(c.project.ID, workspaceMessagesFile)
}

func (c *Client) Activity(ctx context.Context, conversationID string) ([]any, error) {
	if c.project == nil {
		return []any{}, nil
	}
	return c.readJSONL(c.project.ID, workspaceActivityFile)
}

func (c *Client) ConversationLiveState(ctx context.Context, conversationID string) (*ConversationLiveState, error) {
	if c.project == nil {
		return &ConversationLiveState{ConversationID: conversationID}, nil
	}
	path := filepath.Join(c.workspaceDir(c.project.ID), metadataDirName, workspaceLiveFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &ConversationLiveState{ConversationID: conversationID, IsProcessing: false}, nil
	}
	if err != nil {
		return nil, err
	}
	var live ConversationLiveState
	if err := json.Unmarshal(data, &live); err != nil {
		return nil, err
	}
	if live.ConversationID == "" {
		live.ConversationID = conversationID
	}
	return &live, nil
}

func (c *Client) DeleteProjectResources(ctx context.Context, project *Project) error {
	if project == nil {
		project = c.project
	}
	if project == nil {
		return nil
	}
	return os.RemoveAll(c.workspaceDir(project.ID))
}

func (c *Client) GiteaToken(ctx context.Context) (map[string]string, error) {
	return nil, errors.New("local workspaces do not use Gitea")
}

func (c *Client) WorkspaceDir(projectID string) string {
	return c.workspaceDir(projectID)
}

func (c *Client) resultForProject(project *Project) *GreenfieldResult {
	playgroundID := firstNonEmpty(project.PlaygroundID, "local-"+shortProjectID(project.ID))
	playgroundName := firstNonEmpty(project.PlaygroundName, "likeable-"+shortProjectID(project.ID))
	previewURL := c.previewURL(project.ID)
	repoURL := "local://" + project.ID
	return &GreenfieldResult{
		PlaygroundID:        playgroundID,
		PlaygroundName:      playgroundName,
		PlaygroundStatus:    "ready",
		RepoURL:             repoURL,
		PreviewURL:          previewURL,
		SelectedServiceName: defaultPreviewService,
		Repositories: []GreenfieldRepository{{
			Role:          "source",
			RepoURL:       repoURL,
			SourceRepoURL: "",
			Provider:      localProviderName,
			ServiceNames:  []string{defaultPreviewService},
		}},
		Services: []GreenfieldService{{
			Name:       defaultPreviewService,
			URL:        previewURL,
			Type:       "static",
			Visibility: "public",
		}},
	}
}

func (c *Client) ensureWorkspace(ctx context.Context, project *Project) error {
	dir := c.workspaceDir(project.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := c.ensureMetadataDir(project.ID); err != nil {
		return err
	}
	indexPath := filepath.Join(dir, "index.html")
	if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(indexPath, []byte(defaultIndexHTML(project.Title)), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "styles.css"), []byte(defaultStylesCSS()), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte(defaultAppJS(project.Title)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) ensureMetadataDir(projectID string) error {
	return os.MkdirAll(filepath.Join(c.workspaceDir(projectID), metadataDirName), 0o700)
}

func (c *Client) workspaceDir(projectID string) string {
	cleanID := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, strings.TrimSpace(projectID))
	if cleanID == "" {
		cleanID = "unknown"
	}
	return filepath.Join(c.workspaceRoot, cleanID)
}

func (c *Client) previewURL(projectID string) string {
	return c.baseURL + "/api/projects/" + projectID + "/preview/"
}

func shortProjectID(projectID string) string {
	projectID = strings.ReplaceAll(strings.TrimSpace(projectID), "-", "")
	if len(projectID) > 12 {
		return projectID[:12]
	}
	if projectID == "" {
		return "project"
	}
	return projectID
}

func (c *Client) runProjectGeneration(ctx context.Context, conversationID string, project *Project, text string, attachmentPaths []string) error {
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if err := c.appendActivity(project.ID, map[string]any{
		"id":          "local-start-" + startedAt,
		"type":        "status",
		"message":     likeableNotificationOpen + "Applying request" + likeableNotificationDone,
		"occurred_at": startedAt,
	}); err != nil {
		return err
	}
	if err := c.writeLive(project.ID, &ConversationLiveState{
		ConversationID: conversationID,
		IsProcessing:   true,
		StreamText:     likeableNotificationOpen + "Applying request",
		StartedAt:      startedAt,
	}); err != nil {
		return err
	}
	defer func() {
		_ = c.writeLive(project.ID, &ConversationLiveState{ConversationID: conversationID, IsProcessing: false, StreamText: "", StartedAt: startedAt})
	}()

	current, err := c.workspaceSnapshot(project.ID)
	if err != nil {
		return err
	}
	responseText, err := c.openAIText(ctx, appBuilderInstructions(), appBuilderInput(project, text, current, attachmentPaths))
	if err != nil {
		_ = c.appendAssistantMessage(project.ID, conversationID, likeableNotificationOpen+"Build agent failed: "+err.Error()+likeableNotificationDone)
		return err
	}
	generated, err := parseGeneratedAppResponse(responseText)
	if err != nil {
		_ = c.appendAssistantMessage(project.ID, conversationID, likeableNotificationOpen+"Build agent returned an invalid patch."+likeableNotificationDone)
		return err
	}
	if err := c.applyGeneratedFiles(project.ID, generated.Files); err != nil {
		_ = c.appendAssistantMessage(project.ID, conversationID, likeableNotificationOpen+"Build agent patch could not be applied: "+err.Error()+likeableNotificationDone)
		return err
	}
	summary := strings.TrimSpace(generated.Summary)
	if summary == "" {
		summary = "Updated the app."
	}
	body := likeableNotificationOpen + summary + likeableNotificationDone
	if err := c.appendAssistantMessage(project.ID, conversationID, body); err != nil {
		return err
	}
	return c.appendActivity(project.ID, map[string]any{
		"id":          "local-done-" + time.Now().UTC().Format(time.RFC3339Nano),
		"type":        "status",
		"message":     body,
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (c *Client) runPromptImprove(ctx context.Context, conversationID string, project *Project, text string) error {
	response, err := c.openAIText(ctx, "Rewrite the prompt. Return only the improved prompt wrapped in the exact markers requested by the user.", text)
	if err != nil {
		return err
	}
	if !strings.Contains(response, "LIKEABLE_PROMPT_IMPROVE_START") {
		response = extractPlainText(response)
		response = "[[LIKEABLE_PROMPT_IMPROVE_START]]\n" + strings.TrimSpace(response) + "\n[[LIKEABLE_PROMPT_IMPROVE_END]]"
	}
	return c.appendAssistantMessage(project.ID, conversationID, response)
}

func (c *Client) openAIText(ctx context.Context, instructions, input string) (string, error) {
	if len(input) > maxWorkspacePromptBytes {
		input = input[:maxWorkspacePromptBytes]
	}
	body, err := json.Marshal(map[string]any{
		"model":             c.openAIModel,
		"instructions":      instructions,
		"input":             input,
		"max_output_tokens": 12000,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIResponsesEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.openAIAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &PlatformError{Code: "OPENAI_REQUEST_FAILED", Status: resp.StatusCode, Message: strings.TrimSpace(string(data))}
	}
	text := responseOutputText(data)
	if strings.TrimSpace(text) == "" {
		return "", errors.New("OpenAI response did not contain text")
	}
	return text, nil
}

func responseOutputText(data []byte) string {
	var decoded struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Text string `json:"text"`
				Type string `json:"type"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return ""
	}
	if strings.TrimSpace(decoded.OutputText) != "" {
		return decoded.OutputText
	}
	var builder strings.Builder
	for _, item := range decoded.Output {
		for _, content := range item.Content {
			if strings.TrimSpace(content.Text) != "" {
				if builder.Len() > 0 {
					builder.WriteString("\n")
				}
				builder.WriteString(content.Text)
			}
		}
	}
	return builder.String()
}

func appBuilderInstructions() string {
	return `You are a coding agent editing a small static web app.

Return only JSON. Do not wrap it in markdown.

Schema:
{
  "summary": "short user-facing update",
  "files": [
    {"path": "index.html", "content": "..."},
    {"path": "styles.css", "content": "..."},
    {"path": "app.js", "content": "..."}
  ]
}

Rules:
- Build a complete, polished, responsive static app using HTML, CSS, and vanilla JavaScript.
- Keep paths relative. Do not write outside the project.
- Do not include secrets or API keys.
- Prefer editing existing files over inventing many files.
- The preview is served from the project root, so use relative asset paths.`
}

func appBuilderInput(project *Project, userPrompt string, current map[string]string, attachmentPaths []string) string {
	var builder strings.Builder
	builder.WriteString("Project title: ")
	if project != nil {
		builder.WriteString(project.Title)
	}
	builder.WriteString("\n\nUser request:\n")
	builder.WriteString(userPrompt)
	if len(attachmentPaths) > 0 {
		builder.WriteString("\n\nAttachments are available to the host but not embedded here. Filenames:\n")
		for _, path := range attachmentPaths {
			builder.WriteString("- ")
			builder.WriteString(filepath.Base(path))
			builder.WriteByte('\n')
		}
	}
	builder.WriteString("\n\nCurrent files:\n")
	for path, content := range current {
		builder.WriteString("\n--- ")
		builder.WriteString(path)
		builder.WriteString(" ---\n")
		builder.WriteString(content)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func (c *Client) workspaceSnapshot(projectID string) (map[string]string, error) {
	dir := c.workspaceDir(projectID)
	out := map[string]string{}
	for _, name := range []string{"index.html", "styles.css", "app.js"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(data) > maxWorkspaceFileBytes {
			data = data[:maxWorkspaceFileBytes]
		}
		out[name] = string(data)
	}
	return out, nil
}

func parseGeneratedAppResponse(raw string) (generatedAppResponse, error) {
	raw = strings.TrimSpace(stripMarkdownFence(raw))
	var out generatedAppResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start < 0 || end <= start {
			return out, err
		}
		if retryErr := json.Unmarshal([]byte(raw[start:end+1]), &out); retryErr != nil {
			return out, err
		}
	}
	if len(out.Files) == 0 {
		return out, errors.New("no files returned")
	}
	return out, nil
}

func stripMarkdownFence(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "```") {
		return raw
	}
	lines := strings.Split(raw, "\n")
	if len(lines) >= 3 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		return strings.Join(lines[1:len(lines)-1], "\n")
	}
	return raw
}

func (c *Client) applyGeneratedFiles(projectID string, files []generatedFile) error {
	root := c.workspaceDir(projectID)
	for _, file := range files {
		rel, err := cleanGeneratedPath(file.Path)
		if err != nil {
			return err
		}
		if len(file.Content) > maxGeneratedFileBytes {
			return fmt.Errorf("%s is too large", rel)
		}
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(file.Content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func cleanGeneratedPath(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" {
		return "", errors.New("generated file path is empty")
	}
	if strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("absolute generated path %q is not allowed", raw)
	}
	clean := filepath.Clean(raw)
	if clean == "." || strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+".."+string(filepath.Separator)) {
		return "", fmt.Errorf("generated path %q escapes the workspace", raw)
	}
	if clean == metadataDirName || strings.HasPrefix(clean, metadataDirName+string(filepath.Separator)) || strings.HasPrefix(clean, ".git"+string(filepath.Separator)) {
		return "", fmt.Errorf("generated path %q is reserved", raw)
	}
	return clean, nil
}

func (c *Client) appendAssistantMessage(projectID, conversationID, body string) error {
	return c.appendJSONL(projectID, workspaceMessagesFile, map[string]any{
		"id":              "assistant-" + time.Now().UTC().Format(time.RFC3339Nano),
		"role":            "assistant",
		"body":            body,
		"created_at":      time.Now().UTC().Format(time.RFC3339Nano),
		"conversation_id": conversationID,
	})
}

func (c *Client) appendActivity(projectID string, value map[string]any) error {
	return c.appendJSONL(projectID, workspaceActivityFile, value)
}

func (c *Client) appendJSONL(projectID, filename string, value map[string]any) error {
	if err := c.ensureMetadataDir(projectID); err != nil {
		return err
	}
	path := filepath.Join(c.workspaceDir(projectID), metadataDirName, filename)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (c *Client) readJSONL(projectID, filename string) ([]any, error) {
	path := filepath.Join(c.workspaceDir(projectID), metadataDirName, filename)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []any{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	out := []any{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err == nil {
			out = append(out, item)
		}
	}
	return out, scanner.Err()
}

func (c *Client) writeLive(projectID string, live *ConversationLiveState) error {
	if err := c.ensureMetadataDir(projectID); err != nil {
		return err
	}
	data, err := json.Marshal(live)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.workspaceDir(projectID), metadataDirName, workspaceLiveFile), data, 0o600)
}

func extractPlainText(value string) string {
	value = stripMarkdownFence(value)
	return strings.TrimSpace(value)
}

func defaultIndexHTML(title string) string {
	escapedTitle := html.EscapeString(firstNonEmpty(title, "New app"))
	return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>` + escapedTitle + `</title>
    <link rel="stylesheet" href="./styles.css" />
  </head>
  <body>
    <main class="shell">
      <section class="hero">
        <p class="eyebrow">Likeable standalone</p>
        <h1>` + escapedTitle + `</h1>
        <p class="lede">Send a prompt in the chat to generate this project. This preview is served from the local droplet workspace.</p>
        <button id="primaryAction" type="button">Preview is ready</button>
      </section>
    </main>
    <script src="./app.js" type="module"></script>
  </body>
</html>
`
}

func defaultStylesCSS() string {
	return `:root {
  color-scheme: light dark;
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  background: #101417;
  color: #f5f7f8;
}

* { box-sizing: border-box; }

body {
  margin: 0;
  min-height: 100vh;
  background:
    linear-gradient(120deg, rgba(92, 190, 167, 0.18), transparent 32%),
    linear-gradient(300deg, rgba(255, 204, 102, 0.18), transparent 36%),
    #101417;
}

.shell {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 32px;
}

.hero {
  width: min(720px, 100%);
}

.eyebrow {
  margin: 0 0 14px;
  color: #9fd9ca;
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

h1 {
  margin: 0;
  font-size: clamp(42px, 8vw, 86px);
  line-height: 0.95;
  letter-spacing: 0;
}

.lede {
  max-width: 560px;
  margin: 22px 0 28px;
  color: #c7d0d4;
  font-size: 18px;
  line-height: 1.55;
}

button {
  border: 0;
  border-radius: 8px;
  padding: 13px 18px;
  background: #f2c766;
  color: #15110a;
  font-weight: 750;
  cursor: pointer;
}
`
}

func defaultAppJS(title string) string {
	escapedTitle := strings.ReplaceAll(firstNonEmpty(title, "this app"), "`", "'")
	return "document.getElementById('primaryAction')?.addEventListener('click', () => {\n  alert(`" + escapedTitle + " is running from a local Likeable workspace.`);\n});\n"
}
