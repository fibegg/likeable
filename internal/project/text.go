package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fibegg/likeable/internal/domain"
)

func TitleFromPrompt(prompt string) string {
	fields := strings.Fields(prompt)
	if len(fields) > 8 {
		fields = fields[:8]
	}
	title := CleanTitle(strings.Join(fields, " "))
	if title == "" {
		return "Untitled app"
	}
	return title
}

func CleanTitle(title string) string {
	title = strings.Join(strings.Fields(strings.TrimSpace(title)), " ")
	runes := []rune(title)
	if len(runes) > 80 {
		title = string(runes[:80])
	}
	return title
}

func DefaultTitle(existing int) string {
	if existing <= 0 {
		return "New playground"
	}
	return fmt.Sprintf("New playground %d", existing+1)
}

func SourceName(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == '-' || r == '_' || r == '.' || r == ' ' {
			b.WriteRune('-')
		}
	}
	name := strings.Trim(b.String(), "-.")
	if name == "" {
		name = "likeable-app"
	}
	if len(name) > 40 {
		name = name[:40]
	}
	return name + "-" + uuidTail()
}

func PreviewSubdomain(project *domain.Project) string {
	seed := ""
	if project != nil {
		seed = project.ID
		if strings.TrimSpace(seed) == "" {
			seed = project.ConversationID
		}
	}
	if strings.TrimSpace(seed) == "" {
		seed = strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	suffix := dnsSafeHexSuffix(seed)
	return "lk-" + suffix
}

func AgentPrompt(project *domain.Project, userText string) string {
	return fmt.Sprintf(`[[LIKEABLE_SYSTEM_CONTEXT_START]]
Likeable project context:
- title: %s
- Likeable project_id: %s
- Fibe conversation_id: %s
- target Fibe playground_id: %s
- target private source repo: %s
- target preview_url: %s
- target app subdomain: %s

For app/environment changes, target only this project playground. If the user asks for environment shape changes, resolve the current playspec/source from playground_id %s, then use fibe_templates_develop with target_type="playground", mode="apply", post_apply="rollout_target", and wait=true. Target playground_id %s only. Do not use rollout_all, do not update default/global Import Templates, and do not mutate other playgrounds unless an admin/global template workflow explicitly asks for it.

For normal source edits, prefer direct Brownfield changes on the live playground workspace for playground_id %s. Use Fibe MCP/local playground tools to resolve the mounted source paths, edit only that mounted project, and let the current playground reload. Do not create, relink, fork, or replace the project source unless the trusted Likeable project context explicitly requires an environment-shape workflow.
[[LIKEABLE_SYSTEM_CONTEXT_END]]
[LIKEABLE_USER_CONTEXT_START]]
User request:
%s
[LIKEABLE_USER_CONTEXT_END]]`,
		project.Title,
		project.ID,
		project.ConversationID,
		project.PlaygroundID,
		project.RepoURL,
		project.PreviewURL,
		PreviewSubdomain(project),
		project.PlaygroundID,
		project.PlaygroundID,
		project.PlaygroundID,
		userText,
	)
}

func dnsSafeHexSuffix(seed string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(seed) {
		if (r >= 'a' && r <= 'f') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			if b.Len() == 16 {
				return b.String()
			}
		}
	}
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])[:16]
}

func uuidTail() string {
	raw := strings.ReplaceAll(strconv.FormatInt(time.Now().UnixNano(), 36), "-", "")
	if len(raw) > 8 {
		return raw[len(raw)-8:]
	}
	return raw
}
