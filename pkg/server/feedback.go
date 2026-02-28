package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/raterudder/raterudder/pkg/log"
	"github.com/raterudder/raterudder/pkg/types"
)

type feedbackRequest struct {
	Sentiment string `json:"sentiment"`
	Comment   string `json:"comment"`
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := s.getUser(r)
	siteID := s.getSiteID(r)

	var req feedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Ctx(ctx).WarnContext(ctx, "failed to decode feedback request", slog.Any("error", err))
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Sentiment == "" {
		writeJSONError(w, "sentiment is required", http.StatusBadRequest)
		return
	}

	feedback := types.Feedback{
		Sentiment: req.Sentiment,
		Comment:   req.Comment,
		SiteID:    siteID,
		UserID:    user.ID,
		Time:      time.Now().UTC(),
	}

	if err := s.storage.InsertFeedback(ctx, feedback); err != nil {
		log.Ctx(ctx).ErrorContext(ctx, "failed to insert feedback", slog.Any("error", err))
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) isAdmin(user types.User) bool {
	for _, adminEmail := range s.adminEmails {
		if user.Email == adminEmail {
			return true
		}
	}
	return false
}
