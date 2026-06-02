package web

import (
	"net/http"

	"atol-server/internal/version"
)

type versionResponse struct {
	OK   bool         `json:"ok"`
	Data version.Info `json:"data"`
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, versionResponse{
		OK:   true,
		Data: s.versionInfo,
	})
}
