package fibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	projecttext "github.com/fibegg/likeable/internal/project"
)

type GreenfieldResult struct {
	PlaygroundID        string
	PlaygroundName      string
	PlayspecID          string
	PropID              string
	RepoURL             string
	PreviewURL          string
	SelectedServiceName string
	Repositories        []GreenfieldRepository
	Services            []GreenfieldService
}

type GreenfieldRepository struct {
	Role          string
	PropID        string
	RepoURL       string
	SourceRepoURL string
	Provider      string
	ServiceNames  []string
}

type GreenfieldService struct {
	Name         string
	URL          string
	Type         string
	Visibility   string
	AuthRequired bool
}

func (c *Client) CreateConversation(ctx context.Context, conversationID, title string) error {
	var out map[string]any
	args := []string{"agents", "create-conversation", c.agentID, "--conversation-id", conversationID}
	if strings.TrimSpace(title) != "" {
		args = append(args, "--title", title)
	}
	return c.runCLI(ctx, args, nil, &out)
}

func (c *Client) EnsureConversation(ctx context.Context, conversationID, title string) error {
	err := c.CreateConversation(ctx, conversationID, title)
	if err == nil || IsIdempotentConversationCreateError(err) {
		return nil
	}
	return err
}

func (c *Client) CreateGreenfield(ctx context.Context, project *Project) (*GreenfieldResult, error) {
	name := firstNonEmpty(project.PlaygroundName, projecttext.SourceNameForProject(project))
	serviceSubdomains := projecttext.ServiceSubdomains(project)
	args := c.greenfieldArgs(name, serviceSubdomains)
	for key, value := range greenfieldVariables(project) {
		args = append(args, "--var", key+"="+value)
	}
	var status map[string]any
	if err := c.runCLI(ctx, args, nil, &status); err != nil {
		if filtered, ok := serviceSubdomainsWithoutUnknowns(serviceSubdomains, err); ok {
			args = c.greenfieldArgs(name, filtered)
			for key, value := range greenfieldVariables(project) {
				args = append(args, "--var", key+"="+value)
			}
			status = nil
			if retryErr := c.runCLI(ctx, args, nil, &status); retryErr != nil {
				return c.recoverGreenfieldAfterCreateError(ctx, project, name, retryErr)
			}
		} else {
			return c.recoverGreenfieldAfterCreateError(ctx, project, name, err)
		}
	}
	result := parseGreenfieldStatus(status)
	if result.PlaygroundID == "" {
		return nil, errors.New("workspace creation did not return an id")
	}
	if result.PlaygroundName == "" {
		result.PlaygroundName = name
	}
	if result.PreviewURL == "" || len(result.Repositories) == 0 {
		if recovered, err := c.GreenfieldByPlaygroundID(ctx, result.PlaygroundID); err == nil {
			fillMissingGreenfieldFields(result, recovered)
		}
	}
	if result.PlaygroundName == "" {
		result.PlaygroundName = name
	}
	result.selectPrimary()
	return result, nil
}

func (c *Client) greenfieldArgs(name string, serviceSubdomains map[string]string) []string {
	args := []string{"greenfield", "--name", name, "--git-provider", "gitea", "--private", "--wait-timeout", "10m"}
	if c.marqueeID != "" {
		args = append(args, "--marquee-id", c.marqueeID)
	}
	if c.templateVersionID != "" {
		args = append(args, "--template-version-id", c.templateVersionID)
	}
	for service, subdomain := range serviceSubdomains {
		args = append(args, "--service-subdomain", service+"="+subdomain)
	}
	return args
}

func (c *Client) recoverGreenfieldAfterCreateError(ctx context.Context, project *Project, name string, err error) (*GreenfieldResult, error) {
	if recovered, recoverErr := c.GreenfieldByPlaygroundName(ctx, name); recoverErr == nil && recovered.PlaygroundID != "" {
		return recovered, nil
	}
	if recovered, recoverErr := c.FindGreenfieldBySubdomain(ctx, projecttext.PreviewSubdomain(project)); recoverErr == nil && recovered.PlaygroundID != "" {
		return recovered, nil
	}
	return nil, err
}

