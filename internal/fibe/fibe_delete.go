package fibe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	projecttext "github.com/fibegg/likeable/internal/project"
)

const (
	resourceDeleteMaxAttempts       = 3
	resourceDeleteRetryDelay        = 5 * time.Second
	platformCodeResourceNotFound    = "RESOURCE_NOT_FOUND"
	platformCodeGone                = "GONE"
	platformCodeConflict            = "CONFLICT"
	platformCodeLocked              = "LOCKED"
	platformCodeRateLimited         = "RATE_LIMITED"
	platformCodeResourceBusy        = "RESOURCE_BUSY"
	platformCodeRemoteRequestFailed = "REMOTE_REQUEST_FAILED"
)

func (c *Client) GiteaToken(ctx context.Context) (map[string]string, error) {
	var raw map[string]any
	if err := c.runCLI(ctx, []string{"agents", "gitea-token", c.agentID}, nil, &raw); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for key, value := range raw {
		out[key] = fmt.Sprint(value)
	}
	return out, nil
}

func (c *Client) DeleteProjectResources(ctx context.Context, project *Project) error {
	var errs []error
	source, sourceErr := c.projectTemplateSource(ctx, project)
	if sourceErr != nil {
		errs = append(errs, fmt.Errorf("inspect project template source: %w", sourceErr))
	}
	if project.PlaygroundID != "" {
		if err := c.deleteFibeResourceWithRetry(ctx, "playgrounds", project.PlaygroundID); err != nil {
			errs = append(errs, fmt.Errorf("delete playground: %w", err))
		}
	}
	if project.PlayspecID != "" {
		if err := c.deleteFibeResourceWithRetry(ctx, "playspecs", project.PlayspecID); err != nil {
			errs = append(errs, fmt.Errorf("delete playspec: %w", err))
		}
	}
	if source.ProjectOwned {
		if source.TemplateID != "" && source.TemplateVersionID != "" {
			if err := c.deleteTemplateVersionWithRetry(ctx, source.TemplateID, source.TemplateVersionID); err != nil {
				errs = append(errs, fmt.Errorf("delete template version: %w", err))
			}
		}
		if source.TemplateID != "" {
			if err := c.deleteFibeResourceWithRetry(ctx, "templates", source.TemplateID); err != nil {
				errs = append(errs, fmt.Errorf("delete template: %w", err))
			}
		}
	}
	for _, propID := range projectPropIDs(project) {
		if err := c.deleteFibeResourceWithRetry(ctx, "props", propID); err != nil {
			errs = append(errs, fmt.Errorf("delete prop %s: %w", propID, err))
		}
	}
	for _, repoURL := range projectRepoURLs(project) {
		if err := c.DeleteGiteaRepo(ctx, repoURL); err != nil {
			errs = append(errs, fmt.Errorf("delete gitea repo %s: %w", repoURL, err))
		}
	}
	if project.ConversationID != "" {
		if err := c.DeleteConversation(ctx, project.ConversationID); err != nil {
			errs = append(errs, fmt.Errorf("delete conversation: %w", err))
		}
	}
	return errors.Join(errs...)
}

type projectTemplateSource struct {
	TemplateID        string
	TemplateVersionID string
	TemplateName      string
	ProjectOwned      bool
}

func (c *Client) projectTemplateSource(ctx context.Context, project *Project) (projectTemplateSource, error) {
	var out projectTemplateSource
	if project == nil || strings.TrimSpace(project.PlayspecID) == "" {
		return out, nil
	}
	var playspec map[string]any
	if err := c.runCLI(ctx, []string{"playspecs", "get", project.PlayspecID}, nil, &playspec); err != nil {
		if resourceAlreadyDeleted(err) {
			return out, nil
		}
		return out, err
	}
	sourceTemplate := anyMap(firstAny(playspec["source_template"], playspec["sourceTemplate"]))
	sourceVersion := anyMap(firstAny(playspec["source_template_version"], playspec["sourceTemplateVersion"]))
	out.TemplateID = numberString(firstAny(sourceTemplate["id"], playspec["source_template_id"], playspec["sourceTemplateID"]))
	out.TemplateVersionID = numberString(firstAny(sourceVersion["id"], playspec["source_template_version_id"], playspec["sourceTemplateVersionID"]))
	out.TemplateName = firstNonEmpty(fmt.Sprint(sourceTemplate["name"]))
	out.ProjectOwned = projectOwnedTemplateName(project, out.TemplateName)
	if !out.ProjectOwned && out.TemplateID != "" && out.TemplateVersionID != "" {
		owned, err := c.projectTemplateVersionOwnedBySource(ctx, out, project)
		if err != nil {
			return out, err
		}
		out.ProjectOwned = owned
	}
	return out, nil
}

func (c *Client) projectTemplateVersionOwnedBySource(ctx context.Context, source projectTemplateSource, project *Project) (bool, error) {
	if project == nil || strings.TrimSpace(source.TemplateID) == "" || strings.TrimSpace(source.TemplateVersionID) == "" {
		return false, nil
	}
	var raw map[string]any
	if err := c.runCLI(ctx, []string{"templates", "versions", "list", source.TemplateID}, nil, &raw); err != nil {
		if resourceAlreadyDeleted(err) {
			return false, nil
		}
		return false, err
	}
	for _, item := range objectSlice(firstAny(raw["Data"], raw["data"], raw["items"], raw["template_versions"], raw["templateVersions"])) {
		if numberString(item["id"]) != source.TemplateVersionID {
			continue
		}
		versionSource := anyMap(item["source"])
		propID := numberString(firstAny(versionSource["prop_id"], versionSource["propID"]))
		repoURL := firstNonEmpty(fmt.Sprint(versionSource["prop_repository_url"]), fmt.Sprint(versionSource["repository_url"]), fmt.Sprint(versionSource["repo_url"]))
		for _, projectPropID := range projectPropIDs(project) {
			if propID != "" && propID == projectPropID {
				return true, nil
			}
		}
		for _, projectRepoURL := range projectRepoURLs(project) {
			if sameNormalizedURL(repoURL, projectRepoURL) {
				return true, nil
			}
		}
	}
	return false, nil
}

