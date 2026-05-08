package fibe

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
)

type fibeAssignmentInput struct {
	Label          string `json:"label"`
	AgentID        string `json:"agent_id"`
	AgentIDAlias   string `json:"agentId"`
	ServerID       string `json:"server_id"`
	ServerIDAlias  string `json:"serverId"`
	MarqueeID      string `json:"marquee_id"`
	MarqueeIDAlias string `json:"marqueeId"`
}

func GlobalAssignment(cfg map[string]string) Assignment {
	return Assignment{
		AgentID:   strings.TrimSpace(cfg["fibe_agent_id"]),
		MarqueeID: strings.TrimSpace(cfg["fibe_marquee_id"]),
	}
}

func AssignmentForNewProject(cfg map[string]string, email string) (Assignment, error) {
	pool, err := assignmentPoolFromConfig(cfg)
	if err != nil {
		return Assignment{}, err
	}
	if assignment, ok := selectAssignment(email, pool); ok {
		return assignment, nil
	}
	return GlobalAssignment(cfg), nil
}

func AssignmentForProject(cfg map[string]string, project *Project, email string) (Assignment, error) {
	if project != nil && (strings.TrimSpace(project.AgentID) != "" || strings.TrimSpace(project.MarqueeID) != "") {
		global := GlobalAssignment(cfg)
		return Assignment{
			AgentID:   firstNonEmpty(project.AgentID, global.AgentID),
			MarqueeID: firstNonEmpty(project.MarqueeID, global.MarqueeID),
		}, nil
	}
	global := GlobalAssignment(cfg)
	if global.AgentID != "" {
		return global, nil
	}
	pool, err := assignmentPoolFromConfig(cfg)
	if err != nil {
		return Assignment{}, err
	}
	if assignment, ok := selectAssignment(email, pool); ok {
		return assignment, nil
	}
	return global, nil
}

func assignmentPoolFromConfig(cfg map[string]string) ([]Assignment, error) {
	return ParseAssignmentPool(firstNonEmpty(cfg["fibe_agent_server_pool"], cfg["fibe_agent_marquee_pool"]))
}

func ParseAssignmentPool(raw string) ([]Assignment, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var inputs []fibeAssignmentInput
	if err := json.Unmarshal([]byte(raw), &inputs); err != nil {
		return nil, fmt.Errorf("agent/server pool must be a JSON array: %w", err)
	}
	out := make([]Assignment, 0, len(inputs))
	for i, input := range inputs {
		assignment := Assignment{
			Label:     strings.TrimSpace(input.Label),
			AgentID:   firstNonEmpty(input.AgentID, input.AgentIDAlias),
			MarqueeID: firstNonEmpty(input.ServerID, input.ServerIDAlias, input.MarqueeID, input.MarqueeIDAlias),
		}
		if assignment.Label == "" && assignment.AgentID == "" && assignment.MarqueeID == "" {
			continue
		}
		if assignment.AgentID == "" || assignment.MarqueeID == "" {
			return nil, fmt.Errorf("agent/server pool row %d requires both agent_id and server_id", i+1)
		}
		out = append(out, assignment)
	}
	return out, nil
}

func EncodeAssignmentPool(pool []Assignment) string {
	if len(pool) == 0 {
		return "[]"
	}
	data, err := json.Marshal(pool)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func selectAssignment(email string, pool []Assignment) (Assignment, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(pool) == 0 {
		return Assignment{}, false
	}
	var selected Assignment
	var selectedKey string
	var selectedScore uint64
	for _, assignment := range pool {
		if assignment.AgentID == "" || assignment.MarqueeID == "" {
			continue
		}
		key := assignment.AgentID + "\x00" + assignment.MarqueeID
		score := assignmentScore(email, key)
		if selected.AgentID == "" || score > selectedScore || (score == selectedScore && key < selectedKey) {
			selected = assignment
			selectedKey = key
			selectedScore = score
		}
	}
	if selected.AgentID == "" {
		return Assignment{}, false
	}
	return selected, true
}

func assignmentScore(email, key string) uint64 {
	sum := sha256.Sum256([]byte(email + "\x00" + key))
	return binary.BigEndian.Uint64(sum[:8])
}
