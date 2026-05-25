package fibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	sdkfibe "github.com/fibegg/sdk/fibe"
)

type ConversationLiveState struct {
	ConversationID    string `json:"conversationId,omitempty"`
	ConversationIDAlt string `json:"conversation_id,omitempty"`
	IsProcessing      bool   `json:"isProcessing"`
	StreamText        string `json:"streamText"`
	CurrentActivityID string `json:"currentActivityId,omitempty"`
	QueuedTurns       int    `json:"queuedTurns,omitempty"`
	StartedAt         string `json:"startedAt,omitempty"`
}

func (c *Client) SendMessage(ctx context.Context, conversationID, text string, attachmentPaths []string, busyPolicy string) error {
	if strings.TrimSpace(busyPolicy) == "" {
		busyPolicy = "queue"
	}
	preparedAttachments, err := prepareMessageAttachmentPaths(attachmentPaths)
	if err != nil {
		return err
	}
	defer cleanupPreparedAttachments(preparedAttachments)

	attachmentFilenames := make([]string, 0, len(preparedAttachments))
	for _, attachment := range preparedAttachments {
		upload, err := c.sdk.Agents.UploadByIdentifier(ctx, c.agentID, &sdkfibe.AgentUploadParams{
			FilePath:       attachment.path,
			ConversationID: conversationID,
		})
		if err != nil {
			return wrapSDKError(err)
		}
		if upload == nil || strings.TrimSpace(upload.Filename) == "" {
			return &PlatformError{
				Code:    "VALIDATION_FAILED",
				Status:  422,
				Message: "attachment upload did not return a filename",
			}
		}
		attachmentFilenames = append(attachmentFilenames, upload.Filename)
	}
	_, err = c.sdk.Agents.ChatByIdentifier(ctx, c.agentID, &sdkfibe.AgentChatParams{
		Text:                text,
		ConversationID:      conversationID,
		BusyPolicy:          busyPolicy,
		AttachmentFilenames: attachmentFilenames,
	})
	return wrapSDKError(err)
}

func (c *Client) StartAgentChat(ctx context.Context) error {
	if strings.TrimSpace(c.marqueeID) == "" {
		return &PlatformError{
			Code:    "FIBE_MARQUEE_NOT_CONFIGURED",
			Message: "Fibe Marquee is not configured for this project",
		}
	}
	_, err := c.sdk.Agents.StartChatByAgentIdentifier(ctx, c.agentID, c.marqueeID)
	return wrapSDKError(err)
}

func IsAgentRuntimeUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	var platformErr *PlatformError
	if errors.As(err, &platformErr) {
		text = strings.ToLower(strings.Join([]string{
			platformErr.Code,
			platformErr.Message,
			platformErr.Stderr,
			err.Error(),
		}, " "))
	}
	return containsAny(text,
		"no running agentchat",
		"no running agent chat",
		"no running chat",
		"start a chat first",
		"agent unreachable",
		"connection refused",
		"runtime reachable: no",
	)
}

func IsConversationMissingError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	var platformErr *PlatformError
	if errors.As(err, &platformErr) {
		text = strings.ToLower(strings.Join([]string{
			platformErr.Code,
			platformErr.Message,
			platformErr.Stderr,
			err.Error(),
		}, " "))
	}
	return strings.Contains(text, "conversation") && containsAny(text, "not found", "http 404")
}

func (c *Client) Interrupt(ctx context.Context, conversationID string) error {
	_, err := c.sdk.Agents.InterruptByIdentifier(ctx, c.agentID, &sdkfibe.AgentConversationParams{ConversationID: strings.TrimSpace(conversationID)})
	return wrapSDKError(err)
}

func (c *Client) StartPlayground(ctx context.Context, playgroundID string) error {
	return c.controlPlayground(ctx, "start", playgroundID)
}

func (c *Client) StopPlayground(ctx context.Context, playgroundID string) error {
	return c.controlPlayground(ctx, "stop", playgroundID)
}

func (c *Client) RestartPlayground(ctx context.Context, playgroundID string) error {
	return c.controlPlayground(ctx, "hard-restart", playgroundID)
}

