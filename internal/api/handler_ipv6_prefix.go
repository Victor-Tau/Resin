package api

import (
	"net/http"

	"github.com/Resinat/Resin/internal/service"
)

func HandleListIPv6Prefixes(cp *service.ControlPlaneService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefixes, err := cp.ListIPv6Prefixes()
		if err != nil {
			writeServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, prefixes)
	}
}

func HandleSaveIPv6Prefix(cp *service.ControlPlaneService, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input service.IPv6PrefixInput
		if err := DecodeBody(r, &input); err != nil {
			writeDecodeBodyError(w, err)
			return
		}
		prefix, err := cp.SaveIPv6Prefix(PathParam(r, "id"), input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		WriteJSON(w, status, prefix)
	}
}

func HandleDeleteIPv6Prefix(cp *service.ControlPlaneService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := cp.DeleteIPv6Prefix(PathParam(r, "id")); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
