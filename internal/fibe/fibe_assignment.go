package fibe

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type fibeAssignmentInput struct {
	Label          string          `json:"label"`
	AgentID        string          `json:"agent_id"`
	AgentIDAlias   string          `json:"agentId"`
	ServerID       string          `json:"server_id"`
	ServerIDAlias  string          `json:"serverId"`
	MarqueeID      string          `json:"marquee_id"`
	MarqueeIDAlias string          `json:"marqueeId"`
	Status         string          `json:"status"`
	StatusAlias    string          `json:"state"`
	Capacity       json.RawMessage `json:"capacity"`
	MaxProjects    json.RawMessage `json:"max_projects"`
	MaxProjectsAlt json.RawMessage `json:"maxProjects"`
}

const (
	AssignmentStatusActive   = "active"
	AssignmentStatusDraining = "draining"
	AssignmentStatusRetiring = "retiring"
	AssignmentStatusRetired  = "retired"
)

func GlobalAssignment(cfg map[string]string) Assignment {
	return Assignment{
		AgentID:   strings.TrimSpace(cfg["fibe_agent_id"]),
		MarqueeID: strings.TrimSpace(cfg["fibe_marquee_id"]),
		Status:    AssignmentStatusActive,
	}
}

func AssignmentForNewProject(cfg map[string]string, projectID string) (Assignment, error) {
	pool, err := assignmentPoolFromConfig(cfg)
	if err != nil {
		return Assignment{}, err
	}
	if assignment, ok := selectAssignment(projectID, activeAssignments(pool)); ok {
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
			Status:    AssignmentStatusActive,
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
	if assignment, ok := selectAssignment(email, activeAssignments(pool)); ok {
		return assignment, nil
	}
	return global, nil
}

func CurrentAssignmentForProject(cfg map[string]string, project *Project, seed string) (Assignment, bool, error) {
	stored := Assignment{}
	if project != nil {
		stored = Assignment{
			AgentID:   strings.TrimSpace(project.AgentID),
			MarqueeID: strings.TrimSpace(project.MarqueeID),
			Status:    AssignmentStatusActive,
		}
	}
	if stored.AgentID != "" || stored.MarqueeID != "" {
		return stored, false, nil
	}
	global := GlobalAssignment(cfg)
	if global.AgentID != "" {
		return global, !sameAssignment(stored, global), nil
	}
	pool, err := assignmentPoolFromConfig(cfg)
	if err != nil {
		return Assignment{}, false, err
	}
	if len(pool) == 0 {
		return stored, false, nil
	}
	current, ok := selectAssignment(seed, activeAssignments(pool))
	if !ok {
		return stored, false, nil
	}
	return current, !sameAssignment(stored, current), nil
}

func assignmentPoolFromConfig(cfg map[string]string) ([]Assignment, error) {
	return ParseAssignmentPool(firstNonEmpty(cfg["fibe_agent_server_pool"], cfg["fibe_agent_marquee_pool"]))
}

func AssignmentPoolFromConfig(cfg map[string]string) ([]Assignment, error) {
	return assignmentPoolFromConfig(cfg)
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
			Capacity:  firstPositiveRawInt(input.Capacity, input.MaxProjects, input.MaxProjectsAlt),
		}
		status, err := normalizeAssignmentStatus(firstNonEmpty(input.Status, input.StatusAlias))
		if err != nil {
			return nil, fmt.Errorf("agent/server pool row %d has invalid status: %w", i+1, err)
		}
		assignment.Status = status
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

func firstPositiveRawInt(values ...json.RawMessage) int {
	for _, raw := range values {
		value := positiveRawInt(raw)
		if value > 0 {
			return value
		}
	}
	return 0
}

func positiveRawInt(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var numeric int
	if err := json.Unmarshal(raw, &numeric); err == nil {
		if numeric > 0 {
			return numeric
		}
		return 0
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || value <= 0 {
		return 0
	}
	return value
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

func selectAssignment(seed string, pool []Assignment) (Assignment, bool) {
	seed = strings.ToLower(strings.TrimSpace(seed))
	if seed == "" || len(pool) == 0 {
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
		score := assignmentScore(seed, key)
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

func activeAssignments(pool []Assignment) []Assignment {
	out := make([]Assignment, 0, len(pool))
	for _, assignment := range pool {
		if AssignmentStatus(assignment) == AssignmentStatusActive {
			out = append(out, assignment)
		}
	}
	return out
}

func assignmentInPool(assignment Assignment, pool []Assignment) bool {
	if assignment.AgentID == "" {
		return false
	}
	for _, candidate := range pool {
		if sameAssignment(assignment, candidate) {
			return true
		}
	}
	return false
}

func sameAssignment(left Assignment, right Assignment) bool {
	return strings.TrimSpace(left.AgentID) == strings.TrimSpace(right.AgentID) &&
		strings.TrimSpace(left.MarqueeID) == strings.TrimSpace(right.MarqueeID)
}

func assignmentScore(email, key string) uint64 {
	sum := sha256.Sum256([]byte(email + "\x00" + key))
	return binary.BigEndian.Uint64(sum[:8])
}

func AssignmentStatus(assignment Assignment) string {
	status, err := normalizeAssignmentStatus(assignment.Status)
	if err != nil {
		return AssignmentStatusActive
	}
	return status
}

func AllowsExistingProjects(assignment Assignment) bool {
	switch AssignmentStatus(assignment) {
	case AssignmentStatusActive, AssignmentStatusDraining:
		return true
	default:
		return false
	}
}

func normalizeAssignmentStatus(raw string) (string, error) {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "" {
		return AssignmentStatusActive, nil
	}
	switch status {
	case AssignmentStatusActive, AssignmentStatusDraining, AssignmentStatusRetiring, AssignmentStatusRetired:
		return status, nil
	default:
		return "", fmt.Errorf("%q", raw)
	}
}
