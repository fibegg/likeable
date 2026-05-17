package fibe

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

const maxInlineImageAttachmentBytes = 12 << 20

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
	var out map[string]any
	if strings.TrimSpace(busyPolicy) == "" {
		busyPolicy = "queue"
	}
	args := []string{"agents", "send-message", c.agentID, "--conversation-id", conversationID, "--busy-policy", busyPolicy}
	images := make([]string, 0, len(attachmentPaths))
	for _, path := range attachmentPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		imageDataURL, inline, err := inlineImageAttachmentDataURL(path)
		if err != nil {
			return err
		}
		if inline {
			images = append(images, imageDataURL)
			continue
		}
		args = append(args, "--attach", path)
	}
	payload := map[string]any{"text": text}
	if len(images) > 0 {
		payload["images"] = images
	}
	return c.runCLI(ctx, append(args, "-f", "-"), payload, &out)
}

func inlineImageAttachmentDataURL(path string) (string, bool, error) {
	contentType := inlineImageContentType(path)
	if contentType == "" {
		return "", false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", false, fmt.Errorf("read image attachment: %w", err)
	}
	if info.Size() > maxInlineImageAttachmentBytes {
		return "", false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("read image attachment: %w", err)
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), true, nil
}

func inlineImageContentType(path string) string {
	contentType := strings.ToLower(strings.TrimSpace(mime.TypeByExtension(filepath.Ext(path))))
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	switch contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return contentType
	default:
		return ""
	}
}

func (c *Client) StartAgentChat(ctx context.Context) error {
	if strings.TrimSpace(c.marqueeID) == "" {
		return &PlatformError{
			Code:    "FIBE_MARQUEE_NOT_CONFIGURED",
			Message: "Fibe Marquee is not configured for this project",
		}
	}
	var out map[string]any
	return c.runCLI(ctx, []string{"agents", "start-chat", c.agentID, "--marquee-id", c.marqueeID}, nil, &out)
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
	var out map[string]any
	args := []string{"agents", "interrupt", c.agentID}
	if strings.TrimSpace(conversationID) != "" {
		args = append(args, "--conversation-id", conversationID)
	}
	return c.runCLI(ctx, args, nil, &out)
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
	return c.runCLI(ctx, []string{"playgrounds", action, playgroundID}, nil, nil)
}

func (c *Client) Messages(ctx context.Context, conversationID string) ([]any, error) {
	var out struct {
		Content []any `json:"content"`
	}
	err := c.runCLI(ctx, []string{"agents", "messages", c.agentID, "--conversation-id", conversationID}, nil, &out)
	return out.Content, err
}

func (c *Client) Activity(ctx context.Context, conversationID string) ([]any, error) {
	var out struct {
		Content []any `json:"content"`
	}
	err := c.runCLI(ctx, []string{"agents", "activity", c.agentID, "--conversation-id", conversationID}, nil, &out)
	return out.Content, err
}

func (c *Client) ConversationLiveState(ctx context.Context, conversationID string) (*ConversationLiveState, error) {
	var out ConversationLiveState
	err := c.runCLI(ctx, []string{"agents", "live-state", c.agentID, "--conversation-id", conversationID}, nil, &out)
	if out.ConversationID == "" {
		out.ConversationID = out.ConversationIDAlt
	}
	return &out, err
}