func serviceSubdomainsWithoutUnknowns(serviceSubdomains map[string]string, err error) (map[string]string, bool) {
	unknowns := unknownServiceSubdomains(err)
	if len(unknowns) == 0 {
		return nil, false
	}
	unknownSet := make(map[string]bool, len(unknowns))
	for _, service := range unknowns {
		unknownSet[service] = true
	}
	filtered := make(map[string]string, len(serviceSubdomains))
	changed := false
	for service, subdomain := range serviceSubdomains {
		if unknownSet[service] {
			changed = true
			continue
		}
		filtered[service] = subdomain
	}
	return filtered, changed
}

func unknownServiceSubdomains(err error) []string {
	var platformErr *PlatformError
	text := err.Error()
	if errors.As(err, &platformErr) {
		text = strings.Join([]string{platformErr.Message, platformErr.Stderr}, "\n")
	}
	lower := strings.ToLower(text)
	marker := "unknown exposed service(s):"
	idx := strings.Index(lower, marker)
	if idx < 0 {
		return nil
	}
	raw := text[idx+len(marker):]
	if cut := strings.IndexAny(raw, "\n.;"); cut >= 0 {
		raw = raw[:cut]
	}
	parts := strings.Split(raw, ",")
	unknowns := make([]string, 0, len(parts))
	for _, part := range parts {
		service := strings.Trim(strings.TrimSpace(part), `"'[]{}()`)
		if service != "" {
			unknowns = append(unknowns, service)
		}
	}
	return unknowns
}

func (c *Client) FindGreenfieldBySubdomain(ctx context.Context, subdomain string) (*GreenfieldResult, error) {
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return nil, errors.New("workspace subdomain is not available")
	}
	for page := 1; page <= 5; page++ {
		playgrounds, hasMore, err := c.listPlaygrounds(ctx, page, 100)
		if err != nil {
			return nil, err
		}
		for _, playground := range playgrounds {
			id := numberString(playground["id"])
			if id == "" {
				continue
			}
			result, err := c.GreenfieldByPlaygroundID(ctx, id)
			if err != nil {
				continue
			}
			for _, service := range result.Services {
				if routeMatchesSubdomain(service.URL, subdomain) {
					return result, nil
				}
			}
			if routeMatchesSubdomain(result.PreviewURL, subdomain) {
				return result, nil
			}
		}
		if !hasMore {
			break
		}
	}
	return nil, fmt.Errorf("workspace with subdomain %q was not found", subdomain)
}

func (c *Client) GreenfieldByPlaygroundName(ctx context.Context, playgroundName string) (*GreenfieldResult, error) {
	playgroundName = strings.TrimSpace(playgroundName)
	if playgroundName == "" {
		return nil, errors.New("workspace name is not available")
	}
	var playground map[string]any
	if err := c.runCLI(ctx, []string{"playgrounds", "get", playgroundName}, nil, &playground); err != nil {
		return nil, err
	}
	result := greenfieldResultFromPlayground(playground)
	if result.PlaygroundID == "" {
		return nil, fmt.Errorf("workspace %q did not return an id", playgroundName)
	}
	if result.PlaygroundName == "" {
		result.PlaygroundName = playgroundName
	}
	if recovered, err := c.GreenfieldByPlaygroundID(ctx, result.PlaygroundID); err == nil {
		fillMissingGreenfieldFields(result, recovered)
	}
	return result, nil
}

func (c *Client) GreenfieldByPlaygroundID(ctx context.Context, playgroundID string) (*GreenfieldResult, error) {
	playgroundID = strings.TrimSpace(playgroundID)
	if playgroundID == "" {
		return nil, errors.New("workspace id is not available")
	}
	var debug map[string]any
	if err := c.runCLI(ctx, []string{"playgrounds", "debug", playgroundID}, nil, &debug); err != nil {
		return nil, err
	}
	result := greenfieldResultFromDebug(debug)
	if result.PlaygroundID == "" {
		result.PlaygroundID = playgroundID
	}
	if result.PlayspecID != "" {
		_ = c.hydrateGreenfieldSource(ctx, result)
	}
	return result, nil
}

