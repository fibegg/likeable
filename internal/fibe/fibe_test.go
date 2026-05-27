package fibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	projecttext "github.com/fibegg/likeable/internal/project"
	"golang.org/x/image/bmp"
)

func testFibeBaseURL() string {
	return firstNonEmpty(os.Getenv("FIBE_URL"), "server.test:3000")
}

func testFibeNormalizedBaseURL() string {
	return normalizeFibeBaseURL(testFibeBaseURL())
}

func newTestClient(t *testing.T, server *httptest.Server, agentID, marqueeID string) *Client {
	t.Helper()
	if agentID == "" {
		agentID = "agent"
	}
	client, err := NewClient(Config{
		BaseURL:   server.URL,
		APIKey:    "test",
		AgentID:   agentID,
		MarqueeID: marqueeID,
		HTTP:      server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestIsRetryableProvisioningError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "internal error wrapped in validation status",
			err:  &PlatformError{Code: "INTERNAL_ERROR", Status: 422, Message: "unexpected status 422"},
			want: true,
		},
		{
			name: "server error",
			err:  &PlatformError{Code: "UNKNOWN_ERROR", Status: 502, Message: "bad gateway"},
			want: true,
		},
		{
			name: "locked conflict",
			err:  &PlatformError{Code: "CONFLICT", Status: 409, Message: "resource is locked, try again"},
			want: true,
		},
		{
			name: "validation failure",
			err:  &PlatformError{Code: "VALIDATION_FAILED", Status: 422, Message: "name is invalid"},
			want: false,
		},
		{
			name: "greenfield default missing through remote request",
			err:  &PlatformError{Code: "REMOTE_REQUEST_FAILED", Status: 422, Message: "fibe: REMOTE_REQUEST_FAILED (422): No default greenfield template version is configured"},
			want: false,
		},
		{
			name: "greenfield default unavailable",
			err:  &PlatformError{Code: "GREENFIELD_DEFAULT_TEMPLATE_VERSION_UNAVAILABLE", Status: 422, Message: "Default greenfield template version is configured but is not available"},
			want: false,
		},
		{
			name: "greenfield default waiting on source mirrors",
			err: &PlatformError{
				Code:    "GREENFIELD_DEFAULT_TEMPLATE_VERSION_UNAVAILABLE",
				Status:  422,
				Message: "Default greenfield template version is configured but is not available",
				Details: map[string]any{
					"mirrors_ready":   false,
					"missing_sources": []any{"https://github.com/fibegg/app"},
				},
			},
			want: true,
		},
		{
			name: "greenfield default mirror lag through remote request details",
			err: &PlatformError{
				Code:    "REMOTE_REQUEST_FAILED",
				Status:  422,
				Message: "fibe: GREENFIELD_DEFAULT_TEMPLATE_VERSION_UNAVAILABLE (422): Default greenfield template version is configured but is not available",
				Details: map[string]any{
					"mirrors_ready": false,
				},
			},
			want: true,
		},
		{
			name: "system template mirror unavailable through remote request",
			err:  &PlatformError{Code: "REMOTE_REQUEST_FAILED", Status: 422, Message: "fibe: SYSTEM_TEMPLATE_MIRROR_UNAVAILABLE (503): System template source mirror is not available for https://github.com/fibegg/app"},
			want: true,
		},
		{
			name: "configuration failure",
			err:  &PlatformError{Code: "FIBE_CONFIGURATION_ERROR", Message: "Fibe platform is not configured"},
			want: false,
		},
		{
			name: "runtime billing required",
			err:  &PlatformError{Code: "INTERNAL_ERROR", Status: 402, Message: "unexpected status 402"},
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("greenfield failed"),
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsRetryableProvisioningError(tc.err); got != tc.want {
				t.Fatalf("IsRetryableProvisioningError()=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestPlatformErrorPublicProjectErrorKindClassifiesGreenfieldConfiguration(t *testing.T) {
	for _, err := range []*PlatformError{
		{Code: "GREENFIELD_DEFAULT_TEMPLATE_VERSION_UNAVAILABLE", Status: 422, Message: "Default greenfield template version is configured but is not available"},
		{Code: "REMOTE_REQUEST_FAILED", Status: 422, Message: "fibe: REMOTE_REQUEST_FAILED (422): No default greenfield template version is configured"},
	} {
		if got := err.PublicProjectErrorKind(); got != "configuration" {
			t.Fatalf("PublicProjectErrorKind(%v)=%q, want configuration", err, got)
		}
	}
}

func TestPlatformErrorPublicProjectErrorKindDoesNotClassifyMirrorLagAsConfiguration(t *testing.T) {
	err := &PlatformError{Code: "REMOTE_REQUEST_FAILED", Status: 422, Message: "fibe: SYSTEM_TEMPLATE_MIRROR_UNAVAILABLE (503): System template source mirror is not available for https://github.com/fibegg/app"}
	if got := err.PublicProjectErrorKind(); got != "" {
		t.Fatalf("PublicProjectErrorKind(%v)=%q, want empty", err, got)
	}
}

func TestPlatformErrorPublicProjectErrorKindClassifiesRuntimeBilling(t *testing.T) {
	err := &PlatformError{Code: "INTERNAL_ERROR", Status: 402, Message: "unexpected status 402"}
	if got := err.PublicProjectErrorKind(); got != "runtime_billing" {
		t.Fatalf("PublicProjectErrorKind(%v)=%q, want runtime_billing", err, got)
	}
}

func TestNewClientConfiguresSDK(t *testing.T) {
	client, err := NewClient(Config{
		BaseURL: testFibeBaseURL(),
		APIKey:  "test",
		AgentID: "agent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.BaseURL() != testFibeNormalizedBaseURL() {
		t.Fatalf("baseURL=%q, want normalized platform URL", client.BaseURL())
	}
	if client.sdk == nil {
		t.Fatal("sdk client was not configured")
	}
}

func TestIsIdempotentConversationCreateError(t *testing.T) {
	if !IsIdempotentConversationCreateError(&PlatformError{Code: "INTERNAL_ERROR", Status: 422, Message: "conversation already exists"}) {
		t.Fatal("duplicate/upsert conversation failure should be treated as idempotent")
	}
	if IsIdempotentConversationCreateError(&PlatformError{Code: "INTERNAL_ERROR", Status: 422, Message: "fibe: INTERNAL_ERROR (422): unexpected status 422"}) {
		t.Fatal("generic 422 failure must not be treated as an idempotent duplicate")
	}
	if IsIdempotentConversationCreateError(&PlatformError{Code: "FIBE_CONFIGURATION_ERROR", Message: "Fibe platform is not configured"}) {
		t.Fatal("configuration failure must remain fatal")
	}
}

func TestIsPlaygroundAlreadyStoppedError(t *testing.T) {
	if !IsPlaygroundAlreadyStoppedError(&PlatformError{Code: "INVALID_STATE", Status: 422, Message: "Cannot stop playground from current status"}) {
		t.Fatal("already-stopped invalid state should be idempotent")
	}
	if IsPlaygroundAlreadyStoppedError(&PlatformError{Code: "INVALID_STATE", Status: 422, Message: "Cannot start playground from current status"}) {
		t.Fatal("start invalid state must not look like an already-stopped stop")
	}
	if IsPlaygroundAlreadyStoppedError(&PlatformError{Code: "VALIDATION_FAILED", Status: 422, Message: "Cannot stop playground from current status"}) {
		t.Fatal("unrelated error code must not be idempotent")
	}
}

func TestIsPlaygroundMissingError(t *testing.T) {
	for _, err := range []error{
		&PlatformError{Code: "NOT_FOUND", Status: 404, Message: "Playground not found"},
		&PlatformError{Code: "INTERNAL_ERROR", Status: 404, Message: "unexpected status 404"},
		&PlatformError{Code: "RESOURCE_NOT_FOUND", Status: 422, Message: "playground is missing"},
	} {
		if !IsPlaygroundMissingError(err) {
			t.Fatalf("IsPlaygroundMissingError(%v)=false, want true", err)
		}
	}
	if IsPlaygroundMissingError(&PlatformError{Code: "NOT_FOUND", Status: 404, Message: "Conversation not found"}) {
		t.Fatal("generic conversation 404 must not look like a playground miss when message is specific")
	}
	if IsPlaygroundMissingError(errors.New("playground not found")) {
		t.Fatal("plain errors must not look like platform playground misses")
	}
}

func TestIsRuntimeBillingRequiredError(t *testing.T) {
	for _, err := range []error{
		&PlatformError{Code: "INTERNAL_ERROR", Status: 402, Message: "unexpected status 402"},
		&PlatformError{Code: "MARQUEE_NOT_FUNDED", Status: 422, Message: "This Marquee is not funded. Fund it to continue."},
		errors.New("fibe: INTERNAL_ERROR (402): unexpected status 402"),
	} {
		if !IsRuntimeBillingRequiredError(err) {
			t.Fatalf("IsRuntimeBillingRequiredError(%v)=false, want true", err)
		}
	}
	if IsRuntimeBillingRequiredError(&PlatformError{Code: "INTERNAL_ERROR", Status: 422, Message: "unexpected status 422"}) {
		t.Fatal("generic internal 422 must not look like runtime billing")
	}
	if IsRuntimeBillingRequiredError(errors.New("payment settings page is unavailable")) {
		t.Fatal("generic payment text must not look like runtime billing")
	}
}

func TestIsAgentRuntimeUnavailableError(t *testing.T) {
	for _, err := range []error{
		&PlatformError{Code: "UNPROCESSABLE_ENTITY", Status: 422, Message: "No running AgentChat for Agent#1"},
		&PlatformError{Code: "NOT_FOUND", Status: 404, Message: "No running chat. Start a chat first."},
		&PlatformError{Code: "AGENT_COMMUNICATION_FAILED", Status: 422, Message: "Agent unreachable: connection refused"},
		errors.New("runtime reachable: no"),
	} {
		if !IsAgentRuntimeUnavailableError(err) {
			t.Fatalf("IsAgentRuntimeUnavailableError(%v)=false, want true", err)
		}
	}
	if IsAgentRuntimeUnavailableError(&PlatformError{Code: "FIBE_CONFIGURATION_ERROR", Message: "Fibe platform is not configured"}) {
		t.Fatal("configuration failure must not look like an agent runtime outage")
	}
}

func TestIsConversationMissingError(t *testing.T) {
	for _, err := range []error{
		&PlatformError{Code: "NOT_FOUND", Status: 404, Message: "Conversation not found"},
		&PlatformError{Code: "UNPROCESSABLE_ENTITY", Status: 422, Message: "HTTP 404: {\"message\":\"Conversation not found\"}"},
	} {
		if !IsConversationMissingError(err) {
			t.Fatalf("IsConversationMissingError(%v)=false, want true", err)
		}
	}
	if IsConversationMissingError(&PlatformError{Code: "NOT_FOUND", Status: 404, Message: "Playground not found"}) {
		t.Fatal("unrelated not found failure must not look like a missing conversation")
	}
}

func TestStartAgentChatUsesConfiguredMarquee(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/agents/agent-1/chats" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test" {
			t.Fatalf("Authorization=%q, want bearer key", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		writeJSONResponse(t, w, map[string]any{"id": 1, "status": "running"})
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent-1", "multipass")
	if err := client.StartAgentChat(t.Context()); err != nil {
		t.Fatal(err)
	}
	if body["marquee_id"] != "multipass" {
		t.Fatalf("body=%#v, want configured marquee", body)
	}
}

func TestControlPlaygroundLifecycleUsesSDKActions(t *testing.T) {
	var actions []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/playgrounds/123/operations" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, fmt.Sprint(body["action_type"]))
		writeJSONResponse(t, w, map[string]any{"id": 123, "status": "running"})
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent-1", "")
	if err := client.StartPlayground(t.Context(), "123"); err != nil {
		t.Fatal(err)
	}
	if err := client.StopPlayground(t.Context(), "123"); err != nil {
		t.Fatal(err)
	}
	if err := client.RestartPlayground(t.Context(), "123"); err != nil {
		t.Fatal(err)
	}
	want := []string{"start", "stop", "hard_restart"}
	if strings.Join(actions, ",") != strings.Join(want, ",") {
		t.Fatalf("actions=%v, want %v", actions, want)
	}
}

func TestCreateGreenfieldUsesTemplateVersionIDOnlyWhenConfigured(t *testing.T) {
	for _, tc := range []struct {
		name              string
		templateVersionID string
		wantPresent       bool
	}{
		{name: "omitted", templateVersionID: "", wantPresent: false},
		{name: "present", templateVersionID: "42", wantPresent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			templatePath := filepath.Join(t.TempDir(), "template.yml")
			if err := os.WriteFile(templatePath, []byte("services:\n  app:\n    image: test\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("LIKEABLE_GREENFIELD_TEMPLATE_BODY_PATH", templatePath)
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/greenfields" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				writeJSONResponse(t, w, map[string]any{
					"name": "test-app-0123456789abcdef",
					"playground": map[string]any{
						"id":          123,
						"name":        "test-app-0123456789abcdef",
						"playspec_id": 456,
					},
					"playspec": map[string]any{"id": 456},
					"prop":     map[string]any{"id": 789},
					"repo":     map[string]any{"repository_url": "http://gitea.test/owner/repo.git"},
					"service_urls": []map[string]any{
						{"name": "app", "type": "dynamic", "url": "http://lk-test.phoenix.test", "visibility": "external"},
					},
				})
			}))
			defer server.Close()
			client := newTestClient(t, server, "agent", "marquee-1")
			client.templateVersionID = tc.templateVersionID
			project := &Project{
				ID:             "01234567-89ab-cdef-0123-456789abcdef",
				Title:          "Test app",
				ConversationID: "likeable-0123456789abcdef0123456789abcdef",
			}
			result, err := client.CreateGreenfield(t.Context(), project)
			if err != nil {
				t.Fatal(err)
			}
			if result.PlaygroundID != "123" || result.PlayspecID != "456" || result.PropID != "789" {
				t.Fatalf("parsed ids=%+v, want playground=123 playspec=456 prop=789", result)
			}
			if result.PlaygroundName != "test-app-0123456789abcdef" {
				t.Fatalf("playground name=%q, want deterministic fallback", result.PlaygroundName)
			}
			if body["name"] != "test-app-0123456789abcdef" {
				t.Fatalf("body=%#v, want deterministic name", body)
			}
			if body["git_provider"] != "gitea" || body["private"] != true {
				t.Fatalf("body=%#v, want private gitea greenfield", body)
			}
			hasTemplateVersionID := body["template_version_id"] != nil
			if hasTemplateVersionID != tc.wantPresent {
				t.Fatalf("template_version_id present=%v, want %v; body=%#v", hasTemplateVersionID, tc.wantPresent, body)
			}
			if tc.wantPresent && body["template_version_id"] != float64(42) {
				t.Fatalf("body=%#v, want template_version_id 42", body)
			}
			if !tc.wantPresent && !strings.Contains(fmt.Sprint(body["template_body"]), "services:") {
				t.Fatalf("body=%#v, want bundled template body", body)
			}
			subdomains := body["service_subdomains"].(map[string]any)
			if subdomains["app"] != "lk-0123456789abcdef" {
				t.Fatalf("subdomains=%#v, want app service subdomain", subdomains)
			}
			if subdomains["admin"] != "lk-0123456789abcdef-admin" {
				t.Fatalf("subdomains=%#v, want admin service subdomain", subdomains)
			}
			vars := body["variables"].(map[string]any)
			if vars["app_subdomain"] != "lk-0123456789abcdef" {
				t.Fatalf("variables=%#v, want app subdomain variable", vars)
			}
			if vars["subdomain"] != "lk-0123456789abcdef" {
				t.Fatalf("variables=%#v, want generic subdomain variable", vars)
			}
			if vars["SUBDOMAIN"] != "lk-0123456789abcdef" {
				t.Fatalf("variables=%#v, want uppercase generic subdomain variable", vars)
			}
			if vars["admin_subdomain"] != "lk-0123456789abcdef-admin" {
				t.Fatalf("variables=%#v, want admin subdomain variable", vars)
			}
		})
	}
}

func TestCreateGreenfieldRetriesWithoutUnknownServiceSubdomain(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/greenfields" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, body)
		if len(requests) == 1 {
			writeJSONStatus(t, w, http.StatusUnprocessableEntity, map[string]any{
				"error": map[string]any{
					"code":    "REMOTE_REQUEST_FAILED",
					"message": "fibe: REMOTE_REQUEST_FAILED (422): service_subdomains are invalid: unknown exposed service(s): admin",
				},
			})
			return
		}
		writeJSONResponse(t, w, map[string]any{
			"playground":   map[string]any{"id": 123},
			"playspec":     map[string]any{"id": 456},
			"prop":         map[string]any{"id": 789},
			"repo":         map[string]any{"repository_url": "http://gitea.test/owner/repo.git"},
			"service_urls": []map[string]any{{"name": "app", "type": "dynamic", "url": "http://lk-test.phoenix.test", "visibility": "external"}},
		})
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "30")
	project := &Project{
		ID:             "01234567-89ab-cdef-0123-456789abcdef",
		Title:          "Test app",
		ConversationID: "likeable-0123456789abcdef0123456789abcdef",
	}
	result, err := client.CreateGreenfield(t.Context(), project)
	if err != nil {
		t.Fatal(err)
	}
	if result.PlaygroundID != "123" {
		t.Fatalf("playground id=%q, want retry success", result.PlaygroundID)
	}
	if len(requests) != 2 {
		t.Fatalf("requests=%v, want initial failed request plus retry", requests)
	}
	firstSubdomains := requests[0]["service_subdomains"].(map[string]any)
	if firstSubdomains["admin"] != "lk-0123456789abcdef-admin" {
		t.Fatalf("first subdomains=%#v, want admin subdomain", firstSubdomains)
	}
	retrySubdomains := requests[1]["service_subdomains"].(map[string]any)
	if _, ok := retrySubdomains["admin"]; ok {
		t.Fatalf("retry subdomains=%#v, should omit unknown admin", retrySubdomains)
	}
	if retrySubdomains["app"] != "lk-0123456789abcdef" {
		t.Fatalf("retry subdomains=%#v, should keep app subdomain", retrySubdomains)
	}
	if requests[1]["marquee_id"] != "30" {
		t.Fatalf("retry body=%#v, should keep stored marquee id", requests[1])
	}
}

func TestGreenfieldSubdomainsAreStableDNSLabels(t *testing.T) {
	a := projecttext.PreviewSubdomain(&Project{ID: "9447f6b8-3d10-47d6-a946-0afc6bdd4406"})
	b := projecttext.PreviewSubdomain(&Project{ID: "f57dc08b-5e22-4b84-bdf4-e77ca7b94c99"})
	if a != "lk-9447f6b83d1047d6" {
		t.Fatalf("subdomain=%q, want stable UUID-derived value", a)
	}
	if a == b {
		t.Fatalf("different project IDs produced same subdomain %q", a)
	}
	for _, subdomain := range []string{a, "ws-" + a, b, "ws-" + b} {
		if len(subdomain) > 63 {
			t.Fatalf("subdomain %q length=%d exceeds DNS label limit", subdomain, len(subdomain))
		}
		for _, r := range subdomain {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				t.Fatalf("subdomain %q contains non-DNS-safe rune %q", subdomain, r)
			}
		}
	}
}

func TestFibeAssignmentPoolMapsSeedDeterministically(t *testing.T) {
	pool := []Assignment{
		{Label: "A", AgentID: "agent-a", MarqueeID: "marquee-a"},
		{Label: "B", AgentID: "agent-b", MarqueeID: "marquee-b"},
	}
	reversed := []Assignment{pool[1], pool[0]}

	first, ok := selectAssignment("Project-A", pool)
	if !ok {
		t.Fatal("expected assignment")
	}
	second, ok := selectAssignment("project-a", reversed)
	if !ok {
		t.Fatal("expected assignment from reversed pool")
	}
	if first != second {
		t.Fatalf("assignment changed with case/order: first=%+v second=%+v", first, second)
	}
}

func TestFibeAssignmentFallsBackToGlobalConfig(t *testing.T) {
	assignment, err := AssignmentForNewProject(map[string]string{
		"fibe_agent_id":   "global-agent",
		"fibe_marquee_id": "global-marquee",
	}, "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if assignment.AgentID != "global-agent" || assignment.MarqueeID != "global-marquee" {
		t.Fatalf("assignment=%+v, want global pair", assignment)
	}
}

func TestCurrentAssignmentForProjectKeepsStoredPairEvenWhenMissingFromPool(t *testing.T) {
	assignment, changed, err := CurrentAssignmentForProject(map[string]string{
		"fibe_agent_server_pool": `[{"agent_id":"current-agent","server_id":"current-marquee"}]`,
	}, &Project{AgentID: "stale-agent", MarqueeID: "stale-marquee"}, "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("changed=true, want stored project binding preserved")
	}
	if assignment.AgentID != "stale-agent" || assignment.MarqueeID != "stale-marquee" {
		t.Fatalf("assignment=%+v, want stored pair", assignment)
	}
}

func TestCurrentAssignmentForProjectKeepsStoredPairInPool(t *testing.T) {
	assignment, changed, err := CurrentAssignmentForProject(map[string]string{
		"fibe_agent_server_pool": `[{"agent_id":"stored-agent","server_id":"stored-marquee"},{"agent_id":"other-agent","server_id":"other-marquee"}]`,
	}, &Project{AgentID: "stored-agent", MarqueeID: "stored-marquee"}, "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("changed=true, want stored pool assignment preserved")
	}
	if assignment.AgentID != "stored-agent" || assignment.MarqueeID != "stored-marquee" {
		t.Fatalf("assignment=%+v, want stored pool pair", assignment)
	}
}

func TestParseAssignmentPoolDefaultsStatusActive(t *testing.T) {
	pool, err := ParseAssignmentPool(`[{"agent_id":"agent-1","server_id":"server-1"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(pool) != 1 || pool[0].Status != AssignmentStatusActive {
		t.Fatalf("pool=%+v, want active status", pool)
	}
}

func TestParseAssignmentPoolPreservesCapacity(t *testing.T) {
	pool, err := ParseAssignmentPool(`[{"agentId":"agent-1","serverId":"server-1","maxProjects":"200"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(pool) != 1 || pool[0].Capacity != 200 {
		t.Fatalf("pool=%+v, want capacity 200", pool)
	}
	if encoded := EncodeAssignmentPool(pool); !strings.Contains(encoded, `"capacity":200`) {
		t.Fatalf("encoded=%s, want capacity preserved", encoded)
	}
}

func TestParseAssignmentPoolRejectsInvalidStatus(t *testing.T) {
	_, err := ParseAssignmentPool(`[{"agent_id":"agent-1","server_id":"server-1","status":"paused"}]`)
	if err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("err=%v, want invalid status error", err)
	}
}

func TestAssignmentForNewProjectUsesOnlyActiveRowsAndProjectSeed(t *testing.T) {
	cfg := map[string]string{
		"fibe_agent_server_pool": `[
			{"agent_id":"agent-a","server_id":"server-a","status":"draining"},
			{"agent_id":"agent-b","server_id":"server-b","status":"retired"},
			{"agent_id":"agent-c","server_id":"server-c","status":"active"}
		]`,
	}
	assignment, err := AssignmentForNewProject(cfg, "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if assignment.AgentID != "agent-c" || assignment.MarqueeID != "server-c" {
		t.Fatalf("assignment=%+v, want only active pair", assignment)
	}
}

func TestAssignmentForNewProjectVariesByProjectID(t *testing.T) {
	cfg := map[string]string{
		"fibe_agent_server_pool": `[{"agent_id":"agent-a","server_id":"server-a"},{"agent_id":"agent-b","server_id":"server-b"}]`,
	}
	first, err := AssignmentForNewProject(cfg, "project-0")
	if err != nil {
		t.Fatal(err)
	}
	second, err := AssignmentForNewProject(cfg, "project-2")
	if err != nil {
		t.Fatal(err)
	}
	if first.AgentID == second.AgentID && first.MarqueeID == second.MarqueeID {
		t.Fatalf("assignments=%+v/%+v, want different project IDs to be able to land on different pairs", first, second)
	}
}

func TestParseGreenfieldStatusPrefersAppPreviewURL(t *testing.T) {
	result := parseGreenfieldStatus(map[string]any{
		"status": "success",
		"service_urls": []any{
			map[string]any{"name": "ws", "type": "static", "url": "http://ws-starter.phoenix.test", "visibility": "external"},
			map[string]any{"name": "app", "type": "dynamic", "url": "http://starter.phoenix.test", "visibility": "external"},
		},
	})
	if result.PreviewURL != "https://starter.phoenix.test" {
		t.Fatalf("PreviewURL=%q, want app URL", result.PreviewURL)
	}
}

func TestParseGreenfieldStatusReturnsAllServicesAndRepos(t *testing.T) {
	result := parseGreenfieldStatus(map[string]any{
		"status": "success",
		"props": []any{
			map[string]any{"id": float64(11), "repository_url": "http://gitea.test/owner/app.git", "service_names": []any{"app"}},
			map[string]any{"id": float64(12), "repository_url": "http://gitea.test/owner/admin.git", "service_names": []any{"admin"}},
		},
		"repos": []any{
			map[string]any{"repository_url": "http://gitea.test/owner/app", "source_repo_url": "https://github.com/fibegg/go-fibe-app", "service_names": []any{"app"}},
			map[string]any{"repository_url": "http://gitea.test/owner/backend.git", "source_repo_url": "https://github.com/fibegg/go-fibe", "service_names": []any{"web", "worker"}},
		},
		"service_urls": []any{
			map[string]any{"name": "admin", "type": "dynamic", "url": "http://admin.phoenix.test", "visibility": "external"},
			map[string]any{"name": "app", "type": "dynamic", "url": "http://app.phoenix.test", "visibility": "external"},
		},
	})

	if result.PreviewURL != "https://app.phoenix.test" || result.SelectedServiceName != "app" {
		t.Fatalf("selected service=%q url=%q", result.SelectedServiceName, result.PreviewURL)
	}
	if len(result.Services) != 2 {
		t.Fatalf("services=%+v, want 2", result.Services)
	}
	if len(result.Repositories) != 3 {
		t.Fatalf("repositories=%+v, want 3", result.Repositories)
	}
	if result.repositoryForService("app").SourceRepoURL != "https://github.com/fibegg/go-fibe-app" {
		t.Fatalf("app repo did not merge source metadata: %+v", result.repositoryForService("app"))
	}
}

func TestCreateGreenfieldRecoversFailureByPlaygroundName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case http.MethodPost + " /api/greenfields":
			writeJSONStatus(t, w, http.StatusUnprocessableEntity, map[string]any{
				"error": map[string]any{"code": "INTERNAL_ERROR", "message": "unexpected status 422"},
			})
		case http.MethodGet + " /api/playgrounds/hope-1111111122223333":
			writeJSONResponse(t, w, map[string]any{"id": 41, "name": "hope-1111111122223333", "status": "in_progress", "playspec_id": 44})
		case http.MethodGet + " /api/playgrounds/41/debug":
			writeJSONResponse(t, w, map[string]any{"diagnostics": map[string]any{
				"playground": map[string]any{"id": 41, "name": "hope-1111111122223333", "playspec_id": 44, "status": "running"},
				"routes":     []map[string]any{{"service": "app", "playground_subdomain": "lk-1111111122223333", "traefik_host": "lk-1111111122223333.troll-wish-copper.fibe.work"}},
			}})
		case http.MethodGet + " /api/playspecs/44":
			writeJSONResponse(t, w, map[string]any{"id": 44, "services": []map[string]any{{"name": "app", "type": "dynamic", "prop_id": 24, "repo_url": "https://git.example.test/owner/hope"}}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "21")
	result, err := client.CreateGreenfield(t.Context(), &Project{
		ID:    "11111111-2222-3333-4444-555555555555",
		Title: "Hope",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PlaygroundID != "41" || result.PlaygroundName != "hope-1111111122223333" {
		t.Fatalf("result=%+v, want recovered playground 41 by name", result)
	}
	if result.PreviewURL != "https://lk-1111111122223333.troll-wish-copper.fibe.work" {
		t.Fatalf("PreviewURL=%q", result.PreviewURL)
	}
}

func TestCreateGreenfieldRecoversHeadlessLinkFailureBySubdomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case http.MethodPost + " /api/greenfields":
			writeJSONStatus(t, w, http.StatusNotFound, map[string]any{
				"error": map[string]any{"code": "LOCAL_PLAYGROUNDS_DIR_MISSING", "message": "directory /opt/fibe/playgrounds does not exist"},
			})
		case http.MethodGet + " /api/playgrounds/hope-1111111122223333":
			writeJSONStatus(t, w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "RESOURCE_NOT_FOUND", "message": "not found"}})
		case http.MethodGet + " /api/playgrounds":
			writeJSONResponse(t, w, map[string]any{
				"data": []map[string]any{{"id": 32, "name": "hope-ugsj58bp", "status": "running", "playspec_id": 42}},
				"meta": map[string]any{"page": 1, "per_page": 100, "total": 1},
			})
		case http.MethodGet + " /api/playgrounds/32/debug":
			writeJSONResponse(t, w, map[string]any{"diagnostics": map[string]any{
				"playground": map[string]any{"id": 32, "playspec_id": 42, "status": "running"},
				"routes":     []map[string]any{{"service": "frontend", "playground_subdomain": "lk-1111111122223333", "traefik_host": "lk-1111111122223333.troll-wish-copper.fibe.work"}},
			}})
		case http.MethodGet + " /api/playspecs/42":
			writeJSONResponse(t, w, map[string]any{"id": 42, "services": []map[string]any{{"name": "frontend", "type": "dynamic", "prop_id": 23, "repo_url": "https://git.example.test/owner/hope"}}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "21")
	result, err := client.CreateGreenfield(t.Context(), &Project{
		ID:    "11111111-2222-3333-4444-555555555555",
		Title: "Hope",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PlaygroundID != "32" || result.PlayspecID != "42" || result.PropID != "23" {
		t.Fatalf("result ids=%+v, want playground=32 playspec=42 prop=23", result)
	}
	if result.PreviewURL != "https://lk-1111111122223333.troll-wish-copper.fibe.work" {
		t.Fatalf("PreviewURL=%q", result.PreviewURL)
	}
	if result.RepoURL != "https://git.example.test/owner/hope" {
		t.Fatalf("RepoURL=%q", result.RepoURL)
	}
}

func TestFrameBlockingHeaderDetectsIframeBlockers(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Frame-Options", "SAMEORIGIN")
	if got := frameBlockingHeader(headers); got == "" {
		t.Fatal("expected X-Frame-Options to be treated as frame-blocking")
	}
}

func TestSendMessageUploadsConversationAttachments(t *testing.T) {
	attachmentPath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(attachmentPath, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	var uploadConversationID string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case http.MethodPost + " /api/agents/agent/uploads":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			uploadConversationID = r.FormValue("conversation_id")
			writeJSONResponse(t, w, map[string]any{"filename": "uploaded-notes.txt"})
		case http.MethodPost + " /api/agents/agent/messages":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			writeJSONResponse(t, w, map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "")
	if err := client.SendMessage(t.Context(), "conv-1", "Use attachment", []string{attachmentPath}, "steer"); err != nil {
		t.Fatal(err)
	}
	if uploadConversationID != "conv-1" {
		t.Fatalf("upload conversation_id=%q, want conv-1", uploadConversationID)
	}
	if payload["text"] != "Use attachment" {
		t.Fatalf("payload=%#v, want text", payload)
	}
	if payload["conversation_id"] != "conv-1" || payload["busy_policy"] != "steer" {
		t.Fatalf("payload=%#v, want conversation and busy policy", payload)
	}
	attachments := payload["attachmentFilenames"].([]any)
	if len(attachments) != 1 || attachments[0] != "uploaded-notes.txt" {
		t.Fatalf("attachments=%#v, want uploaded filename", attachments)
	}
}

func TestSendMessageUploadsImageAttachments(t *testing.T) {
	attachmentPath := filepath.Join(t.TempDir(), "screenshot.png")
	if err := os.WriteFile(attachmentPath, []byte("png-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	var uploadConversationID string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case http.MethodPost + " /api/agents/agent/uploads":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			uploadConversationID = r.FormValue("conversation_id")
			writeJSONResponse(t, w, map[string]any{"filename": "uploaded-screenshot.png"})
		case http.MethodPost + " /api/agents/agent/messages":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			writeJSONResponse(t, w, map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "")
	if err := client.SendMessage(t.Context(), "conv-1", "Use screenshot", []string{attachmentPath}, "queue"); err != nil {
		t.Fatal(err)
	}
	if uploadConversationID != "conv-1" {
		t.Fatalf("upload conversation_id=%q, want conv-1", uploadConversationID)
	}
	if payload["text"] != "Use screenshot" {
		t.Fatalf("payload=%#v, want text", payload)
	}
	if _, ok := payload["images"]; ok {
		t.Fatalf("payload=%#v, want images omitted", payload)
	}
	attachments := payload["attachmentFilenames"].([]any)
	if len(attachments) != 1 || attachments[0] != "uploaded-screenshot.png" {
		t.Fatalf("attachments=%#v, want uploaded filename", attachments)
	}
}

func TestSendMessageUploadsSniffedExtensionlessImageAttachments(t *testing.T) {
	attachmentPath := filepath.Join(t.TempDir(), "screenshot")
	if err := os.WriteFile(attachmentPath, []byte("\x89PNG\r\n\x1a\npng-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	var uploadCalled bool
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case http.MethodPost + " /api/agents/agent/uploads":
			uploadCalled = true
			writeJSONResponse(t, w, map[string]any{"filename": "uploaded-extensionless.png"})
		case http.MethodPost + " /api/agents/agent/messages":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			writeJSONResponse(t, w, map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "")
	if err := client.SendMessage(t.Context(), "conv-1", "Use screenshot", []string{attachmentPath}, "queue"); err != nil {
		t.Fatal(err)
	}
	if !uploadCalled {
		t.Fatal("expected extensionless image upload")
	}
	if _, ok := payload["images"]; ok {
		t.Fatalf("payload=%#v, want images omitted", payload)
	}
	attachments := payload["attachmentFilenames"].([]any)
	if len(attachments) != 1 || attachments[0] != "uploaded-extensionless.png" {
		t.Fatalf("attachments=%#v, want uploaded filename", attachments)
	}
}

func TestSendMessageConvertsUnsupportedImageAttachmentsBeforeUpload(t *testing.T) {
	attachmentPath := filepath.Join(t.TempDir(), "reference.bmp")
	writeTestBMP(t, attachmentPath)
	var uploadedFilename string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case http.MethodPost + " /api/agents/agent/uploads":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			files := r.MultipartForm.File["file"]
			if len(files) != 1 {
				t.Fatalf("upload files=%#v, want one file", files)
			}
			uploadedFilename = files[0].Filename
			writeJSONResponse(t, w, map[string]any{"filename": "uploaded-reference.jpg"})
		case http.MethodPost + " /api/agents/agent/messages":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			writeJSONResponse(t, w, map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "")
	if err := client.SendMessage(t.Context(), "conv-1", "Use screenshot", []string{attachmentPath}, "queue"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(uploadedFilename, "likeable-attachment-") || !strings.HasSuffix(uploadedFilename, ".jpg") {
		t.Fatalf("uploaded filename=%q, want converted JPEG temp file", uploadedFilename)
	}
	if _, ok := payload["images"]; ok {
		t.Fatalf("payload=%#v, want images omitted", payload)
	}
	attachments := payload["attachmentFilenames"].([]any)
	if len(attachments) != 1 || attachments[0] != "uploaded-reference.jpg" {
		t.Fatalf("attachments=%#v, want uploaded filename", attachments)
	}
}

func TestSendMessageConvertsLargePixelImageAttachmentsBeforeUpload(t *testing.T) {
	attachmentPath := filepath.Join(t.TempDir(), "screenshot.png")
	writeTestPNG(t, attachmentPath, 2400, 1800)
	var uploadedFilename string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case http.MethodPost + " /api/agents/agent/uploads":
			if err := r.ParseMultipartForm(8 << 20); err != nil {
				t.Fatal(err)
			}
			files := r.MultipartForm.File["file"]
			if len(files) != 1 {
				t.Fatalf("upload files=%#v, want one file", files)
			}
			uploadedFilename = files[0].Filename
			writeJSONResponse(t, w, map[string]any{"filename": "uploaded-screenshot.jpg"})
		case http.MethodPost + " /api/agents/agent/messages":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			writeJSONResponse(t, w, map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "")
	if err := client.SendMessage(t.Context(), "conv-1", "Use screenshot", []string{attachmentPath}, "queue"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(uploadedFilename, "likeable-attachment-") || !strings.HasSuffix(uploadedFilename, ".jpg") {
		t.Fatalf("uploaded filename=%q, want converted JPEG temp file", uploadedFilename)
	}
	if _, ok := payload["images"]; ok {
		t.Fatalf("payload=%#v, want images omitted", payload)
	}
	attachments := payload["attachmentFilenames"].([]any)
	if len(attachments) != 1 || attachments[0] != "uploaded-screenshot.jpg" {
		t.Fatalf("attachments=%#v, want uploaded filename", attachments)
	}
}

func TestMessagesAndActivityFallBackToRuntimeWhenFibeSyncIsEmpty(t *testing.T) {
	runtimeStatusCalls := 0
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/conversations/conv-1/messages":
			writeJSONResponse(t, w, []map[string]any{
				{"role": "user", "body": "hello"},
				{"role": "assistant", "body": "done"},
			})
		case "/api/conversations/conv-1/activities":
			writeJSONResponse(t, w, []map[string]any{
				{"id": "activity-1", "type": "stream_start"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer runtime.Close()
	fibeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/agents/agent/messages", "/api/agents/agent/activity":
			writeJSONResponse(t, w, map[string]any{"content": []any{}})
		case "/api/agents/agent/runtime_status":
			runtimeStatusCalls++
			if got := r.Header.Get("Authorization"); got != "Bearer test" {
				t.Fatalf("Authorization=%q, want bearer key", got)
			}
			writeJSONResponse(t, w, map[string]any{"chat_url": runtime.URL})
		default:
			http.NotFound(w, r)
		}
	}))
	defer fibeAPI.Close()
	client := newTestClient(t, fibeAPI, "agent", "")
	messages, err := client.Messages(t.Context(), "conv-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[1].(map[string]any)["body"] != "done" {
		t.Fatalf("messages=%#v, want runtime messages", messages)
	}
	activity, err := client.Activity(t.Context(), "conv-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(activity) != 1 || activity[0].(map[string]any)["id"] != "activity-1" {
		t.Fatalf("activity=%#v, want runtime activity", activity)
	}
	if runtimeStatusCalls != 1 {
		t.Fatalf("runtimeStatusCalls=%d, want cached chat URL", runtimeStatusCalls)
	}
}

func TestConversationLiveStateFetchesRuntimeStreamState(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/agents/agent/live_state" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		query = r.URL.RawQuery
		writeJSONResponse(t, w, map[string]any{"content": map[string]any{
			"conversation_id": "conv-1",
			"isProcessing":    true,
			"streamText":      "working",
			"queuedTurns":     1,
		}})
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "")
	live, err := client.ConversationLiveState(t.Context(), "conv-1")
	if err != nil {
		t.Fatal(err)
	}
	if !live.IsProcessing || live.StreamText == "" || live.QueuedTurns != 1 {
		t.Fatalf("live=%+v, want processing stream state", live)
	}
	if query != "conversation_id=conv-1" {
		t.Fatalf("query=%q, want conversation_id=conv-1", query)
	}
}

func TestDeleteProjectResourcesDeletesFibeAndGiteaResources(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.Method + " " + r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		paths = append(paths, path)
		switch r.Method + " " + r.URL.Path {
		case http.MethodGet + " /api/playspecs/456":
			writeJSONResponse(t, w, map[string]any{
				"id":                      456,
				"source_template":         map[string]any{"id": 321, "name": "shared-template"},
				"source_template_version": map[string]any{"id": 654},
			})
		case http.MethodGet + " /api/import_templates/321/versions":
			writeJSONResponse(t, w, map[string]any{
				"data": []map[string]any{{"id": 654, "source": map[string]any{"prop_id": 789, "prop_repository_url": serverURLFromRequest(r) + "/owner/repo.git"}}},
				"meta": map[string]any{"page": 1, "per_page": 25, "total": 1},
			})
		case http.MethodDelete + " /api/playgrounds/123",
			http.MethodDelete + " /api/playspecs/456",
			http.MethodDelete + " /api/import_templates/321/versions/654",
			http.MethodDelete + " /api/import_templates/321",
			http.MethodDelete + " /api/props/789",
			http.MethodDelete + " /api/props/790",
			http.MethodDelete + " /api/agents/agent/conversations":
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet + " /api/agents/agent/gitea_token":
			writeJSONResponse(t, w, map[string]any{"token": "gitea-token", "gitea_host": serverURLFromRequest(r), "username": "agent"})
		case http.MethodDelete + " /api/v1/repos/owner/repo":
			if got := r.Header.Get("Authorization"); got != "token gitea-token" {
				t.Fatalf("Authorization=%q, want token gitea-token", got)
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete + " /api/v1/repos/owner/admin":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server, "agent", "")
	err := client.DeleteProjectResources(t.Context(), &Project{
		PlaygroundID: "123",
		PlayspecID:   "456",
		PropID:       "789",
		RepoURL:      server.URL + "/owner/repo.git",
		Repositories: []ProjectRepository{
			{PropID: "789", RepoURL: server.URL + "/owner/repo.git", ServiceNames: []string{"app"}},
			{PropID: "790", RepoURL: server.URL + "/owner/admin.git", ServiceNames: []string{"admin"}},
		},
		ConversationID: "likeable-123",
		Title:          "Renamed project",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"GET /api/playspecs/456",
		"GET /api/import_templates/321/versions",
		"DELETE /api/playgrounds/123",
		"DELETE /api/playspecs/456",
		"DELETE /api/import_templates/321/versions/654",
		"DELETE /api/import_templates/321",
		"DELETE /api/props/789",
		"DELETE /api/props/790",
		"DELETE /api/agents/agent/conversations?conversation_id=likeable-123",
		"GET /api/agents/agent/gitea_token",
		"DELETE /api/v1/repos/owner/repo",
		"DELETE /api/v1/repos/owner/admin",
	} {
		if !containsString(paths, path) {
			t.Fatalf("missing request %s; got %v", path, paths)
		}
	}
}

func TestDeleteProjectResourcesTreatsMissingRemoteResourcesAsDeleted(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.Method + " " + r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		paths = append(paths, path)
		writeJSONStatus(t, w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "Resource not found", "code": "RESOURCE_NOT_FOUND"}})
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "")
	if err := client.DeleteProjectResources(t.Context(), &Project{PlaygroundID: "missing", ConversationID: "conv-missing"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"DELETE /api/playgrounds/missing",
		"DELETE /api/agents/agent/conversations?conversation_id=conv-missing",
	} {
		if !containsString(paths, want) {
			t.Fatalf("missing request %q; paths=%v", want, paths)
		}
	}
}

func TestDeleteProjectResourcesSkipsGiteaRepoWhenTokenEndpointIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents/agent/gitea_token" {
			writeJSONStatus(t, w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "Resource not found", "code": "RESOURCE_NOT_FOUND"}})
			return
		}
		t.Fatalf("unexpected request after missing token endpoint: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "")
	if err := client.DeleteProjectResources(t.Context(), &Project{RepoURL: server.URL + "/owner/repo.git"}); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteProjectResourcesDoesNotRetryPermanentAPIFailures(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSONStatus(t, w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "unauthorized", "code": "UNAUTHORIZED"}})
	}))
	defer server.Close()
	client := newTestClient(t, server, "agent", "")
	start := time.Now()
	if err := client.DeleteProjectResources(t.Context(), &Project{PlaygroundID: "123"}); err == nil {
		t.Fatal("expected API failure")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("permanent API failure took %s; want no long retry", elapsed)
	}
	if calls != 1 {
		t.Fatalf("calls=%d, want no retry", calls)
	}
}

func TestResourceDeleteRetryableUsesStructuredPlatformError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		deleted   bool
		retryable bool
	}{
		{
			name:    "not found is deleted",
			err:     &PlatformError{Code: "RESOURCE_NOT_FOUND", Status: http.StatusNotFound, Message: "missing"},
			deleted: true,
		},
		{
			name:      "locked status retries",
			err:       &PlatformError{Code: "LOCKED", Status: http.StatusLocked, Message: "locked"},
			retryable: true,
		},
		{
			name:      "busy detail retries",
			err:       &PlatformError{Code: "VALIDATION_FAILED", Status: http.StatusUnprocessableEntity, Message: "busy", Details: map[string]any{"current_status": "destroying"}},
			retryable: true,
		},
		{
			name: "unauthorized does not retry",
			err:  &PlatformError{Code: "UNAUTHORIZED", Status: http.StatusUnauthorized, Message: "unauthorized"},
		},
		{
			name: "unknown platform error does not retry",
			err:  &PlatformError{Code: platformCodeUnknown, Message: "404 Not Found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resourceAlreadyDeleted(tt.err); got != tt.deleted {
				t.Fatalf("resourceAlreadyDeleted()=%t want %t", got, tt.deleted)
			}
			if got := resourceDeleteRetryable(tt.err); got != tt.retryable {
				t.Fatalf("resourceDeleteRetryable()=%t want %t", got, tt.retryable)
			}
		})
	}
}

