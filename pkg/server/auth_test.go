package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/raterudder/raterudder/pkg/storage"
	"github.com/raterudder/raterudder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/api/idtoken"
)

func TestAuthMiddleware(t *testing.T) {
	// Setup Mocks
	mockStorage := new(mockStorage)

	server := &Server{
		storage:      mockStorage,
		oidcAudience: "test-audience",
		singleSite:   false, // Multi-site mode by default for testing
		tokenValidator: func(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
			if token == "valid-token" || token == "updater-token" {
				email := "user@example.com"
				if token == "updater-token" {
					email = "updater@example.com"
				}
				return &idtoken.Payload{
					Claims:  map[string]interface{}{"email": email},
					Subject: email,
					Expires: time.Now().Add(1 * time.Hour).Unix(),
				}, nil
			}
			return nil, assert.AnError
		},
	}

	// Helper to create request
	createReq := func(method, url string, body interface{}, cookie *http.Cookie) *http.Request {
		var bodyReader *bytes.Buffer
		if body != nil {
			bodyBytes, _ := json.Marshal(body)
			bodyReader = bytes.NewBuffer(bodyBytes)
		} else {
			bodyReader = bytes.NewBuffer(nil)
		}
		req := httptest.NewRequest(method, url, bodyReader)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		return req
	}

	// Helper handler to check context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		siteID, ok := r.Context().Value(siteIDContextKey).(string)
		if ok {
			w.Header().Set("X-Site-ID", siteID)
		}
		user, ok := r.Context().Value(userContextKey).(types.User)
		if ok {
			w.Header().Set("X-Email", user.Email)
			if user.Admin {
				w.Header().Set("X-Admin", "true")
			} else {
				w.Header().Set("X-Admin", "false")
			}
		}
		userReg, ok := r.Context().Value(userToRegisterContextKey).(types.User)
		if ok {
			w.Header().Set("X-Register-Email", userReg.Email)
		}
		w.WriteHeader(http.StatusOK)
	})

	t.Run("Login Bypass", func(t *testing.T) {
		server.singleSite = false
		w := httptest.NewRecorder()
		req := createReq("POST", "/api/auth/login", nil, nil)

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should have empty headers as no auth happened
		assert.Empty(t, w.Header().Get("X-Site-ID"))
		assert.Empty(t, w.Header().Get("X-Email"))
	})

	t.Run("Single Site Mode - No SiteID Required", func(t *testing.T) {
		server.singleSite = true
		w := httptest.NewRecorder()
		// Single site mode now requires auth, so provide a valid cookie
		cookie := &http.Cookie{Name: authTokenCookie, Value: "valid-token"}
		req := createReq("GET", "/api/test", nil, cookie)

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, types.SiteIDNone, w.Header().Get("X-Site-ID"))
	})

	t.Run("Multi Site Mode - No Auth", func(t *testing.T) {
		server.singleSite = false
		w := httptest.NewRecorder()
		req := createReq("GET", "/api/test", nil, nil)

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Multi Site Mode - Auth but No SiteID", func(t *testing.T) {
		server.singleSite = false
		w := httptest.NewRecorder()
		cookie := &http.Cookie{Name: authTokenCookie, Value: "valid-token"}
		req := createReq("GET", "/api/test", nil, cookie)

		// Mock GetUser
		mockStorage.On("GetUser", mock.Anything, "user@example.com").Return(types.User{
			ID:      "user@example.com",
			Email:   "user@example.com",
			SiteIDs: []string{"site1", "site2"},
		}, nil).Once()

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Multi Site Mode - Auth and Valid SiteID (Query Param)", func(t *testing.T) {
		server.singleSite = false
		w := httptest.NewRecorder()
		cookie := &http.Cookie{Name: authTokenCookie, Value: "valid-token"}
		req := createReq("GET", "/api/test?siteID=site1", nil, cookie)

		mockStorage.On("GetUser", mock.Anything, "user@example.com").Return(types.User{
			ID:      "user@example.com",
			Email:   "user@example.com",
			SiteIDs: []string{"site1", "site2"},
			Admin:   false,
		}, nil).Once()

		mockStorage.On("GetSite", mock.Anything, "site1").Return(types.Site{
			ID: "site1",
			Permissions: []types.SitePermissions{
				{UserID: "user@example.com"},
			},
		}, nil).Once()

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "site1", w.Header().Get("X-Site-ID"))
		assert.Equal(t, "user@example.com", w.Header().Get("X-Email"))
	})

	t.Run("Multi Site Mode - Auth and Invalid SiteID", func(t *testing.T) {
		server.singleSite = false
		w := httptest.NewRecorder()
		cookie := &http.Cookie{Name: authTokenCookie, Value: "valid-token"}
		req := createReq("GET", "/api/test?siteID=site3", nil, cookie)

		mockStorage.On("GetUser", mock.Anything, "user@example.com").Return(types.User{
			ID:      "user@example.com",
			Email:   "user@example.com",
			SiteIDs: []string{"site1", "site2"},
		}, nil).Once()

		// Permission check fails
		mockStorage.On("GetSite", mock.Anything, "site3").Return(types.Site{
			ID:          "site3",
			Permissions: []types.SitePermissions{},
		}, nil).Once()

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Multi Site Mode - Auth as Admin bypasses permissions as read-only", func(t *testing.T) {
		server.singleSite = false
		server.adminEmails = []string{"user@example.com"}
		defer func() { server.adminEmails = nil }() // reset

		w := httptest.NewRecorder()
		cookie := &http.Cookie{Name: authTokenCookie, Value: "valid-token"}
		req := createReq("GET", "/api/test?siteID=site3", nil, cookie)

		mockStorage.On("GetUser", mock.Anything, "user@example.com").Return(types.User{
			ID:      "user@example.com",
			Email:   "user@example.com",
			SiteIDs: []string{"site1", "site2"},
			Admin:   false,
		}, nil).Once()

		// Permission check typically fails because they aren't explicit here
		mockStorage.On("GetSite", mock.Anything, "site3").Return(types.Site{
			ID:          "site3",
			Permissions: []types.SitePermissions{},
		}, nil).Once()

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "user@example.com", w.Header().Get("X-Email"))
		assert.Equal(t, "false", w.Header().Get("X-Admin"))
	})

	t.Run("Multi Site Mode - Auth and POST Body SiteID", func(t *testing.T) {
		server.singleSite = false
		w := httptest.NewRecorder()
		cookie := &http.Cookie{Name: authTokenCookie, Value: "valid-token"}
		req := createReq("POST", "/api/test", map[string]string{"siteID": "site2"}, cookie)

		mockStorage.On("GetUser", mock.Anything, "user@example.com").Return(types.User{
			ID:      "user@example.com",
			Email:   "user@example.com",
			SiteIDs: []string{"site1", "site2"},
		}, nil).Once()

		mockStorage.On("GetSite", mock.Anything, "site2").Return(types.Site{
			ID: "site2",
			Permissions: []types.SitePermissions{
				{UserID: "user@example.com"},
			},
		}, nil).Once()

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "site2", w.Header().Get("X-Site-ID"))
	})

	t.Run("Multi Site Mode - Default SiteID if One", func(t *testing.T) {
		server.singleSite = false
		w := httptest.NewRecorder()
		cookie := &http.Cookie{Name: authTokenCookie, Value: "valid-token"}
		req := createReq("GET", "/api/test", nil, cookie)

		mockStorage.On("GetUser", mock.Anything, "user@example.com").Return(types.User{
			ID:      "user@example.com",
			Email:   "user@example.com",
			SiteIDs: []string{"site1"},
		}, nil).Once()

		mockStorage.On("GetSite", mock.Anything, "site1").Return(types.Site{
			ID: "site1",
			Permissions: []types.SitePermissions{
				{UserID: "user@example.com"},
			},
		}, nil).Once()

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "site1", w.Header().Get("X-Site-ID"))
	})

	t.Run("Multi Site Mode - Logout No SiteID", func(t *testing.T) {
		server.singleSite = false
		w := httptest.NewRecorder()
		cookie := &http.Cookie{Name: authTokenCookie, Value: "valid-token"}
		req := createReq("POST", "/api/auth/logout", nil, cookie)

		mockStorage.On("GetUser", mock.Anything, "user@example.com").Return(types.User{
			ID:      "user@example.com",
			Email:   "user@example.com",
			SiteIDs: []string{"site1", "site2"},
		}, nil).Once()

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Update Specific Auth", func(t *testing.T) {
		// Test special update logic
		server.singleSite = false
		server.updateSpecificEmail = "updater@example.com"

		// Mock token validator for updater
		originalValidator := server.tokenValidator
		server.tokenValidator = func(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
			if token == "updater-token" {
				return &idtoken.Payload{
					Claims:  map[string]interface{}{"email": "updater@example.com"},
					Subject: "updater@example.com",
					Expires: time.Now().Add(1 * time.Hour).Unix(),
				}, nil
			}
			return originalValidator(ctx, token, audience)
		}
		defer func() { server.tokenValidator = originalValidator }()

		w := httptest.NewRecorder()
		req := createReq("POST", "/api/update", map[string]string{"siteID": "site1"}, nil)
		req.Header.Set("Authorization", "Bearer updater-token")

		// Update specific should NOT call GetUser even if user doesn't exist in DB logic (bypassed)
		// but we still need GetSite to verify site exists

		mockStorage.On("GetSite", mock.Anything, "site1").Return(types.Site{
			ID: "site1",
			Permissions: []types.SitePermissions{
				{UserID: "updater@example.com"},
			},
		}, nil).Once()

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Empty(t, w.Header().Get("X-Email"))
	})

	t.Run("Multi Site Mode - Join with New User", func(t *testing.T) {
		server.singleSite = false
		w := httptest.NewRecorder()
		cookie := &http.Cookie{Name: authTokenCookie, Value: "valid-token"}
		req := createReq("POST", "/api/join", map[string]string{"inviteCode": "abc", "joinSiteID": "site1"}, cookie)

		// Mock GetUser returning ErrUserNotFound
		mockStorage.On("GetUser", mock.Anything, "user@example.com").Return(types.User{}, storage.ErrUserNotFound).Once()

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should NOT have user header because userContextKey wasn't set
		assert.Empty(t, w.Header().Get("X-Email"))
		// Should have register email header
		assert.Equal(t, "user@example.com", w.Header().Get("X-Register-Email"))
	})

	t.Run("Multi Site Mode - Auth Status with Unregistered User", func(t *testing.T) {
		server.singleSite = false
		w := httptest.NewRecorder()
		cookie := &http.Cookie{Name: authTokenCookie, Value: "valid-token"}
		req := createReq("GET", "/api/auth/status", nil, cookie)

		// Mock GetUser returning ErrUserNotFound
		mockStorage.On("GetUser", mock.Anything, "user@example.com").Return(types.User{}, storage.ErrUserNotFound).Once()

		server.authMiddleware(testHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should NOT have user header because userContextKey wasn't set
		assert.Empty(t, w.Header().Get("X-Email"))
		// Should have register email header
		assert.Equal(t, "user@example.com", w.Header().Get("X-Register-Email"))
	})
}