func (c *Client) listPlaygrounds(ctx context.Context, page, perPage int) ([]map[string]any, bool, error) {
	var raw map[string]any
	if err := c.runCLI(ctx, []string{"playgrounds", "list", "--page", strconv.Itoa(page), "--per-page", strconv.Itoa(perPage)}, nil, &raw); err != nil {
		return nil, false, err
	}
	items := objectSlice(firstAny(raw["Data"], raw["data"], raw["items"], raw["playgrounds"]))
	meta := anyMap(firstAny(raw["Meta"], raw["meta"]))
	hasMore := false
	if totalPages := numberInt(firstAny(meta["total_pages"], meta["totalPages"])); totalPages > page {
		hasMore = true
	} else if nextPage := numberInt(firstAny(meta["next_page"], meta["nextPage"])); nextPage > page {
		hasMore = true
	} else if len(items) == perPage {
		hasMore = true
	}
	return items, hasMore, nil
}

func greenfieldResultFromPlayground(playground map[string]any) *GreenfieldResult {
	return &GreenfieldResult{
		PlaygroundID:   numberString(firstAny(playground["id"], playground["ID"])),
		PlaygroundName: firstNonEmpty(fmt.Sprint(firstAny(playground["name"], playground["Name"]))),
		PlayspecID:     numberString(firstAny(playground["playspec_id"], playground["playspecID"], playground["PlayspecID"])),
	}
}

func greenfieldResultFromDebug(debug map[string]any) *GreenfieldResult {
	result := &GreenfieldResult{}
	diagnostics := anyMap(firstAny(debug["diagnostics"], debug["Diagnostics"]))
	playground := anyMap(firstAny(diagnostics["playground"], debug["playground"]))
	result.PlaygroundID = numberString(playground["id"])
	result.PlaygroundName = firstNonEmpty(fmt.Sprint(playground["name"]), fmt.Sprint(diagnostics["name"]), fmt.Sprint(debug["name"]))
	result.PlayspecID = numberString(firstAny(playground["playspec_id"], playground["playspecID"]))

	for _, route := range objectSlice(firstAny(diagnostics["routes"], debug["routes"])) {
		rawURL := routePreviewURL(route)
		if rawURL == "" {
			continue
		}
		result.Services = append(result.Services, GreenfieldService{
			Name:       routeServiceName(route),
			URL:        rawURL,
			Type:       firstNonEmpty(fmt.Sprint(route["type"]), fmt.Sprint(route["service_type"])),
			Visibility: firstNonEmpty(fmt.Sprint(route["visibility"]), fmt.Sprint(route["exposure"])),
		})
	}
	result.selectPrimary()
	return result
}

func (c *Client) hydrateGreenfieldSource(ctx context.Context, result *GreenfieldResult) error {
	if result == nil || result.PlayspecID == "" {
		return nil
	}
	var playspec map[string]any
	if err := c.runCLI(ctx, []string{"playspecs", "get", result.PlayspecID}, nil, &playspec); err != nil {
		return err
	}
	for _, service := range objectSlice(playspec["services"]) {
		name := serviceName(service)
		propID := numberString(firstAny(service["prop_id"], service["propID"]))
		repoURL := firstNonEmpty(fmt.Sprint(service["repo_url"]), fmt.Sprint(service["repository_url"]), fmt.Sprint(service["clone_url"]), fmt.Sprint(service["html_url"]))
		sourceRepoURL := firstNonEmpty(fmt.Sprint(service["source_repo_url"]), fmt.Sprint(service["source_repository_url"]))
		if propID != "" || repoURL != "" {
			result.upsertRepository(GreenfieldRepository{
				Role:          repositoryRole(service, name),
				PropID:        propID,
				RepoURL:       repoURL,
				SourceRepoURL: sourceRepoURL,
				Provider:      firstNonEmpty(fmt.Sprint(service["git_provider"]), fmt.Sprint(service["provider"])),
				ServiceNames:  []string{name},
			})
		}
	}
	result.selectPrimary()
	return nil
}

