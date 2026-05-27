package likeable

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/fibegg/likeable/internal/store"
)

type customDomainResolver interface {
	LookupCNAME(ctx context.Context, host string) (string, error)
}

func (s *Server) customDomainResolver() customDomainResolver {
	if s.domainDNS != nil {
		return s.domainDNS
	}
	return net.DefaultResolver
}

func (s *Server) projectCustomDomainDNSStatus(ctx context.Context, domain, target string) (string, error) {
	domain = normalizeDNSHost(domain)
	target = normalizeDNSHost(target)
	if domain == "" || target == "" {
		return store.ProjectDomainStatusPendingDNS, nil
	}
	cname, err := s.customDomainResolver().LookupCNAME(ctx, domain)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return store.ProjectDomainStatusPendingDNS, nil
		}
		return "", err
	}
	if normalizeDNSHost(cname) == target {
		return store.ProjectDomainStatusDNSVerified, nil
	}
	return store.ProjectDomainStatusPendingDNS, nil
}

func (s *Server) verifyProjectCustomDomain(ctx context.Context, project *Project) (*Project, error) {
	if project == nil || strings.TrimSpace(project.CustomDomain) == "" {
		return project, nil
	}
	status, err := s.projectCustomDomainDNSStatus(ctx, project.CustomDomain, project.CustomDomainTarget)
	if err != nil {
		return nil, err
	}
	if status == store.ProjectDomainStatusDNSVerified && strings.TrimSpace(project.PlaygroundID) != "" {
		if err := s.syncProjectCustomDomainRouting(ctx, project, project.CustomDomain); err != nil {
			return nil, err
		}
		status = store.ProjectDomainStatusActive
	}
	if status != project.CustomDomainStatus {
		if err := s.store.UpdateProjectDomainStatus(ctx, project.UserID, project.ID, status); err != nil {
			return nil, err
		}
		return s.store.ProjectForUser(ctx, project.UserID, project.ID)
	}
	return project, nil
}

func (s *Server) syncProjectCustomDomainRouting(ctx context.Context, project *Project, domain string) error {
	if project == nil || strings.TrimSpace(project.PlaygroundID) == "" {
		return nil
	}
	serviceHosts := projectCustomDomainServiceHosts(project, domain)
	if len(serviceHosts) == 0 {
		return nil
	}
	client, err := s.fibeClientForProject(ctx, project, "")
	if err != nil {
		return err
	}
	return client.UpdatePlaygroundServiceCustomHosts(ctx, project.PlaygroundID, serviceHosts)
}

func projectCustomDomainServiceHosts(project *Project, domain string) map[string][]string {
	if project == nil {
		return nil
	}
	domain = strings.Trim(strings.ToLower(strings.TrimSpace(domain)), ".")
	selected := projectCustomDomainServiceName(project)
	out := map[string][]string{}
	for _, service := range project.Services {
		name := strings.TrimSpace(service.Name)
		if name == "" {
			continue
		}
		if name == selected && domain != "" {
			out[name] = []string{domain}
		} else {
			out[name] = []string{}
		}
	}
	if len(out) == 0 && selected != "" {
		if domain != "" {
			out[selected] = []string{domain}
		} else {
			out[selected] = []string{}
		}
	}
	return out
}

func normalizeDNSHost(value string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
}
