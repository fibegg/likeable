package likeable

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type contextKey string

const userContextKey contextKey = "user"

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/api/", s.handleAPI)
	mux.HandleFunc("/", s.handleStatic)
	return requestLog(securityHeaders(s.rateLimit(s.csrfProtect(mux))))
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	checks := map[string]string{"sqlite": "ok", "redis": "disabled"}
	status := http.StatusOK
	if err := s.store.Ping(ctx); err != nil {
		checks["sqlite"] = err.Error()
		status = http.StatusServiceUnavailable
	}
	if s.limiter != nil && s.limiter.redis != nil {
		if err := s.limiter.redis.Ping(ctx).Err(); err != nil {
			checks["redis"] = err.Error()
			status = http.StatusServiceUnavailable
		} else {
			checks["redis"] = "ok"
		}
	}
	writeJSON(w, status, map[string]any{"ok": status == http.StatusOK, "checks": checks})
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) csrfProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !unsafeHTTPMethod(r.Method) || r.URL.Path == "/api/stripe/webhook" {
			next.ServeHTTP(w, r)
			return
		}
		if sameOriginRequest(r, s.config.BaseURL) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusForbidden, "cross-site request rejected")
	})
}

func unsafeHTTPMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func sameOriginRequest(r *http.Request, baseURL string) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		origin = strings.TrimSpace(r.Header.Get("Referer"))
	}
	if origin == "" {
		return true
	}
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(originURL.Scheme, base.Scheme) && strings.EqualFold(originURL.Host, base.Host)
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/me":
		s.handleMe(w, r)
	case r.URL.Path == "/api/bootstrap/config":
		s.handleBootstrapConfig(w, r)
	case r.URL.Path == "/api/auth/google/start":
		s.handleGoogleStart(w, r)
	case r.URL.Path == "/api/auth/google/callback":
		s.handleGoogleCallback(w, r)
	case r.URL.Path == "/api/auth/logout":
		s.withUser(s.handleLogout)(w, r)
	case r.URL.Path == "/api/dev/login":
		s.handleDevLogin(w, r)
	case r.URL.Path == "/api/admin/config":
		s.withAdmin(s.handleAdminConfig)(w, r)
	case r.URL.Path == "/api/admin/recovery":
		s.withAdmin(s.handleAdminRecovery)(w, r)
	case r.URL.Path == "/api/admin/billing/health":
		s.withAdmin(s.handleAdminBillingHealth)(w, r)
	case r.URL.Path == "/api/admin/agent-pool/retire":
		s.withAdmin(s.handleAdminAgentPoolRetire)(w, r)
	case r.URL.Path == "/api/admin/users" || strings.HasPrefix(r.URL.Path, "/api/admin/users/"):
		s.withAdmin(s.handleAdminUsers)(w, r)
	case r.URL.Path == "/api/projects":
		s.withUser(s.handleProjects)(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/projects/"):
		s.withUser(s.handleProjectRoute)(w, r)
	case r.URL.Path == "/api/profile/github/start":
		s.withUser(s.handleGithubStart)(w, r)
	case r.URL.Path == "/api/profile/github/callback":
		s.withUser(s.handleGithubCallback)(w, r)
	case r.URL.Path == "/api/profile/delete-all":
		s.withUser(s.handleProfileDeleteAll)(w, r)
	case r.URL.Path == "/api/profile/archives" || strings.HasPrefix(r.URL.Path, "/api/profile/archives/"):
		s.withUser(s.handleProfileArchives)(w, r)
	case r.URL.Path == "/api/billing/checkout":
		s.withUser(s.handleBillingCheckout)(w, r)
	case r.URL.Path == "/api/billing/refresh":
		s.withUser(s.handleBillingRefresh)(w, r)
	case r.URL.Path == "/api/messages" || strings.HasPrefix(r.URL.Path, "/api/messages/"):
		s.withUser(s.handleUserMessages)(w, r)
	case r.URL.Path == "/api/stripe/webhook":
		s.handleStripeWebhook(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) currentUser(r *http.Request) (*User, error) {
	if user := userFromContext(r.Context()); user != nil {
		return user, nil
	}
	cookie, err := r.Cookie("likeable_session")
	if err != nil || cookie.Value == "" {
		return nil, http.ErrNoCookie
	}
	return s.store.UserBySessionToken(r.Context(), cookie.Value)
}

func (s *Server) withUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.currentUser(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if user.AccessStatus == "restricted" && !restrictedUserAllowedPath(r.URL.Path) && normalizeEmail(user.Email) != s.config.AdminEmail {
			writeError(w, http.StatusForbidden, "account access is restricted")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	}
}

func restrictedUserAllowedPath(path string) bool {
	return path == "/api/auth/logout" || path == "/api/profile/delete-all" || path == "/api/messages" || strings.HasPrefix(path, "/api/messages/")
}

func (s *Server) withAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.withUser(func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r.Context())
		if s.config.AdminEmail == "" || normalizeEmail(user.Email) != s.config.AdminEmail {
			writeError(w, http.StatusForbidden, "admin required")
			return
		}
		next(w, r)
	})
}

func userFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(userContextKey).(*User)
	return user
}