func fillMissingGreenfieldFields(result, recovered *GreenfieldResult) {
	if result == nil || recovered == nil {
		return
	}
	if result.PlaygroundID == "" {
		result.PlaygroundID = recovered.PlaygroundID
	}
	if result.PlaygroundName == "" {
		result.PlaygroundName = recovered.PlaygroundName
	}
	if result.PlayspecID == "" {
		result.PlayspecID = recovered.PlayspecID
	}
	if result.PropID == "" {
		result.PropID = recovered.PropID
	}
	if result.RepoURL == "" {
		result.RepoURL = recovered.RepoURL
	}
	if result.PreviewURL == "" {
		result.PreviewURL = recovered.PreviewURL
	}
	if result.SelectedServiceName == "" {
		result.SelectedServiceName = recovered.SelectedServiceName
	}
	result.Repositories = mergeGreenfieldRepositories(result.Repositories, recovered.Repositories)
	result.Services = mergeGreenfieldServices(result.Services, recovered.Services)
	result.selectPrimary()
}

func parseGreenfieldStatus(status map[string]any) *GreenfieldResult {
	result := &GreenfieldResult{}
	if pg, ok := status["playground"].(map[string]any); ok {
		result.PlaygroundID = firstNonEmpty(fmt.Sprint(pg["id"]))
		result.PlaygroundName = firstNonEmpty(fmt.Sprint(pg["name"]))
	}
	if result.PlaygroundName == "" {
		result.PlaygroundName = firstNonEmpty(fmt.Sprint(status["name"]))
	}
	if playspec, ok := status["playspec"].(map[string]any); ok {
		result.PlayspecID = firstNonEmpty(fmt.Sprint(playspec["id"]))
	}
	if prop, ok := status["prop"].(map[string]any); ok {
		result.PropID = firstNonEmpty(fmt.Sprint(prop["id"]))
	}
	if result.PropID == "" {
		if props, ok := status["props"].([]any); ok && len(props) > 0 {
			if prop, ok := props[0].(map[string]any); ok {
				result.PropID = firstNonEmpty(fmt.Sprint(prop["id"]))
			}
		}
	}
	if repo, ok := status["repo"].(map[string]any); ok {
		result.RepoURL = firstNonEmpty(fmt.Sprint(repo["repository_url"]), fmt.Sprint(repo["clone_url"]), fmt.Sprint(repo["html_url"]))
	}
	result.Repositories = parseGreenfieldRepositories(status)
	result.Services = parseGreenfieldServices(status["service_urls"])
	if len(result.Repositories) == 0 && (result.PropID != "" || result.RepoURL != "") {
		result.upsertRepository(GreenfieldRepository{
			Role:         "source",
			PropID:       result.PropID,
			RepoURL:      result.RepoURL,
			ServiceNames: serviceNamesForRepo(result.Repositories, result.PropID, result.RepoURL),
		})
	}
	result.selectPrimary()
	return result
}

func parseGreenfieldServices(raw any) []GreenfieldService {
	urls, ok := raw.([]any)
	if !ok {
		return nil
	}
	services := make([]GreenfieldService, 0, len(urls))
	for _, item := range urls {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rawURL := firstNonEmpty(fmt.Sprint(entry["url"]), fmt.Sprint(entry["preview_url"]))
		if rawURL == "" {
			continue
		}
		services = append(services, GreenfieldService{
			Name:         serviceName(entry),
			URL:          browserPreviewURL(rawURL),
			Type:         firstNonEmpty(fmt.Sprint(entry["type"]), fmt.Sprint(entry["service_type"])),
			Visibility:   firstNonEmpty(fmt.Sprint(entry["visibility"]), fmt.Sprint(entry["exposure"])),
			AuthRequired: boolValue(firstAny(entry["auth_required"], entry["authRequired"])),
		})
	}
	return services
}

func parseGreenfieldRepositories(status map[string]any) []GreenfieldRepository {
	var out []GreenfieldRepository
	for _, prop := range objectSlice(status["props"]) {
		out = append(out, repositoryFromMap(prop, ""))
	}
	if prop, ok := status["prop"].(map[string]any); ok {
		out = append(out, repositoryFromMap(prop, ""))
	}
	for _, repo := range objectSlice(status["repos"]) {
		out = append(out, repositoryFromMap(repo, ""))
	}
	if repo, ok := status["repo"].(map[string]any); ok {
		out = append(out, repositoryFromMap(repo, ""))
	}
	return mergeGreenfieldRepositories(nil, out)
}

