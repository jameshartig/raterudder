package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleListESS(w http.ResponseWriter, r *http.Request) {
	systems := s.ess.ListSystems()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(systems); err != nil {
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
}
