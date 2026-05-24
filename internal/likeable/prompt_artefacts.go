package likeable

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	projecttext "github.com/fibegg/likeable/internal/project"
)

const maxPromptArtefactContentBytes = 24 << 10

var promptArtefactMacro = regexp.MustCompile(`\{\|artefact:([A-Za-z0-9_.-]+)\|\}`)

type promptArtefactConfigRow struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Value   string `json:"value"`
}

func (s *Server) resolvePromptArtefactMacros(ctx context.Context, text string) (string, []projecttext.PromptArtefact, error) {
	matches := promptArtefactMacro.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return text, nil, nil
	}
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return "", nil, err
	}
	configured := parsePromptArtefactConfig(cfg["agent_artefacts"])
	seen := map[string]bool{}
	artefacts := make([]projecttext.PromptArtefact, 0, len(matches))
	for _, match := range matches {
		name := strings.TrimSpace(match[1])
		if seen[name] {
			continue
		}
		content := strings.TrimSpace(configured[name])
		if content == "" {
			return "", nil, fmt.Errorf("prompt artefact %q is not configured", name)
		}
		if len(content) > maxPromptArtefactContentBytes {
			return "", nil, fmt.Errorf("prompt artefact %q is too large", name)
		}
		seen[name] = true
		artefacts = append(artefacts, projecttext.PromptArtefact{Name: name, Content: content})
	}
	resolvedText := promptArtefactMacro.ReplaceAllString(text, `[artefact:$1]`)
	return resolvedText, artefacts, nil
}

func parsePromptArtefactConfig(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}
	}
	var object map[string]string
	if err := json.Unmarshal([]byte(raw), &object); err == nil {
		return normalizePromptArtefactConfig(object)
	}
	var rows []promptArtefactConfigRow
	if err := json.Unmarshal([]byte(raw), &rows); err == nil {
		out := make(map[string]string, len(rows))
		for _, row := range rows {
			name := strings.TrimSpace(row.Name)
			content := firstNonEmptyString(row.Content, row.Value)
			if name != "" && content != "" {
				out[name] = content
			}
		}
		return out
	}
	return map[string]string{}
}

func normalizePromptArtefactConfig(input map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for name, content := range input {
		name = strings.TrimSpace(name)
		content = strings.TrimSpace(content)
		if name != "" && content != "" {
			out[name] = content
		}
	}
	return out
}
