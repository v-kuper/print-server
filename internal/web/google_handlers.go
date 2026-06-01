package web

import (
	"net/http"
	"strings"

	"atol-server/internal/googleintegration"
)

type googleStatusResponse struct {
	OK      bool                      `json:"ok"`
	Message string                    `json:"message,omitempty"`
	Error   string                    `json:"error,omitempty"`
	Status  *googleintegration.Status `json:"status,omitempty"`
}

func (s *Server) handleGoogleStatus(w http.ResponseWriter, r *http.Request) {
	if s.googleClient == nil {
		writeJSON(w, http.StatusOK, googleStatusResponse{
			OK:     true,
			Status: &googleintegration.Status{},
		})
		return
	}
	status := s.googleClient.Status()
	writeJSON(w, http.StatusOK, googleStatusResponse{
		OK:     true,
		Status: &status,
	})
}

func (s *Server) handleGoogleAuthStart(w http.ResponseWriter, r *http.Request) {
	if s.googleClient == nil {
		http.Error(w, "Google integration is not configured", http.StatusServiceUnavailable)
		return
	}
	authURL, err := s.googleClient.AuthURL(s.googleRedirectURI(r), "atol-google")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if s.googleClient == nil {
		http.Error(w, "Google integration is not configured", http.StatusServiceUnavailable)
		return
	}
	if message := strings.TrimSpace(r.URL.Query().Get("error")); message != "" {
		http.Error(w, "Google authorization failed: "+message, http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		http.Error(w, "Google authorization code is missing", http.StatusBadRequest)
		return
	}
	if err := s.googleClient.ExchangeCode(r.Context(), code, s.googleRedirectURI(r)); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><title>Google подключен</title></head><body><h1>Google подключен</h1><p>Можно закрыть эту вкладку и вернуться к ATOL Go Server.</p><p><a href="/">Вернуться в приложение</a></p></body></html>`))
}

func (s *Server) handleGoogleDisconnect(w http.ResponseWriter, r *http.Request) {
	if s.googleClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{
			OK:    false,
			Error: "Google integration is not configured",
		})
		return
	}
	if err := s.googleClient.Disconnect(); err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Google отключен.",
	})
}

func (s *Server) googleRedirectURI(r *http.Request) string {
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	return scheme + "://" + r.Host + "/oauth/google/callback"
}
