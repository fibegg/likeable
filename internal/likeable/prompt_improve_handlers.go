package likeable

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	fibegateway "github.com/fibegg/likeable/internal/fibe"
	"github.com/google/uuid"
)

const (
	promptImproveStart = "[[LIKEABLE_PROMPT_IMPROVE_START]]"
	promptImproveEnd   = "[[LIKEABLE_PROMPT_IMPROVE_END]]"
)

func (s *Server) handleProjectPromptImprove(w http.ResponseWriter, r *http.Request, user *User, project *Project) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	improved, err := s.improvePromptWithAgent(r.Context(), user, project, body.Text)
	if err != nil {
		log.Printf("agent prompt improve failed for project %s: %v", project.ID, err)
		writeJSON(w, http.StatusOK, map[string]any{
			"text":    fallbackImprovedPrompt(body.Text, project.Title),
			"source":  "fallback",
			"warning": "prompt improve agent is unavailable",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": improved, "source": "agent"})
}

func (s *Server) improvePromptWithAgent(ctx context.Context, user *User, project *Project, draft string) (string, error) {
	if user == nil || project == nil {
		return "", fmt.Errorf("project context is required")
	}
	client, err := s.fibeClientForProject(ctx, project, user.Email)
	if err != nil {
		return "", err
	}
	conversationID := "likeable-prompt-improve-" + uuid.NewString()
	ctx, cancel := context.WithTimeout(ctx, 24*time.Second)
	defer cancel()
	if err := client.EnsureConversation(ctx, conversationID, "Likeable prompt improve"); err != nil {
		return "", err
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if err := client.DeleteConversation(cleanupCtx, conversationID); err != nil {
			log.Printf("delete prompt improve conversation %s: %v", conversationID, err)
		}
	}()
	if err := client.SendMessage(ctx, conversationID, promptImproveRequest(project, draft), nil, "reject"); err != nil {
		if fibegateway.IsAgentRuntimeUnavailableError(err) {
			if startErr := s.startProjectAgentChat(ctx, project, client, "prompt improve"); startErr != nil {
				return "", startErr
			}
			if retryErr := client.SendMessage(ctx, conversationID, promptImproveRequest(project, draft), nil, "reject"); retryErr == nil {
				goto poll
			} else {
				return "", retryErr
			}
		}
		return "", err
	}
poll:
	ticker := time.NewTicker(900 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			messages, err := client.Messages(ctx, conversationID)
			if err != nil {
				return "", err
			}
			if improved := extractPromptImprovement(messages); improved != "" {
				return improved, nil
			}
		}
	}
}

func promptImproveRequest(project *Project, draft string) string {
	title := ""
	service := ""
	if project != nil {
		title = strings.TrimSpace(project.Title)
		service = strings.TrimSpace(project.SelectedService)
	}
	draft = strings.TrimSpace(draft)
	if draft == "" {
		draft = "Improve the current app."
	}
	return fmt.Sprintf(`You are Likeable's prompt-improvement agent.

Task:
- Rewrite the user's draft into one stronger prompt for a coding/build agent.
- Do not edit files, do not run tools, do not build, and do not ask follow-up questions.
- Keep the solution universal: do not add domain-specific details unless they are present in the draft or current app context.
- Preserve the current app/domain unless the draft explicitly asks to replace it.
- Keep the user's language when it is clear; otherwise use English.
- Make the prompt specific about outcome, UX, responsive behavior, states, and verification.
- Return only the improved prompt wrapped exactly between:
%s
%s

Current app title: %s
Selected service: %s
User draft:
%s`, promptImproveStart, promptImproveEnd, title, service, draft)
}

func extractPromptImprovement(messages []any) string {
	for i := len(messages) - 1; i >= 0; i-- {
		message, ok := messages[i].(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(fmt.Sprint(message["role"])))
		if role != "assistant" {
			continue
		}
		body := strings.TrimSpace(fmt.Sprint(message["body"]))
		if body == "" || body == "<nil>" {
			continue
		}
		if extracted := betweenMarkers(body, promptImproveStart, promptImproveEnd); extracted != "" {
			return extracted
		}
	}
	return ""
}

func betweenMarkers(value, start, end string) string {
	startIndex := strings.Index(value, start)
	if startIndex < 0 {
		return ""
	}
	contentStart := startIndex + len(start)
	endIndex := strings.Index(value[contentStart:], end)
	if endIndex < 0 {
		return ""
	}
	return strings.TrimSpace(value[contentStart : contentStart+endIndex])
}

func fallbackImprovedPrompt(draft, projectTitle string) string {
	draft = strings.TrimSpace(strings.Join(strings.Fields(draft), " "))
	context := "current app"
	if title := strings.TrimSpace(projectTitle); title != "" {
		context = fmt.Sprintf("current %q app", title)
	}
	if draft == "" {
		return fmt.Sprintf("Improve the %s. Keep the existing product/domain intact, tighten the main user flow, polish the responsive UI, and fix any obvious visual or interaction issues. Do not replace it with an unrelated app.", context)
	}
	return strings.Join([]string{
		fmt.Sprintf("Improve the %s.", context),
		"Requested change: " + draft + ".",
		"Stay in the same product/domain and build on the existing app; do not replace it with an unrelated app.",
		"Make the result production-polished: clear layout hierarchy, responsive states, useful empty/loading/error states, and no overlapping text or controls.",
		"Preserve existing working functionality unless the requested change explicitly says otherwise, then run the app/build and fix visible issues before finishing.",
	}, "\n")
}