func TestHandleAuthStatus(t *testing.T) {
	server := &Server{
		oidcAudience: "test-audience",
	}

	t.Run("Unregistered User", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/auth/status", nil)
		userToRegister := types.User{Email: "new@example.com", ID: "123"}
		ctx := context.WithValue(req.Context(), userToRegisterContextKey, userToRegister)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		server.handleAuthStatus(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp authStatusResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)

		assert.True(t, resp.LoggedIn)
		assert.Equal(t, "new@example.com", resp.Email)
		assert.Empty(t, resp.SiteIDs)
	})

	t.Run("Registered User", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/auth/status", nil)
		user := types.User{Email: "existing@example.com", ID: "456", SiteIDs: []string{"site1"}}
		ctx := context.WithValue(req.Context(), userContextKey, user)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		server.handleAuthStatus(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp authStatusResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)

		assert.True(t, resp.LoggedIn)
		assert.Equal(t, "existing@example.com", resp.Email)
		assert.Equal(t, []string{"site1"}, resp.SiteIDs)
	})
}

func TestHandleLogin(t *testing.T) {
	// Setup Mocks
	mockStorage := new(mockStorage)

	server := &Server{
		storage:      mockStorage,
		oidcAudience: "test-audience",
		singleSite:   false,
		tokenValidator: func(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
			if token == "valid-token" {
				return &idtoken.Payload{
					Claims:  map[string]interface{}{"email": "user@example.com"},
					Subject: "user-subject",
					Expires: time.Now().Add(1 * time.Hour).Unix(),
				}, nil
			} else if token == "no-email-token" {
				return &idtoken.Payload{
					Claims:  map[string]interface{}{},
					Subject: "user-subject",
					Expires: time.Now().Add(1 * time.Hour).Unix(),
				}, nil
			}
			return nil, assert.AnError
		},
	}

	createReq := func(token string) *http.Request {
		body := map[string]string{"token": token}
		bodyBytes, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(bodyBytes))
		return r
	}

	t.Run("Valid Login", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := createReq("valid-token")

		server.handleLogin(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusOK, result.StatusCode)

		cookies := result.Cookies()
		found := false
		for _, c := range cookies {
			if c.Name == authTokenCookie {
				found = true
				assert.Equal(t, "valid-token", c.Value)
				assert.True(t, c.HttpOnly)
				assert.True(t, c.Secure)
				assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
				// Check expiry is roughly correct (within an hour)
				if !c.Expires.IsZero() {
					assert.WithinDuration(t, time.Now().Add(1*time.Hour), c.Expires, 10*time.Second)
				}
			}
		}
		assert.True(t, found, "auth cookie should be set")
	})

	t.Run("Invalid Token", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := createReq("invalid-token")

		server.handleLogin(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Token Missing Email", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := createReq("no-email-token")

		server.handleLogin(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Invalid Request Body", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString("invalid-json"))

		server.handleLogin(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleLogout(t *testing.T) {
	server := &Server{}

	t.Run("Logout", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		// Set a cookie to be cleared
		req.AddCookie(&http.Cookie{
			Name:  authTokenCookie,
			Value: "some-token",
		})

		server.handleLogout(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusOK, result.StatusCode)

		cookies := result.Cookies()
		found := false
		for _, c := range cookies {
			if c.Name == authTokenCookie {
				found = true
				assert.Equal(t, "", c.Value)
				assert.True(t, c.MaxAge < 0)
				assert.True(t, c.Expires.Before(time.Now()))
			}
		}
		assert.True(t, found, "auth cookie should be cleared")
	})
}