func TestProbePreviewURLReturnsEmbeddingBlockedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ready, _, err := ProbePreviewURL(t.Context(), server.Client(), server.URL)
	if ready {
		t.Fatal("preview should not be ready when embedding is blocked")
	}
	var blocked *PreviewEmbeddingBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected PreviewEmbeddingBlockedError, got %T: %v", err, err)
	}
}

func TestProbePreviewURLResultRecognizesMaintenancePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("<!doctype html><title>Maintenance</title><h1>maintenance is ongoing</h1>"))
	}))
	defer server.Close()

	result, err := ProbePreviewURLResult(t.Context(), server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready {
		t.Fatal("maintenance should not mark the runtime preview ready")
	}
	if !result.Displayable || !result.Maintenance {
		t.Fatalf("result=%+v, want displayable maintenance page", result)
	}

	ready, _, err := ProbePreviewURL(t.Context(), server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Fatal("legacy preview readiness should remain false for maintenance")
	}
}

func TestProbePreviewURLResultRecognizesMaintenanceHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Fibe-Maintenance", "true")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("maintenance"))
	}))
	defer server.Close()

	result, err := ProbePreviewURLResult(t.Context(), server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready {
		t.Fatal("maintenance should not mark the runtime preview ready")
	}
	if !result.Displayable || !result.Maintenance {
		t.Fatalf("result=%+v, want displayable maintenance page", result)
	}
}

