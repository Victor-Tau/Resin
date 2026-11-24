package api

import (
	"net/http"

	"github.com/Resinat/Resin/internal/service"
)

func HandleListIPv6Configs(cp *service.ControlPlaneService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		configs, err := cp.ListIPv6Configs()
		if err != nil {
			writeServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, configs)
	}
}

func HandleCreateIPv6Configs(cp *service.ControlPlaneService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var input struct {
			Items []service.IPv6ConfigInput `json:"items"`
		}
		if err := DecodeBody(r, &input); err != nil {
			writeDecodeBodyError(w, err)
			return
		}
		configs, err := cp.CreateIPv6Configs(input.Items)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, configs)
	}
}

func HandleUpdateIPv6Config(cp *service.ControlPlaneService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var input service.IPv6ConfigInput
		if err := DecodeBody(r, &input); err != nil {
			writeDecodeBodyError(w, err)
			return
		}
		config, err := cp.UpdateIPv6Config(PathParam(r, "id"), input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, config)
	}
}

func HandleDeleteIPv6Config(cp *service.ControlPlaneService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := cp.DeleteIPv6Config(PathParam(r, "id")); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