func repositoryFromMap(raw map[string]any, fallbackName string) GreenfieldRepository {
	name := serviceName(raw)
	if name == "" {
		name = fallbackName
	}
	serviceNames := stringSlice(firstAny(raw["service_names"], raw["serviceNames"]))
	if len(serviceNames) == 0 && name != "" {
		serviceNames = []string{name}
	} else if name == "" && len(serviceNames) == 1 {
		name = serviceNames[0]
	}
	return GreenfieldRepository{
		Role:          repositoryRole(raw, name),
		PropID:        numberString(firstAny(raw["prop_id"], raw["propID"], raw["id"])),
		RepoURL:       firstNonEmpty(fmt.Sprint(raw["repository_url"]), fmt.Sprint(raw["repo_url"]), fmt.Sprint(raw["clone_url"]), fmt.Sprint(raw["html_url"])),
		SourceRepoURL: firstNonEmpty(fmt.Sprint(raw["source_repo_url"]), fmt.Sprint(raw["source_repository_url"])),
		Provider:      firstNonEmpty(fmt.Sprint(raw["git_provider"]), fmt.Sprint(raw["provider"])),
		ServiceNames:  serviceNames,
	}
}

func (result *GreenfieldResult) upsertRepository(repository GreenfieldRepository) {
	result.Repositories = mergeGreenfieldRepositories(result.Repositories, []GreenfieldRepository{repository})
}

func mergeGreenfieldRepositories(primary, secondary []GreenfieldRepository) []GreenfieldRepository {
	out := make([]GreenfieldRepository, 0, len(primary)+len(secondary))
	seenByProp := map[string]int{}
	seenByRepo := map[string]int{}
	add := func(repository GreenfieldRepository) {
		repository.Role = strings.TrimSpace(repository.Role)
		repository.PropID = strings.TrimSpace(repository.PropID)
		repository.RepoURL = strings.TrimSpace(repository.RepoURL)
		repository.SourceRepoURL = strings.TrimSpace(repository.SourceRepoURL)
		repository.ServiceNames = normalizeServiceNames(repository.ServiceNames)
		if repository.PropID == "" && repository.RepoURL == "" {
			return
		}
		repoKey := normalizedRepoKey(repository.RepoURL)
		idx, ok := -1, false
		if repository.PropID != "" {
			idx, ok = seenByProp[repository.PropID]
		}
		if !ok && repoKey != "" {
			idx, ok = seenByRepo[repoKey]
		}
		if ok {
			out[idx].ServiceNames = normalizeServiceNames(append(out[idx].ServiceNames, repository.ServiceNames...))
			if out[idx].Role == "" {
				out[idx].Role = repository.Role
			}
			if out[idx].PropID == "" {
				out[idx].PropID = repository.PropID
			}
			if out[idx].RepoURL == "" {
				out[idx].RepoURL = repository.RepoURL
			}
			if out[idx].SourceRepoURL == "" {
				out[idx].SourceRepoURL = repository.SourceRepoURL
			}
			if out[idx].Provider == "" {
				out[idx].Provider = repository.Provider
			}
			if repository.PropID != "" {
				seenByProp[repository.PropID] = idx
			}
			if repoKey != "" {
				seenByRepo[repoKey] = idx
			}
			return
		}
		if repository.PropID != "" {
			seenByProp[repository.PropID] = len(out)
		}
		if repoKey != "" {
			seenByRepo[repoKey] = len(out)
		}
		out = append(out, repository)
	}
	for _, repository := range primary {
		add(repository)
	}
	for _, repository := range secondary {
		add(repository)
	}
	return out
}

func normalizedRepoKey(rawURL string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(rawURL), ".git"))
}

