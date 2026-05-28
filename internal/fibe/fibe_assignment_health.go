package fibe

import (
	"context"
	"strings"
)

type AssignmentHealth struct {
	AgentID                     string   `json:"agentId"`
	MarqueeID                   string   `json:"serverId"`
	AgentStatus                 string   `json:"agentStatus,omitempty"`
	AgentAuthenticated          bool     `json:"agentAuthenticated"`
	MarqueeStatus               string   `json:"serverStatus,omitempty"`
	MarqueeBillingRuntimeActive bool     `json:"serverBillingRuntimeActive"`
	MarqueeChatLaunchable       bool     `json:"serverChatLaunchable"`
	OK                          bool     `json:"ok"`
	Problems                    []string `json:"problems,omitempty"`
}

func (c *Client) AssignmentHealth(ctx context.Context) AssignmentHealth {
	health := AssignmentHealth{
		AgentID:   c.agentID,
		MarqueeID: c.marqueeID,
	}
	if strings.TrimSpace(c.agentID) == "" {
		health.Problems = append(health.Problems, "agent ID is not configured")
	}
	if strings.TrimSpace(c.marqueeID) == "" {
		health.Problems = append(health.Problems, "server ID is not configured")
	}
	if strings.TrimSpace(c.agentID) != "" {
		agent, err := c.sdk.Agents.GetByIdentifier(ctx, c.agentID)
		if err != nil {
			health.Problems = append(health.Problems, "agent: "+wrapSDKError(err).Error())
		} else {
			health.AgentStatus = strings.TrimSpace(agent.Status)
			health.AgentAuthenticated = agent.Authenticated
			if !agent.Authenticated {
				health.Problems = append(health.Problems, "agent is not authenticated")
			}
		}
	}
	if strings.TrimSpace(c.marqueeID) != "" {
		marquee, err := c.sdk.Marquees.GetByIdentifier(ctx, c.marqueeID)
		if err != nil {
			health.Problems = append(health.Problems, "server: "+wrapSDKError(err).Error())
		} else {
			health.MarqueeStatus = strings.TrimSpace(marquee.Status)
			health.MarqueeBillingRuntimeActive = marquee.BillingRuntimeActive
			health.MarqueeChatLaunchable = marquee.ChatLaunchable
			if !strings.EqualFold(health.MarqueeStatus, "active") {
				health.Problems = append(health.Problems, "server is not active")
			}
			if !marquee.BillingRuntimeActive {
				health.Problems = append(health.Problems, "server runtime is not funded")
			}
			if !marquee.ChatLaunchable {
				health.Problems = append(health.Problems, "server chat runtime is not launchable")
			}
		}
	}
	health.OK = len(health.Problems) == 0
	return health
}
