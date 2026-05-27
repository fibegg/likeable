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
	if status != project.CustomDomainStatus {
		if err := s.store.UpdateProjectDomainStatus(ctx, project.UserID, project.ID, status); err != nil {
			return nil, err
		}
		return s.store.ProjectForUser(ctx, project.UserID, project.ID)
	}
	return project, nil
}

func normalizeDNSHost(value string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
}
