package likeable

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		cfg, _ := s.store.ConfigMap(r.Context())
		writeJSON(w, http.StatusOK, map[string]any{
			"user": nil,
			"auth": map[string]any{
				"googleConfigured": s.googleConfigured(r.Context()),
				"devAuth":          s.config.DevAuth,
				"signupMode":       s.signupMode(cfg),
			},
		})
		return
	}
	notices, _ := s.store.ActiveNoticesForUser(r.Context(), user.ID, 3)
	writeJSON(w, http.StatusOK, map[string]any{
		"user":         user,
		"isAdmin":      s.config.AdminEmail != "" && normalizeEmail(user.Email) == s.config.AdminEmail,
		"messageQuota": s.messageQuota(r.Context(), user),
		"projectQuota": s.projectQuota(r.Context(), user),
		"notices":      notices,
	})
}

func (s *Server) googleConfigured(ctx context.Context) bool {
	cfg, _ := s.store.ConfigMap(ctx)
	clientID := strings.TrimSpace(cfg["google_client_id"])
	clientSecret := strings.TrimSpace(cfg["google_client_secret"])
	return clientID != "" && clientSecret != ""
}

func (s *Server) handleBootstrapConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token := strings.TrimSpace(s.config.BootstrapToken)
	if bootstrapTokenDisabled(token) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !validBearerToken(r.Header.Get("Authorization"), token) {
		writeError(w, http.StatusUnauthorized, "invalid bootstrap token")
		return
	}
	userCount, err := s.store.UserCount(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if userCount > 0 {
		writeError(w, http.StatusConflict, "bootstrap is no longer available")
		return
	}
	if s.googleConfigured(r.Context()) {
		writeError(w, http.StatusConflict, "bootstrap is already configured")
		return
	}
	var body struct {
		GoogleClientID          string `json:"google_client_id"`
		GoogleClientIDCamel     string `json:"googleClientId"`
		GoogleClientSecret      string `json:"google_client_secret"`
		GoogleClientSecretCamel string `json:"googleClientSecret"`
		SignupMode              string `json:"signup_mode"`
		SignupModeCamel         string `json:"signupMode"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	clientID := strings.TrimSpace(firstNonEmpty(body.GoogleClientID, body.GoogleClientIDCamel))
	clientSecret := strings.TrimSpace(firstNonEmpty(body.GoogleClientSecret, body.GoogleClientSecretCamel))
	if clientID == "" || clientSecret == "" {
		writeError(w, http.StatusBadRequest, "google client id and secret are required")
		return
	}
	values := map[string]string{
		"google_client_id":     clientID,
		"google_client_secret": clientSecret,
	}
	if mode := strings.TrimSpace(firstNonEmpty(body.SignupMode, body.SignupModeCamel)); mode != "" {
		values["signup_mode"] = normalizeSignupMode(mode)
	}
	if err := s.store.UpsertConfig(r.Context(), values, secretConfigKeys); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cfg, _ := s.store.ConfigMap(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"googleConfigured": s.googleConfigured(r.Context()),
		"signupMode":       s.signupMode(cfg),
	})
}

func bootstrapTokenDisabled(token string) bool {
	return token == "" || strings.EqualFold(token, "placeholder")
}

func validBearerToken(header, expected string) bool {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return false
	}
	actual := strings.TrimSpace(header[len("bearer "):])
	if len(actual) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func (s *Server) oauthConfig(ctx context.Context) (*oauth2.Config, error) {
	cfg, _ := s.store.ConfigMap(ctx)
	clientID := strings.TrimSpace(cfg["google_client_id"])
	clientSecret := strings.TrimSpace(cfg["google_client_secret"])
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("Google OAuth is not configured")
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  s.config.BaseURL + "/api/auth/google/callback",
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}, nil
}

func (s *Server) canSignInEmail(ctx context.Context, email string) (bool, error) {
	email = normalizeEmail(email)
	if email == "" {
		return false, nil
	}
	if s.config.AdminEmail != "" && email == s.config.AdminEmail {
		return true, nil
	}
	if user, err := s.store.UserByEmail(ctx, email); err == nil {
		return user.AccessStatus != "restricted", nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return false, err
	}
	switch s.signupMode(cfg) {
	case "all":
		return true, nil
	case "allowlist":
		return emailAllowedByList(email, cfg["signup_allowed_emails"]), nil
	default:
		return false, nil
	}
}

func (s *Server) signupMode(cfg map[string]string) string {
	return normalizeSignupMode(firstNonEmpty(cfg["signup_mode"], "forbidden"))
}

func normalizeSignupMode(raw string) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch strings.ReplaceAll(mode, "-", "_") {
	case "all", "allow_all", "open", "enabled":
		return "all"
	case "allowlist", "allow_list", "invite", "invited":
		return "allowlist"
	default:
		return "forbidden"
	}
}

func emailAllowedByList(email, raw string) bool {
	for _, item := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		item = normalizeEmail(item)
		if item == "" {
			continue
		}
		if item == email {
			return true
		}
		if strings.HasPrefix(item, "@") && strings.HasSuffix(email, item) {
			return true
		}
	}
	return false
}

func (s *Server) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.oauthConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	state := randomToken()
	http.SetCookie(w, &http.Cookie{Name: "likeable_oauth_state", Value: state, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isHTTPS(s.config.BaseURL), MaxAge: 600})
	http.Redirect(w, r, cfg.AuthCodeURL(state, oauth2.AccessTypeOnline), http.StatusFound)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("likeable_oauth_state")
	if err != nil || subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(r.URL.Query().Get("state"))) != 1 {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	cfg, err := s.oauthConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	token, err := cfg.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "oauth exchange failed")
		return
	}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := s.http.Do(req)
	if err != nil || resp.StatusCode >= 300 {
		writeError(w, http.StatusBadRequest, "failed to fetch google profile")
		return
	}
	defer resp.Body.Close()
	var profile struct {
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil || !profile.VerifiedEmail {
		writeError(w, http.StatusBadRequest, "google email is not verified")
		return
	}
	if allowed, err := s.canSignInEmail(r.Context(), profile.Email); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	} else if !allowed {
		writeError(w, http.StatusForbidden, "signup is closed for this email")
		return
	}
	user, err := s.store.UpsertUser(r.Context(), profile.Email, profile.Name, profile.Picture)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.setSession(w, r, user.ID)
	s.ensureDefaultProject(r.Context(), user)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	if !s.config.DevAuth {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	email := normalizeEmail(r.URL.Query().Get("email"))
	if email == "" {
		email = "admin@example.com"
	}
	if allowed, err := s.canSignInEmail(r.Context(), email); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	} else if !allowed {
		writeError(w, http.StatusForbidden, "signup is closed for this email")
		return
	}
	user, err := s.store.UpsertUser(r.Context(), email, strings.Split(email, "@")[0], "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.setSession(w, r, user.ID)
	s.ensureDefaultProject(r.Context(), user)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) setSession(w http.ResponseWriter, r *http.Request, userID string) {
	token := randomToken()
	if err := s.store.CreateSession(r.Context(), userID, token, 30*24*time.Hour); err != nil {
		log.Printf("create session: %v", err)
	}
	http.SetCookie(w, &http.Cookie{Name: "likeable_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isHTTPS(s.config.BaseURL), MaxAge: int((30 * 24 * time.Hour).Seconds())})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("likeable_session"); err == nil {
		_ = s.store.DeleteSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "likeable_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
