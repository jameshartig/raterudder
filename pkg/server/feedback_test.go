package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raterudder/raterudder/pkg/storage/storagemock"
	"github.com/raterudder/raterudder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleFeedback(t *testing.T) {
	s := &Server{
		adminEmails: []string{"admin@test.com"},
		storage:     &storagemock.MockDatabase{},
	}
	db := s.storage.(*storagemock.MockDatabase)

	t.Run("ValidRequest", func(t *testing.T) {
		reqBody := feedbackRequest{
			Sentiment: "happy",
			Comment:   "Love it",
			SiteID:    "site1",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/feedback", bytes.NewReader(body))

		// Create user with site
		user := types.User{
			ID:    "user1",
			Email: "user@test.com",
			Sites: []types.UserSite{{ID: "site1"}},
		}
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		req = req.WithContext(context.WithValue(req.Context(), allUserSitesContextKey, user.Sites))

		db.On("InsertFeedback", mock.Anything, mock.MatchedBy(func(f types.Feedback) bool {
			return f.Sentiment == "happy" && f.Comment == "Love it" && f.SiteID == "site1" && f.UserID == "user1"
		})).Return(nil).Once()

		rr := httptest.NewRecorder()
		s.handleFeedback(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		db.AssertExpectations(t)
	})

	t.Run("UnauthorizedSite", func(t *testing.T) {
		reqBody := feedbackRequest{
			Sentiment: "happy",
			Comment:   "Love it",
			SiteID:    "site2",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/feedback", bytes.NewReader(body))

		user := types.User{
			ID:    "user1",
			Email: "user@test.com",
			Sites: []types.UserSite{{ID: "site1"}}, // Does not have site2
		}
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		req = req.WithContext(context.WithValue(req.Context(), allUserSitesContextKey, user.Sites))

		rr := httptest.NewRecorder()
		s.handleFeedback(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("AdminCanSubmitAnySite", func(t *testing.T) {
		reqBody := feedbackRequest{
			Sentiment: "happy",
			Comment:   "Love it",
			SiteID:    "site2",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/feedback", bytes.NewReader(body))

		user := types.User{
			ID:    "admin1",
			Email: "admin@test.com", // matches admin email
			Sites: []types.UserSite{{ID: "site1"}},
		}
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		req = req.WithContext(context.WithValue(req.Context(), allUserSitesContextKey, user.Sites))

		db.On("InsertFeedback", mock.Anything, mock.MatchedBy(func(f types.Feedback) bool {
			return f.Sentiment == "happy" && f.Comment == "Love it" && f.SiteID == "site2" && f.UserID == "admin1"
		})).Return(nil).Once()

		rr := httptest.NewRecorder()
		s.handleFeedback(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		db.AssertExpectations(t)
	})
}
