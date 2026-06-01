package web

import (
	"encoding/json"
	"net/http"

	"atol-server/internal/schedule"
	schedulerruntime "atol-server/internal/scheduler"
)

type schedulerStatusResponse struct {
	OK      bool                     `json:"ok"`
	Message string                   `json:"message,omitempty"`
	Error   string                   `json:"error,omitempty"`
	Status  *schedulerruntime.Status `json:"status,omitempty"`
}

func (s *Server) handleSaveSchedule(w http.ResponseWriter, r *http.Request) {
	var settings schedule.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	if err := s.store.SaveSchedule(settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	if s.scheduler != nil {
		if err := s.scheduler.ResetFromNow(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, statusResponse{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Расписание сохранено.",
	})
}

func (s *Server) handleStopSchedule(w http.ResponseWriter, r *http.Request) {
	settings := schedule.DefaultSettings()
	settings.Enabled = false

	if err := s.store.SaveSchedule(settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	if s.scheduler != nil {
		if err := s.scheduler.ResetFromNow(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, statusResponse{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Расписание остановлено.",
	})
}

func (s *Server) handleSchedulerStatus(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		settings, err := s.store.LoadSchedule()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, schedulerStatusResponse{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
		state, err := s.store.LoadScheduleState()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, schedulerStatusResponse{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
		status := schedulerruntime.Status{
			Settings:      settings.Normalized(),
			LastAttemptAt: state.LastAttemptAt,
			LastSuccessAt: state.LastSuccessAt,
			LastError:     state.LastError,
			NextRunAt:     state.NextRunAt,
		}
		writeJSON(w, http.StatusOK, schedulerStatusResponse{
			OK:     true,
			Status: &status,
		})
		return
	}

	status, err := s.scheduler.Status()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, schedulerStatusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, schedulerStatusResponse{
		OK:     true,
		Status: &status,
	})
}
