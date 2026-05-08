package likeable

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	projecttext "github.com/fibegg/likeable/internal/project"
	"golang.org/x/oauth2"
)

var (
	githubRepoNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	urlUserInfoPattern    = regexp.MustCompile(`https?://[^\s/]+@`)
)

func (s *Server) githubOAuthConfig(ctx context.Context) (*oauth2.Config, error) {
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return nil, err
	}
	clientID := strings.TrimSpace(cfg["github_client_id"])
	clientSecret := strings.TrimSpace(cfg["github_client_secret"])
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("GitHub OAuth is not configured")
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  s.config.BaseURL + "/api/profile/github/callback",
		Scopes:       []string{"repo"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}, nil
}

func (s *Server) handleGithubStart(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.githubOAuthConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	state := randomToken()
	http.SetCookie(w, &http.Cookie{Name: "likeable_github_state", Value: state, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isHTTPS(s.config.BaseURL), MaxAge: 600})
	http.Redirect(w, r, cfg.AuthCodeURL(state), http.StatusFound)
}

func (s *Server) handleGithubCallback(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	stateCookie, err := r.Cookie("likeable_github_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	cfg, err := s.githubOAuthConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	token, err := cfg.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "github oauth exchange failed")
		return
	}
	login, err := githubLogin(r.Context(), s.http, token.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	err = s.store.UpsertSocialConnection(r.Context(), SocialConnection{
		UserID:         user.ID,
		Provider:       "github",
		ProviderUserID: login,
		AccessToken:    token.AccessToken,
		Scope:          "repo",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, "/profile", http.StatusFound)
}

func githubLogin(ctx context.Context, client *http.Client, token string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("github profile failed: %s", resp.Status)
	}
	var out struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Login == "" {
		return "", fmt.Errorf("github login missing")
	}
	return out.Login, nil
}

func (s *Server) handleProjectExport(w http.ResponseWriter, r *http.Request, user *User, project *Project) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		RepoName string `json:"repoName"`
		Private  bool   `json:"private"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.RepoName) == "" {
		body.RepoName = projecttext.SourceName(project.Title)
	}
	body.RepoName = strings.TrimSpace(body.RepoName)
	if !githubRepoNamePattern.MatchString(body.RepoName) {
		writeError(w, http.StatusBadRequest, "repoName may only contain letters, numbers, dots, underscores, and hyphens")
		return
	}
	conn, err := s.store.SocialConnection(r.Context(), user.ID, "github")
	if err != nil {
		writeError(w, http.StatusPreconditionRequired, "connect GitHub first")
		return
	}
	jobID, _ := s.store.CreateExportJob(r.Context(), project.ID)
	repoURL, err := s.exportProjectToGithub(r.Context(), user, project, conn, body.RepoName, body.Private)
	if err != nil {
		log.Printf("export project %s to GitHub failed: %v", project.ID, err)
		_ = s.store.FinishExportJob(r.Context(), jobID, "error", "", "Export failed. Try again later.")
		writeError(w, http.StatusBadGateway, "Export failed. Try again later.")
		return
	}
	_ = s.store.FinishExportJob(r.Context(), jobID, "success", repoURL, "")
	s.notifyProjectExportReady(r.Context(), user, project, repoURL)
	writeJSON(w, http.StatusOK, map[string]any{"githubRepoUrl": repoURL, "jobId": jobID})
}

func (s *Server) exportProjectToGithub(ctx context.Context, user *User, project *Project, conn *SocialConnection, repoName string, private bool) (string, error) {
	repoURL, err := createGithubRepo(ctx, s.http, conn.AccessToken, conn.ProviderUserID, repoName, private)
	if err != nil {
		return "", err
	}
	fibe, err := s.fibeClientForProject(ctx, project, user.Email)
	if err != nil {
		return "", err
	}
	giteaToken, err := fibe.GiteaToken(ctx)
	if err != nil {
		return "", err
	}
	temp, err := os.MkdirTemp("", "likeable-export-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	sourceURL := withBasicAuth(project.RepoURL, giteaToken["username"], giteaToken["token"])
	targetURL := withBasicAuth("https://github.com/"+conn.ProviderUserID+"/"+repoName+".git", "x-access-token", conn.AccessToken)
	if err := runGit(ctx, temp, "clone", sourceURL, "."); err != nil {
		return "", err
	}
	if err := runGit(ctx, temp, "remote", "add", "github", targetURL); err != nil {
		return "", err
	}
	if err := runGit(ctx, temp, "push", "github", "HEAD:main", "--force"); err != nil {
		return "", err
	}
	return repoURL, nil
}

func createGithubRepo(ctx context.Context, client *http.Client, token, owner, name string, private bool) (string, error) {
	body := strings.NewReader(fmt.Sprintf(`{"name":%q,"private":%t,"auto_init":false}`, name, private))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.github.com/user/repos", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusUnprocessableEntity {
		return "", fmt.Errorf("github repo create failed: %s", resp.Status)
	}
	var out struct {
		HTMLURL string `json:"html_url"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.HTMLURL != "" {
		return out.HTMLURL, nil
	}
	return "https://github.com/" + owner + "/" + name, nil
}

func withBasicAuth(raw, username, token string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.User = url.UserPassword(username, token)
	return parsed.String()
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = filepath.Clean(dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		safeArgs := redactURLCredentials(strings.Join(args, " "))
		safeOutput := redactURLCredentials(strings.TrimSpace(string(output)))
		return fmt.Errorf("git %s failed: %s", safeArgs, safeOutput)
	}
	return nil
}

func redactURLCredentials(value string) string {
	return urlUserInfoPattern.ReplaceAllStringFunc(value, func(match string) string {
		if strings.HasPrefix(match, "https://") {
			return "https://[redacted]@"
		}
		return "http://[redacted]@"
	})
}
