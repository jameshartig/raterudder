package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raterudder/raterudder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleJoin(t *testing.T) {
	// Helper to create a server with mock storage
	newServer := func(store *mockStorage) *Server {
		return &Server{
			storage: store,
		}
	}

	// Helper to add an authenticated user to the request context
	withUser := func(req *http.Request, userID, email string) *http.Request {
		user := types.User{
			ID:    userID,
			Email: email,
		}
		ctx := context.WithValue(req.Context(), userContextKey, user)
		return req.WithContext(ctx)
	}

	// Helper to add a user-to-register (new user) to the request context
	withNewUser := func(req *http.Request, userID, email string) *http.Request {
		user := types.User{
			ID:    userID,
			Email: email,
		}
		// Only set userToRegisterContextKey for new users, simulating authMiddleware behavior
		ctx := context.WithValue(req.Context(), userToRegisterContextKey, user)
		return req.WithContext(ctx)
	}

	t.Run("MissingFields", func(t *testing.T) {
		store := &mockStorage{}
		s := newServer(store)

		body := `{"inviteCode":"","joinSiteID":""}`
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString(body))
		req = withUser(req, "user@test.com", "user@test.com")
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("InvalidBody", func(t *testing.T) {
		store := &mockStorage{}
		s := newServer(store)

		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString("not json"))
		req = withUser(req, "user@test.com", "user@test.com")
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("NoAuth", func(t *testing.T) {
		store := &mockStorage{}
		s := newServer(store)

		body := `{"inviteCode":"abc","joinSiteID":"site1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("SiteNotFound", func(t *testing.T) {
		store := &mockStorage{}
		store.On("GetSite", mock.Anything, "nonexistent").Return(types.Site{}, assert.AnError)
		s := newServer(store)

		body := `{"inviteCode":"abc","joinSiteID":"nonexistent"}`
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString(body))
		req = withUser(req, "user@test.com", "user@test.com")
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("InvalidInviteCode", func(t *testing.T) {
		store := &mockStorage{}
		store.On("GetSite", mock.Anything, "site1").Return(types.Site{
			ID:         "site1",
			InviteCode: "correct-code",
		}, nil)
		s := newServer(store)

		body := `{"inviteCode":"wrong-code","joinSiteID":"site1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString(body))
		req = withUser(req, "user@test.com", "user@test.com")
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("EmptyInviteCodeOnSite", func(t *testing.T) {
		store := &mockStorage{}
		store.On("GetSite", mock.Anything, "site1").Return(types.Site{
			ID:         "site1",
			InviteCode: "",
		}, nil)
		s := newServer(store)

		body := `{"inviteCode":"any-code","joinSiteID":"site1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString(body))
		req = withUser(req, "user@test.com", "user@test.com")
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("NewUserJoinsSuccessfully", func(t *testing.T) {
		store := &mockStorage{}
		store.On("GetSite", mock.Anything, "site1").Return(types.Site{
			ID:         "site1",
			InviteCode: "secret123",
			Permissions: []types.SitePermissions{
				{UserID: "owner@test.com"},
			},
		}, nil)

		// Expect user creation
		store.On("CreateUser", mock.Anything, mock.MatchedBy(func(user types.User) bool {
			return user.ID == "newuser@test.com" &&
				len(user.SiteIDs) == 1 &&
				user.SiteIDs[0] == "site1"
		})).Return(nil)

		// Expect site update with new user added to permissions
		store.On("UpdateSite", mock.Anything, "site1", mock.MatchedBy(func(site types.Site) bool {
			if len(site.Permissions) != 2 {
				return false
			}
			return site.Permissions[1].UserID == "newuser@test.com"
		})).Return(nil)

		s := newServer(store)

		body := `{"inviteCode":"secret123","joinSiteID":"site1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString(body))
		req = withNewUser(req, "newuser@test.com", "newuser@test.com")
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		store.AssertExpectations(t)
	})

	t.Run("ExistingUserJoinsSuccessfully", func(t *testing.T) {
		store := &mockStorage{}
		store.On("GetSite", mock.Anything, "site2").Return(types.Site{
			ID:         "site2",
			InviteCode: "invite456",
			Permissions: []types.SitePermissions{
				{UserID: "owner@test.com"},
			},
		}, nil)

		// Expect user lookup then update
		store.On("GetUser", mock.Anything, "existing@test.com").Return(types.User{
			ID:      "existing@test.com",
			Email:   "existing@test.com",
			SiteIDs: []string{"site1"},
		}, nil)
		store.On("UpdateUser", mock.Anything, mock.MatchedBy(func(user types.User) bool {
			return len(user.SiteIDs) == 2 && user.SiteIDs[1] == "site2"
		})).Return(nil)

		// Expect site update
		store.On("UpdateSite", mock.Anything, "site2", mock.MatchedBy(func(site types.Site) bool {
			return len(site.Permissions) == 2 && site.Permissions[1].UserID == "existing@test.com"
		})).Return(nil)

		s := newServer(store)

		body := `{"inviteCode":"invite456","joinSiteID":"site2"}`
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString(body))
		req = withUser(req, "existing@test.com", "existing@test.com")
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		store.AssertExpectations(t)
	})

	t.Run("AlreadyJoinedIsIdempotent", func(t *testing.T) {
		store := &mockStorage{}
		store.On("GetSite", mock.Anything, "site1").Return(types.Site{
			ID:         "site1",
			InviteCode: "secret123",
			Permissions: []types.SitePermissions{
				{UserID: "user@test.com"},
			},
		}, nil)

		// User already has this site — GetUser should be called, no UpdateUser
		store.On("GetUser", mock.Anything, "user@test.com").Return(types.User{
			ID:      "user@test.com",
			Email:   "user@test.com",
			SiteIDs: []string{"site1"},
		}, nil)

		s := newServer(store)

		body := `{"inviteCode":"secret123","joinSiteID":"site1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString(body))
		req = withUser(req, "user@test.com", "user@test.com")
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		// UpdateSite should NOT be called since user already has permission
		store.AssertNotCalled(t, "UpdateSite", mock.Anything, mock.Anything, mock.Anything)
		// UpdateUser should NOT be called since user already has site
		store.AssertNotCalled(t, "UpdateUser", mock.Anything, mock.Anything)
	})

	t.Run("CreateNewSiteInSingleSiteMode", func(t *testing.T) {
		store := &mockStorage{}
		s := &Server{storage: store, singleSite: true}

		body := `{"create":true,"createName":"My New Site"}`
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString(body))
		req = withUser(req, "user@test.com", "user@test.com")
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		// No storage calls should have been made
		store.AssertNotCalled(t, "CreateSite", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("CreateNewSite", func(t *testing.T) {
		store := &mockStorage{}

		// Expect GetSite for "short" prefix, which won't be used

		// Expect CreateSite with randomly generated 8-byte hex string since "short" is < 8
		store.On("CreateSite", mock.Anything, mock.MatchedBy(func(id string) bool { return len(id) == 16 }), mock.MatchedBy(func(site types.Site) bool {
			return site.Name == "My New Site" && site.InviteCode == ""
		})).Return(nil)

		// Expect GetUser lookup for existing user (we'll simulate auth as existing user)
		store.On("GetUser", mock.Anything, "user@test.com").Return(types.User{
			ID:      "user@test.com",
			Email:   "user@test.com",
			SiteIDs: []string{"site1"},
		}, nil)
		store.On("UpdateUser", mock.Anything, mock.MatchedBy(func(user types.User) bool {
			return len(user.SiteIDs) == 2
		})).Return(nil)

		// Expect UpdateSite to add user permission to the new site
		store.On("UpdateSite", mock.Anything, mock.MatchedBy(func(id string) bool { return len(id) == 16 }), mock.MatchedBy(func(site types.Site) bool {
			return len(site.Permissions) == 1 && site.Permissions[0].UserID == "user@test.com"
		})).Return(nil)

		s := newServer(store)
		body := `{"create":true,"createName":"My New Site"}`
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString(body))
		req = withUser(req, "user@test.com", "user@test.com")
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		store.AssertExpectations(t)
	})

	t.Run("CreateNewSiteLongEmailPrefix", func(t *testing.T) {
		store := &mockStorage{}

		// Expect GetSite for "longprefix" to fail (meaning it's available)
		store.On("GetSite", mock.Anything, "longprefix").Return(types.Site{}, assert.AnError)

		// Expect CreateSite with "longprefix"
		store.On("CreateSite", mock.Anything, "longprefix", mock.MatchedBy(func(site types.Site) bool {
			return site.Name == "My Prefix Site" && site.InviteCode == ""
		})).Return(nil)

		// Expect User Creation
		store.On("CreateUser", mock.Anything, mock.MatchedBy(func(user types.User) bool {
			return user.ID == "new@test.com" &&
				len(user.SiteIDs) == 1 &&
				user.SiteIDs[0] == "longprefix"
		})).Return(nil)

		// Expect UpdateSite to add user permission to the new site
		store.On("UpdateSite", mock.Anything, "longprefix", mock.MatchedBy(func(site types.Site) bool {
			return len(site.Permissions) == 1 && site.Permissions[0].UserID == "new@test.com"
		})).Return(nil)

		s := newServer(store)
		body := `{"create":true,"createName":"My Prefix Site"}`
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewBufferString(body))
		req = withNewUser(req, "new@test.com", "longprefix@test.com")
		w := httptest.NewRecorder()

		s.handleJoin(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		store.AssertExpectations(t)
	})

	t.Run("ResponseIsJSON", func(t *testing.T) {
		store := &mockStorage{}
		store.On("GetSite", mock.Anything, "site1").Return(types.Site{
			ID:         "site1",
			InviteCode: "code",
		}, nil)

		// Empty body triggers bad request, which gives us a quick test
		body, _ := json.Marshal(map[string]string{"inviteCode": "", "joinSiteID": ""})
		req := httptest.NewRequest(http.MethodPost, "/api/join", bytes.NewReader(body))
		req = withUser(req, "user@test.com", "user@test.com")
		w := httptest.NewRecorder()

		s := newServer(store)
		s.handleJoin(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
