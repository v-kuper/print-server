package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"atol-server/internal/articlesummary"
	"atol-server/internal/receiptsnapshot"
)

type snapshotSummaryResponse struct {
	OK          bool       `json:"ok"`
	Message     string     `json:"message,omitempty"`
	Error       string     `json:"error,omitempty"`
	URL         string     `json:"url,omitempty"`
	Title       string     `json:"title,omitempty"`
	Summary     string     `json:"summary,omitempty"`
	Bullets     []string   `json:"bullets,omitempty"`
	Cached      bool       `json:"cached"`
	GeneratedAt *time.Time `json:"generatedAt,omitempty"`
}

func (s *Server) handleReceiptSnapshot(w http.ResponseWriter, r *http.Request) {
	if s.snapshotStore == nil {
		http.NotFound(w, r)
		return
	}
	snapshot, err := s.snapshotStore.Load(r.Context(), r.PathValue("id"))
	if errors.Is(err, receiptsnapshot.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if snapshot.Status == receiptsnapshot.StatusFailed {
		http.Error(w, "receipt snapshot is unavailable: "+snapshot.Error, http.StatusGone)
		return
	}
	html, err := receiptsnapshot.RenderSnapshotHTML(snapshot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(html)
}

func (s *Server) handleReceiptSnapshotSummary(w http.ResponseWriter, r *http.Request) {
	if s.snapshotStore == nil {
		http.NotFound(w, r)
		return
	}
	lineIndex, err := strconv.Atoi(strings.TrimSpace(r.PathValue("lineIndex")))
	if err != nil || lineIndex < 0 {
		writeJSON(w, http.StatusBadRequest, snapshotSummaryResponse{
			OK:    false,
			Error: "invalid snapshot line index",
		})
		return
	}

	snapshot, err := s.snapshotStore.Load(r.Context(), r.PathValue("id"))
	if errors.Is(err, receiptsnapshot.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, snapshotSummaryResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	if snapshot.Status == receiptsnapshot.StatusFailed {
		writeJSON(w, http.StatusGone, snapshotSummaryResponse{
			OK:    false,
			Error: "receipt snapshot is unavailable: " + snapshot.Error,
		})
		return
	}

	lines := receiptsnapshot.ReceiptLinesForSnapshot(snapshot)
	if lineIndex >= len(lines) {
		writeJSON(w, http.StatusBadRequest, snapshotSummaryResponse{
			OK:    false,
			Error: "snapshot line index is out of range",
		})
		return
	}
	lineURL, err := articlesummary.ValidateArticleURL(lines[lineIndex].Link)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, snapshotSummaryResponse{
			OK:    false,
			Error: "snapshot line has no summarizable link",
		})
		return
	}

	cached, err := s.snapshotStore.LoadSummary(r.Context(), snapshot.ID, lineIndex, lineURL)
	if err == nil {
		writeJSON(w, http.StatusOK, snapshotSummaryPayloadFromCache(cached))
		return
	}
	if !errors.Is(err, receiptsnapshot.ErrSummaryNotFound) {
		writeJSON(w, http.StatusInternalServerError, snapshotSummaryResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	if s.articleSummaryProvider == nil {
		writeJSON(w, http.StatusServiceUnavailable, snapshotSummaryResponse{
			OK:    false,
			Error: "article summary provider is not configured",
		})
		return
	}
	settings, err := s.store.LoadMotivation()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, snapshotSummaryResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	result, err := s.articleSummaryProvider.Summarize(r.Context(), settings.Normalized(), lineURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, snapshotSummaryResponse{
			OK:    false,
			Error: err.Error(),
			URL:   lineURL,
		})
		return
	}
	if result.GeneratedAt.IsZero() {
		result.GeneratedAt = s.clock().UTC()
	}
	if result.URL == "" {
		result.URL = lineURL
	}
	input := receiptsnapshot.SummaryInput{
		SnapshotID:  snapshot.ID,
		LineIndex:   lineIndex,
		URL:         lineURL,
		Title:       result.Title,
		Summary:     result.Summary,
		Bullets:     result.Bullets,
		GeneratedAt: result.GeneratedAt,
	}
	if err := s.snapshotStore.SaveSummary(r.Context(), input); err != nil {
		writeJSON(w, http.StatusInternalServerError, snapshotSummaryResponse{
			OK:    false,
			Error: err.Error(),
			URL:   lineURL,
		})
		return
	}

	writeJSON(w, http.StatusOK, snapshotSummaryPayloadFromResult(result, false))
}

func snapshotSummaryPayloadFromCache(summary receiptsnapshot.Summary) snapshotSummaryResponse {
	generatedAt := summary.GeneratedAt
	return snapshotSummaryResponse{
		OK:          true,
		URL:         summary.URL,
		Title:       summary.Title,
		Summary:     summary.Summary,
		Bullets:     summary.Bullets,
		Cached:      true,
		GeneratedAt: &generatedAt,
	}
}

func snapshotSummaryPayloadFromResult(result articlesummary.Result, cached bool) snapshotSummaryResponse {
	generatedAt := result.GeneratedAt
	return snapshotSummaryResponse{
		OK:          true,
		URL:         result.URL,
		Title:       result.Title,
		Summary:     result.Summary,
		Bullets:     result.Bullets,
		Cached:      cached,
		GeneratedAt: &generatedAt,
	}
}
