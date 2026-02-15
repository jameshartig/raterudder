package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jameshartig/raterudder/pkg/controller"

	"github.com/jameshartig/raterudder/pkg/ess"
	"github.com/jameshartig/raterudder/pkg/types"
	"github.com/jameshartig/raterudder/pkg/utility"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSettings(t *testing.T) {
	mockU := &mockUtility{}
	mockS := &mockStorage{}
	// Default setup for most tests
	mockS.On("GetSettings", mock.Anything, mock.Anything).Return(types.Settings{
		DryRun:          false,
		MinBatterySOC:   10.0,
		UtilityProvider: "comed_hourly",
	}, types.CurrentSettingsVersion, nil)

	// Helper to create server with auth config
	newAuthServer := func(audience string, emails []string, validator TokenValidator) *Server {
		mockES := &mockESS{}
		mockP := ess.NewMap()
		mockP.SetSystem(types.SiteIDNone, mockES)
		// Expect some ESS calls if they happen, e.g. ApplySettings
		mockES.On("ApplySettings", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		mockUMap := utility.NewMap()
		mockUMap.SetProvider("comed_hourly", mockU)

		return &Server{
			utilities:      mockUMap,
			ess:            mockP,
			storage:        mockS,
			controller:     controller.NewController(),
			adminEmails:    emails,
			oidcAudience:   audience,
			tokenValidator: validator,
		}
	}

	// Helper to add user to context
	withUser := func(req *http.Request, email string, isAdmin bool) *http.Request {
		user := types.User{
			ID:    email,
			Email: email,
			Admin: isAdmin,
		}
		ctx := context.WithValue(req.Context(), userContextKey, user)
		ctx = context.WithValue(ctx, siteIDContextKey, types.SiteIDNone)
		return req.WithContext(ctx)
	}

	t.Run("Get Settings", func(t *testing.T) {
		srv := newAuthServer("", nil, nil)
		req := httptest.NewRequest("GET", "/api/settings", nil)
		req = req.WithContext(context.WithValue(req.Context(), siteIDContextKey, types.SiteIDNone))
		w := httptest.NewRecorder()

		srv.handleGetSettings(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		// Check body contains verifiable setting
		assert.Contains(t, w.Body.String(), `"minBatterySOC":10`)
	})

	t.Run("Update Settings - Disabled (No Admin)", func(t *testing.T) {
		srv := newAuthServer("", nil, nil)
		req := httptest.NewRequest("POST", "/api/settings", nil)
		req = withUser(req, "user@example.com", false) // Not admin
		w := httptest.NewRecorder()

		srv.handleUpdateSettings(w, req)
		assert.Equal(t, http.StatusForbidden, w.Result().StatusCode)
	})

	t.Run("Update Settings - Missing Auth", func(t *testing.T) {
		srv := newAuthServer("my-audience", []string{"admin@example.com"}, nil)
		req := httptest.NewRequest("POST", "/api/settings", nil)
		req = req.WithContext(context.WithValue(req.Context(), siteIDContextKey, types.SiteIDNone))
		w := httptest.NewRecorder()

		srv.handleUpdateSettings(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	})

	t.Run("Update Settings - Unauthorized Email", func(t *testing.T) {
		srv := newAuthServer("my-audience", []string{"admin@example.com"}, nil)
		req := httptest.NewRequest("POST", "/api/settings", nil)
		req = withUser(req, "hacker@example.com", false)
		w := httptest.NewRecorder()

		srv.handleUpdateSettings(w, req)
		assert.Equal(t, http.StatusForbidden, w.Result().StatusCode)
	})

	t.Run("Update Settings - Validation Error", func(t *testing.T) {
		srv := newAuthServer("my-audience", []string{"admin@example.com"}, nil)

		// Invalid value (negative battery SOC)
		body := `{"minBatterySOC": -5}`
		req := httptest.NewRequest("POST", "/api/settings", strings.NewReader(body))
		req = withUser(req, "admin@example.com", true)
		w := httptest.NewRecorder()

		srv.handleUpdateSettings(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)

		// Invalid value (IgnoreHourUsageOverMultiple < 1)
		body = `{"ignoreHourUsageOverMultiple": 0}`
		req = httptest.NewRequest("POST", "/api/settings", strings.NewReader(body))
		req = withUser(req, "admin@example.com", true)
		w = httptest.NewRecorder()

		srv.handleUpdateSettings(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	})

	t.Run("Update Settings - Success", func(t *testing.T) {
		srv := newAuthServer("my-audience", []string{"admin@example.com"}, nil)

		body := `{"minBatterySOC": 80, "dryRun": true, "ignoreHourUsageOverMultiple": 5, "solarTrendRatioMax": 3.0, "solarBellCurveMultiplier": 1.0, "utilityProvider": "comed_hourly"}`
		req := httptest.NewRequest("POST", "/api/settings", strings.NewReader(body))
		req = withUser(req, "admin@example.com", true)
		w := httptest.NewRecorder()

		// Expect SetSettings with version
		mockS.On("SetSettings", mock.Anything, mock.Anything, mock.MatchedBy(func(s types.Settings) bool {
			return s.MinBatterySOC == 80.0 && s.DryRun == true
		}), types.CurrentSettingsVersion).Return(nil)

		srv.handleUpdateSettings(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)

		// Verify storage updated
		mockS.AssertExpectations(t)
	})

	t.Run("Auth Status - Is Admin", func(t *testing.T) {
		srv := newAuthServer("my-audience", []string{"admin@example.com"}, nil)
		mockS.On("GetUser", mock.Anything, "admin@example.com").Return(types.User{
			ID:      "admin@example.com",
			SiteIDs: []string{"site1"},
		}, nil).Maybe() // Maybe because logic might vary

		req := httptest.NewRequest("GET", "/api/auth/status", nil)
		req = withUser(req, "admin@example.com", true)
		w := httptest.NewRecorder()

		srv.handleAuthStatus(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		assert.Contains(t, w.Body.String(), `"isAdmin":true`)
		assert.Contains(t, w.Body.String(), `"loggedIn":true`)
	})

	t.Run("Auth Status - Not Admin (Wrong Email)", func(t *testing.T) {
		srv := newAuthServer("my-audience", []string{"admin@example.com"}, nil)

		// Expect GetUser to return valid user
		mockS.On("GetUser", mock.Anything, "user@example.com").Return(types.User{
			ID:      "user@example.com",
			SiteIDs: []string{"site1"},
		}, nil).Maybe()

		req := httptest.NewRequest("GET", "/api/auth/status", nil)
		req = withUser(req, "user@example.com", false)
		w := httptest.NewRecorder()

		srv.handleAuthStatus(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		assert.Contains(t, w.Body.String(), `"isAdmin":false`)
		assert.Contains(t, w.Body.String(), `"loggedIn":true`)
	})

	t.Run("Auth Status - Not Logged In", func(t *testing.T) {
		srv := newAuthServer("my-audience", []string{"admin@example.com"}, nil)

		req := httptest.NewRequest("GET", "/api/auth/status", nil)
		w := httptest.NewRecorder()

		srv.handleAuthStatus(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		assert.Contains(t, w.Body.String(), `"isAdmin":false`)
		assert.Contains(t, w.Body.String(), `"loggedIn":false`)
	})
}
