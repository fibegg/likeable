package fibe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
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
	if project.RepoURL != "" {
		if err := c.DeleteGiteaRepo(ctx, project.RepoURL); err != nil {
			errs = append(errs, fmt.Errorf("delete gitea repo: %w", err))
		}
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
	if project.PropID != "" {
		if err := c.deleteFibeResourceWithRetry(ctx, "props", project.PropID); err != nil {
			errs = append(errs, fmt.Errorf("delete prop: %w", err))
		}
	}
	if project.ConversationID != "" {
		if err := c.DeleteConversation(ctx, project.ConversationID); err != nil {
			errs = append(errs, fmt.Errorf("delete conversation: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (c *Client) deleteFibeResourceWithRetry(ctx context.Context, resource, id string) error {
	var lastErr error
	for attempt := 0; attempt < 18; attempt++ {
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
		case <-time.After(10 * time.Second):
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
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not found") || strings.Contains(message, "404") || strings.Contains(message, "gone")
}

func resourceDeleteRetryable(err error) bool {
	if err == nil || resourceAlreadyDeleted(err) {
		return false
	}
	message := strings.ToLower(err.Error())
	permanentTokens := []string{
		"executable file not found",
		"forbidden",
		"invalid",
		"no such file or directory",
		"no such host",
		"not configured",
		"unauthorized",
	}
	for _, token := range permanentTokens {
		if strings.Contains(message, token) {
			return false
		}
	}
	retryableTokens := []string{
		"409",
		"423",
		"425",
		"429",
		"500",
		"502",
		"503",
		"504",
		"conflict",
		"deleting",
		"depend",
		"destroying",
		"in progress",
		"in use",
		"in_progress",
		"locked",
		"must be in_progress",
		"pending",
		"referenced",
		"still in use",
	}
	for _, token := range retryableTokens {
		if strings.Contains(message, token) {
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