func (c *Client) controlPlayground(ctx context.Context, action, playgroundID string) error {
	playgroundID = strings.TrimSpace(playgroundID)
	if playgroundID == "" {
		return errors.New("playground ID is required")
	}
	actionType := strings.ReplaceAll(strings.TrimSpace(action), "-", "_")
	if actionType == "restart" {
		actionType = sdkfibe.PlaygroundActionHardRestart
	}
	_, err := c.sdk.Playgrounds.ActionByIdentifier(ctx, playgroundID, &sdkfibe.PlaygroundActionParams{ActionType: actionType})
	return wrapSDKError(err)
}

func (c *Client) Messages(ctx context.Context, conversationID string) ([]any, error) {
	out, err := c.sdk.Agents.GetMessagesByIdentifierWithParams(ctx, c.agentID, &sdkfibe.AgentDataParams{ConversationID: conversationID})
	return c.recordsWithRuntimeFallback(ctx, conversationID, "messages", agentDataRecords(out), wrapSDKError(err))
}

func (c *Client) Activity(ctx context.Context, conversationID string) ([]any, error) {
	out, err := c.sdk.Agents.GetActivityByIdentifierWithParams(ctx, c.agentID, &sdkfibe.AgentDataParams{ConversationID: conversationID})
	return c.recordsWithRuntimeFallback(ctx, conversationID, "activities", agentDataRecords(out), wrapSDKError(err))
}

func agentDataRecords(out *sdkfibe.AgentData) []any {
	if out == nil || out.Content == nil {
		return nil
	}
	records, ok := out.Content.([]any)
	if ok {
		return records
	}
	return []any{out.Content}
}

func (c *Client) recordsWithRuntimeFallback(ctx context.Context, conversationID, resource string, records []any, cliErr error) ([]any, error) {
	if cliErr == nil && len(records) > 0 {
		return records, nil
	}
	runtimeRecords, runtimeErr := c.runtimeConversationRecords(ctx, conversationID, resource)
	if runtimeErr == nil && (len(runtimeRecords) > 0 || cliErr != nil) {
		return runtimeRecords, nil
	}
	return records, cliErr
}

func (c *Client) runtimeConversationRecords(ctx context.Context, conversationID, resource string) ([]any, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, errors.New("conversation ID is required")
	}
	chatURL, err := c.resolveRuntimeChatURL(ctx)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(chatURL, "/") + "/api/conversations/" + url.PathEscape(conversationID) + "/" + resource
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("runtime %s returned HTTP %d: %s", resource, res.StatusCode, strings.TrimSpace(string(body)))
	}
	var out []any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) resolveRuntimeChatURL(ctx context.Context) (string, error) {
	if strings.TrimSpace(c.runtimeChatURL) != "" {
		return c.runtimeChatURL, nil
	}
	out, err := c.sdk.Agents.RuntimeStatusByIdentifier(ctx, c.agentID)
	if err != nil {
		return "", wrapSDKError(err)
	}
	chatURL := ""
	if out != nil && out.ChatURL != nil {
		chatURL = strings.TrimSpace(*out.ChatURL)
	}
	if chatURL == "" {
		return "", errors.New("runtime chat URL is missing")
	}
	c.runtimeChatURL = chatURL
	return chatURL, nil
}

func (c *Client) ConversationLiveState(ctx context.Context, conversationID string) (*ConversationLiveState, error) {
	out, err := c.sdk.Agents.LiveStateByIdentifier(ctx, c.agentID, &sdkfibe.AgentDataParams{ConversationID: conversationID})
	if out == nil {
		return &ConversationLiveState{}, wrapSDKError(err)
	}
	result := &ConversationLiveState{
		ConversationID:    out.ConversationID,
		ConversationIDAlt: out.ConversationIDAlt,
		IsProcessing:      out.IsProcessing,
		StreamText:        out.StreamText,
		CurrentActivityID: out.CurrentActivityID,
		QueuedTurns:       out.QueuedTurns,
		StartedAt:         out.StartedAt,
	}
	if result.ConversationID == "" {
		result.ConversationID = result.ConversationIDAlt
	}
	return result, wrapSDKError(err)
}
