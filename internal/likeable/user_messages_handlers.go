package likeable

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func (s *Server) handleUserMessages(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	rest := strings.TrimPrefix(r.URL.Path, "/api/messages")
	if rest == "" || rest == "/" {
		switch r.Method {
		case http.MethodGet:
			notices, err := s.store.NoticesForUser(r.Context(), user.ID, boundedQueryInt(r.URL.Query().Get("limit"), 50, 1, 100))
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"messages": notices})
		case http.MethodPost:
			var body struct {
				Body string `json:"body"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json")
				return
			}
			notice, err := s.store.AddUserNotice(r.Context(), UserNotice{
				UserID:   user.ID,
				Sender:   "user",
				Severity: "info",
				Body:     body.Body,
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			s.sendAdminEmailAsync("Likeable support message from "+user.Email, supportEmailBody(user, notice.Body))
			writeJSON(w, http.StatusCreated, map[string]any{"message": notice})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[1] != "dismiss" || r.Method != http.MethodPost {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	notice, err := s.store.DismissUserNotice(r.Context(), user.ID, parts[0])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": notice})
}
