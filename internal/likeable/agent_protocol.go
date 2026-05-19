package likeable

import (
	"strings"

	fibegateway "github.com/fibegg/likeable/internal/fibe"
)

const (
	likeableNotificationStart = "[[LIKEABLE_NOTIFICATION_START]]"
	likeableNotificationEnd   = "[[LIKEABLE_NOTIFICATION_END]]"
)

func sanitizeAgentProtocolMessages(messages []any) []any {
	if len(messages) == 0 {
		return messages
	}
	out := make([]any, len(messages))
	for i, item := range messages {
		message, ok := item.(map[string]any)
		if !ok {
			out[i] = item
			continue
		}
		role, _ := message["role"].(string)
		body, _ := message["body"].(string)
		if role != "assistant" || body == "" {
			out[i] = item
			continue
		}
		copy := make(map[string]any, len(message))
		for key, value := range message {
			copy[key] = value
		}
		copy["body"] = notificationProtocolOnly(body)
		out[i] = copy
	}
	return out
}

func sanitizeAgentProtocolLiveState(live *fibegateway.ConversationLiveState) {
	if live == nil || live.StreamText == "" {
		return
	}
	if filtered := notificationProtocolOnly(live.StreamText); filtered != "" {
		live.StreamText = filtered
		return
	}
	if message := agentLiveStateErrorNotification(live.StreamText); message != "" {
		live.StreamText = likeableNotificationStart + message + likeableNotificationEnd
		return
	}
	live.StreamText = ""
}

func notificationProtocolOnly(value string) string {
	var builder strings.Builder
	cursor := 0
	for cursor < len(value) {
		relativeStart := strings.Index(value[cursor:], likeableNotificationStart)
		if relativeStart == -1 {
			break
		}
		start := cursor + relativeStart
		contentStart := start + len(likeableNotificationStart)
		relativeEnd := strings.Index(value[contentStart:], likeableNotificationEnd)
		if relativeEnd == -1 {
			builder.WriteString(value[start:])
			break
		}
		end := contentStart + relativeEnd + len(likeableNotificationEnd)
		builder.WriteString(value[start:end])
		cursor = end
	}
	return builder.String()
}

func agentLiveStateErrorNotification(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}
	switch {
	case strings.Contains(normalized, "invalid api key"),
		strings.Contains(normalized, "external api key"),
		strings.Contains(normalized, "authentication_error"),
		strings.Contains(normalized, "provider key"):
		return "Build agent authentication failed. Check the Fibe agent provider key, then try again."
	case strings.Contains(normalized, "rate limit"):
		return "The build agent is rate limited. Try again shortly."
	default:
		return ""
	}
}