func mergeGreenfieldServices(primary, secondary []GreenfieldService) []GreenfieldService {
	out := make([]GreenfieldService, 0, len(primary)+len(secondary))
	seen := map[string]int{}
	add := func(service GreenfieldService) {
		service.Name = strings.TrimSpace(service.Name)
		service.URL = strings.TrimSpace(service.URL)
		if service.Name == "" || service.URL == "" {
			return
		}
		key := strings.ToLower(service.Name)
		if idx, ok := seen[key]; ok {
			if out[idx].URL == "" {
				out[idx].URL = service.URL
			}
			if out[idx].Type == "" {
				out[idx].Type = service.Type
			}
			if out[idx].Visibility == "" {
				out[idx].Visibility = service.Visibility
			}
			out[idx].AuthRequired = out[idx].AuthRequired || service.AuthRequired
			return
		}
		seen[key] = len(out)
		out = append(out, service)
	}
	for _, service := range primary {
		add(service)
	}
	for _, service := range secondary {
		add(service)
	}
	return out
}

func (result *GreenfieldResult) selectPrimary() {
	if result == nil {
		return
	}
	if len(result.Services) > 0 {
		bestIndex := 0
		bestScore := -1 << 30
		for idx, service := range result.Services {
			entry := map[string]any{"name": service.Name, "type": service.Type, "visibility": service.Visibility}
			score := serviceURLScore(entry, service.URL)
			if score > bestScore {
				bestScore = score
				bestIndex = idx
			}
		}
		selected := result.Services[bestIndex]
		result.SelectedServiceName = selected.Name
		result.PreviewURL = selected.URL
	}
	if result.PropID == "" || result.RepoURL == "" {
		if repo := result.repositoryForService(result.SelectedServiceName); repo != nil {
			if result.PropID == "" {
				result.PropID = repo.PropID
			}
			if result.RepoURL == "" {
				result.RepoURL = repo.RepoURL
			}
		}
	}
	if result.PropID == "" || result.RepoURL == "" {
		for _, repo := range result.Repositories {
			if result.PropID == "" {
				result.PropID = repo.PropID
			}
			if result.RepoURL == "" {
				result.RepoURL = repo.RepoURL
			}
			if result.PropID != "" && result.RepoURL != "" {
				break
			}
		}
	}
}

func (result *GreenfieldResult) repositoryForService(serviceName string) *GreenfieldRepository {
	serviceName = strings.TrimSpace(serviceName)
	for i := range result.Repositories {
		for _, candidate := range result.Repositories[i].ServiceNames {
			if strings.EqualFold(candidate, serviceName) {
				return &result.Repositories[i]
			}
		}
	}
	return nil
}

func serviceURLScore(entry map[string]any, rawURL string) int {
	name := strings.ToLower(strings.TrimSpace(fmt.Sprint(entry["name"])))
	serviceType := strings.ToLower(strings.TrimSpace(fmt.Sprint(entry["type"])))
	visibility := strings.ToLower(strings.TrimSpace(fmt.Sprint(entry["visibility"])))
	score := 0
	if visibility == "external" || visibility == "public" {
		score += 10
	}
	if serviceType == "dynamic" {
		score += 30
	}
	if name == "app" || name == "web" || name == "frontend" {
		score += 100
	}
	if strings.Contains(name, "ws") || strings.Contains(name, "websocket") || strings.Contains(name, "cable") {
		score -= 100
	}
	if parsed, err := url.Parse(rawURL); err == nil {
		host := strings.ToLower(parsed.Hostname())
		if strings.HasPrefix(host, "ws-") || strings.Contains(host, ".ws-") {
			score -= 80
		}
	}
	return score
}

func greenfieldVariables(project *Project) map[string]string {
	subdomains := projecttext.ServiceSubdomains(project)
	return map[string]string{
		"subdomain":       subdomains["app"],
		"app_subdomain":   subdomains["app"],
		"admin_subdomain": subdomains["admin"],
	}
}

func routePreviewURL(route map[string]any) string {
	rawURL := firstNonEmpty(
		fmt.Sprint(route["url"]),
		fmt.Sprint(route["preview_url"]),
	)
	if rawURL != "" {
		if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
			return browserPreviewURL(rawURL)
		}
		return "https://" + strings.TrimLeft(rawURL, "/")
	}
	host := firstNonEmpty(
		fmt.Sprint(route["traefik_host"]),
		firstStringFromSlice(route["traefik_hosts"]),
		firstStringFromSlice(route["expected_hosts"]),
	)
	if host == "" {
		return ""
	}
	scheme := "https"
	if strings.Contains(host, "localhost") || strings.Contains(host, "127.0.0.1") {
		scheme = "http"
	}
	return scheme + "://" + host
}

func browserPreviewURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "http" || !localPreviewHost(parsed.Hostname()) {
		return rawURL
	}
	parsed.Scheme = "https"
	return parsed.String()
}

func localPreviewHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "phoenix.test" || strings.HasSuffix(host, ".phoenix.test")
}

func routeServiceName(route map[string]any) string {
	return firstNonEmpty(
		fmt.Sprint(route["service_name"]),
		fmt.Sprint(route["serviceName"]),
		fmt.Sprint(route["service"]),
		fmt.Sprint(route["name"]),
		"app",
	)
}

func routeMatchesSubdomain(rawURL, subdomain string) bool {
	rawURL = strings.TrimSpace(rawURL)
	subdomain = strings.TrimSpace(subdomain)
	if rawURL == "" || subdomain == "" {
		return false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == subdomain || strings.HasPrefix(host, subdomain+".")
}

func serviceName(raw map[string]any) string {
	return firstNonEmpty(
		fmt.Sprint(raw["service_name"]),
		fmt.Sprint(raw["serviceName"]),
		fmt.Sprint(raw["service"]),
		fmt.Sprint(raw["name"]),
	)
}

func repositoryRole(raw map[string]any, serviceName string) string {
	role := firstNonEmpty(
		fmt.Sprint(raw["role"]),
		fmt.Sprint(raw["repo_role"]),
		fmt.Sprint(raw["repository_role"]),
	)
	if role != "" {
		return role
	}
	if serviceName != "" {
		return serviceName
	}
	return "source"
}

func serviceNamesForRepo(repositories []GreenfieldRepository, propID, repoURL string) []string {
	for _, repository := range repositories {
		if propID != "" && repository.PropID == propID {
			return repository.ServiceNames
		}
		if sameURL(repository.RepoURL, repoURL) {
			return repository.ServiceNames
		}
	}
	return nil
}

func sameURL(a, b string) bool {
	a = strings.TrimSuffix(strings.TrimSpace(a), ".git")
	b = strings.TrimSuffix(strings.TrimSpace(b), ".git")
	return a != "" && strings.EqualFold(a, b)
}

func normalizeServiceNames(names []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		out = append(out, name)
	}
	return out
}

func stringSlice(raw any) []string {
	switch value := raw.(type) {
	case []string:
		return normalizeServiceNames(value)
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if text := firstNonEmpty(fmt.Sprint(item)); text != "" {
				out = append(out, text)
			}
		}
		return normalizeServiceNames(out)
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		parts := strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
		})
		return normalizeServiceNames(parts)
	default:
		return nil
	}
}

func boolValue(raw any) bool {
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "y":
			return true
		}
	case float64:
		return value != 0
	case int:
		return value != 0
	}
	return false
}

func firstAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func anyMap(raw any) map[string]any {
	if value, ok := raw.(map[string]any); ok {
		return value
	}
	return map[string]any{}
}

func objectSlice(raw any) []map[string]any {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out
}

func firstStringFromSlice(raw any) string {
	values, ok := raw.([]any)
	if !ok {
		return ""
	}
	for _, value := range values {
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func numberString(raw any) string {
	switch value := raw.(type) {
	case int:
		if value > 0 {
			return strconv.Itoa(value)
		}
	case int64:
		if value > 0 {
			return strconv.FormatInt(value, 10)
		}
	case float64:
		if value > 0 {
			return strconv.FormatInt(int64(value), 10)
		}
	case json.Number:
		if text := value.String(); text != "" && text != "0" {
			return text
		}
	default:
		text := firstNonEmpty(fmt.Sprint(raw))
		if text != "" && text != "0" {
			return text
		}
	}
	return ""
}

func numberInt(raw any) int {
	text := numberString(raw)
	if text == "" {
		return 0
	}
	value, _ := strconv.Atoi(text)
	return value
}