func TestProbePreviewURLResultKeepsPlain503NotDisplayable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	result, err := ProbePreviewURLResult(t.Context(), server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.Displayable || result.Maintenance {
		t.Fatalf("result=%+v, want ordinary 503 to stay behind placeholder", result)
	}
}

func TestProbePreviewURLResultAllowsLocalPhoenixSelfSignedTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		},
	}
	client := &http.Client{Transport: transport}

	result, err := ProbePreviewURLResult(t.Context(), client, "https://lk-test.phoenix.test")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready || !result.Displayable {
		t.Fatalf("result=%+v, want ready displayable preview", result)
	}
}

func fakeFibeCLI(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fibe")
	logPath := filepath.Join(dir, "commands.log")
	stdinPath := filepath.Join(dir, "stdin.json")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + logPath + `"
case "$*" in
  *"greenfield"*)
    echo '{"status":"success","playground":{"id":123},"playspec":{"id":456},"prop":{"id":789},"repo":{"repository_url":"http://gitea.test/owner/repo.git"},"service_urls":[{"name":"app","type":"dynamic","url":"http://lk-test.phoenix.test","visibility":"external"}]}'
    ;;
  *"playgrounds get"*)
    echo '{"id":123,"status":"running"}'
    ;;
  *"playspecs get"*)
    echo '{"id":456,"source_template":{"id":321,"name":"test-app-abc12345"},"source_template_version_id":654,"services":[{"prop_id":789}]}'
    ;;
  *"templates versions list"*)
    echo '{"Data":[{"id":654,"source":{"prop_id":789,"prop_repository_url":"http://gitea.test/owner/repo.git"}}]}'
    ;;
  *"wait playground"*)
    echo '{"status":"running"}'
    ;;
  *"agents send-message"*)
    cat > "` + stdinPath + `"
    echo '{"ok":true}'
    ;;
  *"agents live-state"*)
    echo '{"conversationId":"conv-1","isProcessing":true,"streamText":"[[LIKEABLE_NOTIFICATION_START]]Building[[LIKEABLE_NOTIFICATION_END]]","queuedTurns":1}'
    ;;
  *"agents gitea-token"*)
    echo '{"token":"gitea-token","username":"agent"}'
    ;;
  *"agents create-conversation"*|*"agents start-chat"*|*"agents delete-conversation"*|*"agents interrupt"*|*"agents messages"*|*"agents activity"*|*"playgrounds delete"*|*"playgrounds start"*|*"playgrounds stop"*|*"playgrounds hard-restart"*|*"playspecs delete"*|*"templates versions destroy"*|*"templates delete"*|*"props delete"*)
    echo '{"ok":true,"content":[]}'
    ;;
  *)
    echo "unexpected command: $*" >&2
    exit 64
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return path, logPath, stdinPath
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeTestBMP(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(40 * x), G: uint8(50 * y), B: 180, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := bmp.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTestPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 210, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func writeJSONStatus(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func serverURLFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
