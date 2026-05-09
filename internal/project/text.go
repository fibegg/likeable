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
	return SourceNamePrefix(title) + "-" + uuidTail()
}

func SourceNamePrefix(title string) string {
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
	return name
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

func ServiceSubdomains(project *domain.Project) map[string]string {
	app := PreviewSubdomain(project)
	return map[string]string{
		"app":   app,
		"admin": app + "-admin",
	}
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
- selected service: %s
- project services:
%s
- project repositories:
%s

For app/environment changes, target only this project playground. If the user asks for environment shape changes, resolve the current playspec/source from playground_id %s, then use fibe_templates_develop with target_type="playground", mode="apply", post_apply="rollout_target", and wait=true. Target playground_id %s only. Do not use rollout_all, do not update default/global Import Templates, and do not mutate other playgrounds unless an admin/global template workflow explicitly asks for it.

If the user asks to replace the stack or move away from the current template family, use fibe_playgrounds_retemplate or the equivalent generic Fibe playground retemplate workflow for playground_id %s. Keep the same playground, provision missing private source Props through the platform when needed, expose every user-facing service with clear names, and do not hardcode Likeable-specific behavior into the application.

For normal source edits, prefer direct Brownfield changes on the live playground workspace for playground_id %s. Use Fibe MCP/local playground tools to resolve the mounted source paths, edit only that mounted project, and let the current playground reload. Do not create, relink, fork, or replace the project source unless the trusted Likeable project context explicitly requires an environment-shape workflow.
[[LIKEABLE_SYSTEM_CONTEXT_END]]
[[LIKEABLE_USER_CONTEXT_START]]
User request:
%s
[[LIKEABLE_USER_CONTEXT_END]]`,
		project.Title,
		project.ID,
		project.ConversationID,
		project.PlaygroundID,
		project.RepoURL,
		project.PreviewURL,
		PreviewSubdomain(project),
		selectedServiceLine(project),
		formatProjectServices(project),
		formatProjectRepositories(project),
		project.PlaygroundID,
		project.PlaygroundID,
		project.PlaygroundID,
		project.PlaygroundID,
		userText,
	)
}

func selectedServiceLine(project *domain.Project) string {
	if project == nil {
		return "none"
	}
	selected := strings.TrimSpace(project.SelectedService)
	for _, service := range project.Services {
		if selected == "" || strings.EqualFold(service.Name, selected) {
			return strings.TrimSpace(service.Name + " " + service.URL)
		}
	}
	if selected != "" {
		return selected
	}
	return "none"
}

func formatProjectServices(project *domain.Project) string {
	if project == nil || len(project.Services) == 0 {
		return "  - none"
	}
	var b strings.Builder
	for _, service := range project.Services {
		name := strings.TrimSpace(service.Name)
		if name == "" {
			name = "service"
		}
		fmt.Fprintf(&b, "  - %s: %s\n", name, service.URL)
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatProjectRepositories(project *domain.Project) string {
	if project == nil {
		return "  - none"
	}
	if len(project.Repositories) == 0 {
		if strings.TrimSpace(project.RepoURL) == "" {
			return "  - none"
		}
		return "  - source: " + project.RepoURL
	}
	var b strings.Builder
	for _, repository := range project.Repositories {
		role := strings.TrimSpace(repository.Role)
		if role == "" {
			role = "source"
		}
		names := strings.Join(repository.ServiceNames, ",")
		if names != "" {
			names = " [" + names + "]"
		}
		fmt.Fprintf(&b, "  - %s%s: %s\n", role, names, repository.RepoURL)
	}
	return strings.TrimRight(b.String(), "\n")
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