func projectPropIDs(project *Project) []string {
	if project == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	add(project.PropID)
	for _, repository := range project.Repositories {
		add(repository.PropID)
	}
	return out
}

func projectRepoURLs(project *Project) []string {
	if project == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[strings.ToLower(value)] {
			return
		}
		seen[strings.ToLower(value)] = true
		out = append(out, value)
	}
	add(project.RepoURL)
	for _, repository := range project.Repositories {
		add(repository.RepoURL)
	}
	return out
}

func projectOwnedTemplateName(project *Project, templateName string) bool {
	if project == nil {
		return false
	}
	templateName = strings.TrimSpace(templateName)
	playgroundName := strings.TrimSpace(project.PlaygroundName)
	if playgroundName != "" && (templateName == playgroundName || strings.HasPrefix(templateName, playgroundName+"-")) {
		return true
	}
	prefix := projecttext.SourceNamePrefix(project.Title)
	return prefix != "" && strings.HasPrefix(templateName, prefix+"-")
}

func sameNormalizedURL(a, b string) bool {
	a = strings.TrimSuffix(strings.TrimSpace(a), ".git")
	b = strings.TrimSuffix(strings.TrimSpace(b), ".git")
	return a != "" && strings.EqualFold(a, b)
}

func (c *Client) deleteTemplateVersionWithRetry(ctx context.Context, templateID, versionID string) error {
	var lastErr error
	for attempt := 0; attempt < resourceDeleteMaxAttempts; attempt++ {
		err := c.runCLI(ctx, []string{"templates", "versions", "destroy", templateID, versionID}, nil, nil)
		if resourceAlreadyDeleted(err) {
			return nil
		}
		if err == nil {
			return nil
		}
		lastErr = err
		if !resourceDeleteRetryable(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(resourceDeleteRetryDelay):
		}
	}
	return lastErr
}

func (c *Client) deleteFibeResourceWithRetry(ctx context.Context, resource, id string) error {
	var lastErr error
	for attempt := 0; attempt < resourceDeleteMaxAttempts; attempt++ {
		if err := c.deleteFibeResource(ctx, resource, id); err != nil {
			lastErr = err
			if !resourceDeleteRetryable(err) {
				return err
			}
		} else {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(resourceDeleteRetryDelay):
		}
	}
	return lastErr
}

func (c *Client) deleteFibeResource(ctx context.Context, resource, id string) error {
	err := c.runCLI(ctx, []string{resource, "delete", id}, nil, nil)
	if resourceAlreadyDeleted(err) {
		return nil
	}
	return err
}

func resourceAlreadyDeleted(err error) bool {
	platformErr := platformError(err)
	return platformErr != nil && (platformErr.Status == http.StatusNotFound || platformErr.Status == http.StatusGone || platformErr.Code == platformCodeResourceNotFound || platformErr.Code == platformCodeGone)
}

func resourceDeleteRetryable(err error) bool {
	if err == nil || resourceAlreadyDeleted(err) {
		return false
	}
	platformErr := platformError(err)
	return platformErr != nil && platformDeleteRetryable(platformErr)
}

func platformError(err error) *PlatformError {
	var platformErr *PlatformError
	if errors.As(err, &platformErr) {
		return platformErr
	}
	return nil
}

func platformDeleteRetryable(err *PlatformError) bool {
	switch err.Status {
	case http.StatusConflict, http.StatusLocked, http.StatusTooEarly, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	switch err.Code {
	case platformCodeConflict, platformCodeLocked, platformCodeRateLimited, platformCodeResourceBusy, platformCodeRemoteRequestFailed:
		return true
	}
	return structuredResourceStateBusy(err.Details)
}

func structuredResourceStateBusy(details map[string]any) bool {
	for _, key := range []string{"status", "state", "current_status"} {
		value, ok := details[key].(string)
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "deleting", "destroying", "in_progress", "in progress", "locked", "pending":
			return true
		}
	}
	return false
}

func (c *Client) DeleteGiteaRepo(ctx context.Context, repoURL string) error {
	owner, repo, baseURL, err := giteaRepoDeleteTarget(repoURL)
	if err != nil {
		return err
	}
	token, err := c.GiteaToken(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+"/api/v1/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token["token"])
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("source cleanup failed: %s", resp.Status)
	}
	return nil
}

func (c *Client) DeleteConversation(ctx context.Context, conversationID string) error {
	err := c.runCLI(ctx, []string{"agents", "delete-conversation", c.agentID, "--conversation-id", conversationID}, nil, nil)
	if resourceAlreadyDeleted(err) {
		return nil
	}
	return err
}

func giteaRepoDeleteTarget(raw string) (string, string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", "", "", errors.New("repo URL must include scheme and host")
	}
	parts := strings.Split(strings.Trim(strings.TrimSuffix(parsed.Path, ".git"), "/"), "/")
	if len(parts) < 2 {
		return "", "", "", errors.New("repo URL must include owner and repo")
	}
	owner := parts[len(parts)-2]
	repo := parts[len(parts)-1]
	return owner, repo, parsed.Scheme + "://" + parsed.Host, nil
}
