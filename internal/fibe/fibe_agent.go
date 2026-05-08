package fibe

import (
	"context"
	"strings"
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
	var out map[string]any
	if strings.TrimSpace(busyPolicy) == "" {
		busyPolicy = "queue"
	}
	args := []string{"agents", "send-message", c.agentID, "--conversation-id", conversationID, "--busy-policy", busyPolicy}
	for _, path := range attachmentPaths {
		if strings.TrimSpace(path) != "" {
			args = append(args, "--attach", path)
		}
	}
	payload := map[string]any{"text": text}
	return c.runCLI(ctx, append(args, "-f", "-"), payload, &out)
}

func (c *Client) Interrupt(ctx context.Context, conversationID string) error {
	var out map[string]any
	args := []string{"agents", "interrupt", c.agentID}
	if strings.TrimSpace(conversationID) != "" {
		args = append(args, "--conversation-id", conversationID)
	}
	return c.runCLI(ctx, args, nil, &out)
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
