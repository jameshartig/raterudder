package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jameshartig/raterudder/pkg/controller"
	"github.com/jameshartig/raterudder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/idtoken"

	"github.com/jameshartig/raterudder/pkg/ess"
	"github.com/jameshartig/raterudder/pkg/utility"
)

func TestHandleUpdate(t *testing.T) {
	// Scenario: High price -> Should Discharge
	mockU := &mockUtility{}
	mockU.On("GetCurrentPrice", mock.Anything).Return(types.Price{DollarsPerKWH: 0.15, TSStart: time.Now()}, nil)
	mockU.On("GetFuturePrices", mock.Anything).Return([]types.Price{}, nil)
	mockU.On("GetConfirmedPrices", mock.Anything, mock.Anything, mock.Anything).Return([]types.Price{}, nil)

	mockS := &mockStorage{}
	mockS.On("GetSettings", mock.Anything, mock.Anything).Return(types.Settings{
		DryRun:          true,
		MinBatterySOC:   5.0,
		UtilityProvider: "comed_hourly",
	}, types.CurrentSettingsVersion, nil)
	mockS.On("GetLatestEnergyHistoryTime", mock.Anything, mock.Anything).Return(time.Time{}, 0, nil)
	mockS.On("GetLatestPriceHistoryTime", mock.Anything, mock.Anything).Return(time.Time{}, 0, nil)
	mockS.On("UpsertEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockS.On("UpsertPrice", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockS.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
	mockS.On("InsertAction", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockES := &mockESS{}
	mockES.On("ApplySettings", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Add GetEnergyHistory expectation
	mockES.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
	mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{BatterySOC: 80}, nil)
	// We might need strict matching for SetModes later, but for now:
	mockES.On("SetModes", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockP := ess.NewMap()
	mockP.SetSystem(types.SiteIDNone, mockES)

	mockUMap := utility.NewMap()
	mockUMap.SetProvider("comed_hourly", mockU)

	srv := &Server{
		utilities:  mockUMap,
		ess:        mockP,
		storage:    mockS,
		listenAddr: ":8080",
		controller: controller.NewController(),
		bypassAuth: true,
	}

	req := httptest.NewRequest("GET", "/api/update", nil)
	req = req.WithContext(context.WithValue(req.Context(), siteIDContextKey, types.SiteIDNone))
	w := httptest.NewRecorder()

	srv.handleUpdate(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	t.Run("Handle Update - Auth", func(t *testing.T) {
		mockU := &mockUtility{}
		mockU.On("GetCurrentPrice", mock.Anything).Return(types.Price{DollarsPerKWH: 0.10, TSStart: time.Now()}, nil)
		mockU.On("GetFuturePrices", mock.Anything).Return([]types.Price{}, nil)
		mockU.On("GetConfirmedPrices", mock.Anything, mock.Anything, mock.Anything).Return([]types.Price{}, nil)

		mockS := &mockStorage{}
		mockS.On("GetSettings", mock.Anything, mock.Anything).Return(types.Settings{DryRun: true, UtilityProvider: "comed_hourly"}, types.CurrentSettingsVersion, nil)
		mockS.On("GetLatestEnergyHistoryTime", mock.Anything, mock.Anything).Return(time.Time{}, 0, nil)
		mockS.On("GetLatestPriceHistoryTime", mock.Anything, mock.Anything).Return(time.Time{}, 0, nil)
		mockS.On("UpsertEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockS.On("UpsertPrice", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockS.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
		// InsertAction might not be called if validation fails, so we can't strict expect it or we use .Maybe()
		// But in this test suite we are testing auth failures mostly, so handleUpdate might not reach InsertAction.
		// However, for the success cases it will.
		mockS.On("InsertAction", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		// Mock GetUser for notadmin check
		mockS.On("GetUser", mock.Anything, "notadmin@example.com").Return(types.User{}, fmt.Errorf("user not found"))

		// Helper to create server with auth config
		newAuthServer := func(audience, email string, adminEmails []string, validator TokenValidator) *Server {
			mockES := &mockESS{}
			mockES.On("ApplySettings", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mockES.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
			mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{BatterySOC: 50}, nil)
			mockES.On("SetModes", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			mockP := ess.NewMap()
			mockP.SetSystem(types.SiteIDNone, mockES)

			mockUMap := utility.NewMap()
			mockUMap.SetProvider("comed_hourly", mockU)

			return &Server{
				utilities:              mockUMap,
				ess:                    mockP,
				storage:                mockS,
				controller:             controller.NewController(),
				updateSpecificAudience: audience,
				oidcAudience:           audience,
				updateSpecificEmail:    email,
				adminEmails:            adminEmails,
				tokenValidator:         validator,
				singleSite:             true,
			}
		}

		t.Run("Missing Authorization Header - Specific Email", func(t *testing.T) {
			srv := newAuthServer("my-audience", "check@example.com", nil, nil)
			req := httptest.NewRequest("GET", "/api/update", nil)
			w := httptest.NewRecorder()

			handler := srv.authMiddleware(http.HandlerFunc(srv.handleUpdate))
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
		})

		t.Run("Invalid Authorization Header Format", func(t *testing.T) {
			srv := newAuthServer("my-audience", "check@example.com", nil, nil)
			req := httptest.NewRequest("GET", "/api/update", nil)
			req.Header.Set("Authorization", "Basic user:pass")
			w := httptest.NewRecorder()

			handler := srv.authMiddleware(http.HandlerFunc(srv.handleUpdate))
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
		})

		t.Run("Invalid Token", func(t *testing.T) {
			validator := func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error) {
				return nil, fmt.Errorf("invalid token")
			}
			srv := newAuthServer("my-audience", "check@example.com", nil, validator)
			req := httptest.NewRequest("GET", "/api/update", nil)
			req.Header.Set("Authorization", "Bearer bad-token")
			w := httptest.NewRecorder()

			handler := srv.authMiddleware(http.HandlerFunc(srv.handleUpdate))
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
		})

		t.Run("Admin Email Fallback - Valid", func(t *testing.T) {
			validator := func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error) {
				assert.Equal(t, "valid-token", idToken)
				assert.Equal(t, "my-audience", audience)
				return &idtoken.Payload{Claims: map[string]interface{}{"email": "admin@example.com"}}, nil
			}
			srv := newAuthServer("my-audience", "", []string{"admin@example.com"}, validator)
			req := httptest.NewRequest("GET", "/api/update", nil)
			req.Header.Set("Authorization", "Bearer valid-token")
			req = req.WithContext(context.WithValue(req.Context(), siteIDContextKey, types.SiteIDNone))
			w := httptest.NewRecorder()

			srv.handleUpdate(w, req)
			assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		})

		t.Run("Valid Token, Specific Email Wrong", func(t *testing.T) {
			validator := func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Claims: map[string]interface{}{"email": "wrong@example.com"}}, nil
			}
			srv := newAuthServer("my-audience", "right@example.com", nil, validator)
			req := httptest.NewRequest("GET", "/api/update", nil)
			req.Header.Set("Authorization", "Bearer valid-token")
			w := httptest.NewRecorder()

			handler := srv.authMiddleware(http.HandlerFunc(srv.handleUpdate))
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
		})

		t.Run("Valid Token, Correct Specific Email", func(t *testing.T) {
			validator := func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Claims: map[string]interface{}{"email": "right@example.com"}}, nil
			}
			srv := newAuthServer("my-audience", "right@example.com", nil, validator)
			req := httptest.NewRequest("GET", "/api/update", nil)
			req.Header.Set("Authorization", "Bearer valid-token")
			req = req.WithContext(context.WithValue(req.Context(), siteIDContextKey, types.SiteIDNone))
			w := httptest.NewRecorder()

			srv.handleUpdate(w, req)
			assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		})
		t.Run("Admin Email Fallback - Invalid", func(t *testing.T) {
			validator := func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Claims: map[string]interface{}{"email": "notadmin@example.com"}}, nil
			}
			srv := newAuthServer("my-audience", "", []string{"admin@example.com"}, validator)
			req := httptest.NewRequest("GET", "/api/update", nil)
			req.Header.Set("Authorization", "Bearer valid-token")
			w := httptest.NewRecorder()

			handler := srv.authMiddleware(http.HandlerFunc(srv.handleUpdate))
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
		})

		t.Run("No Auth Configured - Blocked", func(t *testing.T) {
			srv := newAuthServer("my-audience", "", nil, nil)
			req := httptest.NewRequest("GET", "/api/update", nil)
			w := httptest.NewRecorder()

			handler := srv.authMiddleware(http.HandlerFunc(srv.handleUpdate))
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
		})
	})

	t.Run("Paused Updates", func(t *testing.T) {
		mockS := &mockStorage{}
		mockS.On("GetSettings", mock.Anything, mock.Anything).Return(types.Settings{
			Pause:           true,
			UtilityProvider: "comed_hourly",
		}, types.CurrentSettingsVersion, nil)
		mockS.On("GetLatestEnergyHistoryTime", mock.Anything, mock.Anything).Return(time.Time{}, 0, nil)
		mockS.On("GetLatestPriceHistoryTime", mock.Anything, mock.Anything).Return(time.Time{}, 0, nil)
		mockS.On("UpsertEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockS.On("UpsertPrice", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		mockES := &mockESS{}
		mockES.On("ApplySettings", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockES.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)

		mockP := ess.NewMap()
		mockP.SetSystem(types.SiteIDNone, mockES)

		mockU := &mockUtility{}
		mockU.On("GetCurrentPrice", mock.Anything).Return(types.Price{DollarsPerKWH: 0.10, TSStart: time.Now()}, nil)
		mockU.On("GetFuturePrices", mock.Anything).Return([]types.Price{}, nil)
		mockU.On("GetConfirmedPrices", mock.Anything, mock.Anything, mock.Anything).Return([]types.Price{}, nil)

		mockUMap := utility.NewMap()
		mockUMap.SetProvider("comed_hourly", mockU)

		srv := &Server{
			utilities:  mockUMap,
			ess:        mockP,
			storage:    mockS,
			listenAddr: ":8080",
			controller: controller.NewController(),
			bypassAuth: true,
		}

		req := httptest.NewRequest("GET", "/api/update", nil)
		req = req.WithContext(context.WithValue(req.Context(), siteIDContextKey, types.SiteIDNone))
		w := httptest.NewRecorder()

		srv.handleUpdate(w, req)

		assert.Equal(t, http.StatusOK, w.Result().StatusCode)

		var resp map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&resp)
		assert.Equal(t, "paused", resp["status"])

		mockES.AssertNotCalled(t, "GetStatus")
		mockES.AssertNotCalled(t, "SetModes")
	})

	t.Run("Action - Emergency Mode", func(t *testing.T) {
		mockS := &mockStorage{}
		mockS.On("GetSettings", mock.Anything, mock.Anything).Return(types.Settings{UtilityProvider: "comed_hourly"}, types.CurrentSettingsVersion, nil)
		mockS.On("GetLatestEnergyHistoryTime", mock.Anything, mock.Anything).Return(time.Time{}, 0, nil)
		mockS.On("GetLatestPriceHistoryTime", mock.Anything, mock.Anything).Return(time.Time{}, 0, nil)
		mockS.On("UpsertEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockS.On("UpsertPrice", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		mockES := &mockESS{}
		mockES.On("ApplySettings", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockES.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
		mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{EmergencyMode: true}, nil)

		mockP := ess.NewMap()
		mockP.SetSystem(types.SiteIDNone, mockES)

		// Expect InsertAction with specific description
		mockS.On("InsertAction", mock.Anything, mock.Anything, mock.MatchedBy(func(a types.Action) bool {
			return a.Description == "In emergency mode" && a.Fault
		})).Return(nil)

		mockU := &mockUtility{}
		mockU.On("GetConfirmedPrices", mock.Anything, mock.Anything, mock.Anything).Return([]types.Price{}, nil)

		mockUMap := utility.NewMap()
		mockUMap.SetProvider("comed_hourly", mockU)

		srv := &Server{
			utilities:  mockUMap,
			ess:        mockP,
			storage:    mockS,
			listenAddr: ":8080",
			controller: controller.NewController(),
			bypassAuth: true,
		}

		req := httptest.NewRequest("GET", "/api/update", nil)
		req = req.WithContext(context.WithValue(req.Context(), siteIDContextKey, types.SiteIDNone))
		w := httptest.NewRecorder()
		srv.handleUpdate(w, req)

		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		var resp map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&resp)
		assert.Equal(t, "emergency mode", resp["status"])

		mockU.AssertNotCalled(t, "GetCurrentPrice")
		mockES.AssertNotCalled(t, "SetModes")
	})

	t.Run("Action - Alarms Present", func(t *testing.T) {
		mockS := &mockStorage{}
		mockS.On("GetSettings", mock.Anything, mock.Anything).Return(types.Settings{UtilityProvider: "comed_hourly"}, types.CurrentSettingsVersion, nil)
		mockS.On("GetLatestEnergyHistoryTime", mock.Anything, mock.Anything).Return(time.Time{}, 0, nil)
		mockS.On("GetLatestPriceHistoryTime", mock.Anything, mock.Anything).Return(time.Time{}, 0, nil)
		mockS.On("UpsertEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockS.On("UpsertPrice", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		mockES := &mockESS{}
		mockES.On("ApplySettings", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockES.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
		mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{
			Alarms: []types.SystemAlarm{{Name: "Test Alarm"}},
		}, nil)

		mockP := ess.NewMap()
		mockP.SetSystem(types.SiteIDNone, mockES)

		// Expect InsertAction with specific description
		mockS.On("InsertAction", mock.Anything, mock.Anything, mock.MatchedBy(func(a types.Action) bool {
			return a.Description == "1 alarms present" && a.Fault
		})).Return(nil)

		mockU := &mockUtility{}
		mockU.On("GetConfirmedPrices", mock.Anything, mock.Anything, mock.Anything).Return([]types.Price{}, nil)

		mockUMap := utility.NewMap()
		mockUMap.SetProvider("comed_hourly", mockU)

		srv := &Server{
			utilities:  mockUMap,
			ess:        mockP,
			storage:    mockS,
			listenAddr: ":8080",
			controller: controller.NewController(),
			bypassAuth: true,
		}

		req := httptest.NewRequest("GET", "/api/update", nil)
		req = req.WithContext(context.WithValue(req.Context(), siteIDContextKey, types.SiteIDNone))
		w := httptest.NewRecorder()
		srv.handleUpdate(w, req)

		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		var resp map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&resp)
		assert.Equal(t, "alarms present", resp["status"])

		mockU.AssertNotCalled(t, "GetCurrentPrice")
		mockES.AssertNotCalled(t, "SetModes")
	})

	t.Run("Handle Update - Backfill Logic", func(t *testing.T) {
		t.Run("Version Mismatch - Backfill Triggered", func(t *testing.T) {
			mockS := &mockStorage{}
			// Set up old version
			lastTime := time.Date(2023, 10, 27, 12, 0, 0, 0, time.UTC)
			mockS.On("GetLatestEnergyHistoryTime", mock.Anything, mock.Anything).Return(lastTime, 0, nil) // Version 0 < CurrentVersion
			mockS.On("GetLatestPriceHistoryTime", mock.Anything, mock.Anything).Return(lastTime, 0, nil)

			// Expect backfill from 5 days ago, not lastTime
			// We can verify this by checking the start time in GetEnergyHistory call to ESS
			// But for now, let's just ensure it calls GetEnergyHistory
			mockES := &mockESS{}
			mockES.On("ApplySettings", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{BatterySOC: 50}, nil)
			mockES.On("SetModes", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			// Capture arguments to Verify start time
			var startTimes []time.Time
			mockES.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				startTimes = append(startTimes, args.Get(1).(time.Time))
			}).Return([]types.EnergyStats{}, nil).Maybe()

			mockP := ess.NewMap()
			mockP.SetSystem(types.SiteIDNone, mockES)

			mockU := &mockUtility{}
			mockU.On("GetCurrentPrice", mock.Anything).Return(types.Price{}, nil)
			mockU.On("GetFuturePrices", mock.Anything).Return([]types.Price{}, nil)
			mockU.On("GetConfirmedPrices", mock.Anything, mock.Anything, mock.Anything).Return([]types.Price{}, nil)

			mockUMap := utility.NewMap()
			mockUMap.SetProvider("comed_hourly", mockU)

			// Other storage expectations
			mockS.On("GetSettings", mock.Anything, mock.Anything).Return(types.Settings{UtilityProvider: "comed_hourly"}, types.CurrentSettingsVersion, nil)
			mockS.On("UpsertEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mockS.On("UpsertPrice", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mockS.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
			mockS.On("InsertAction", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			srv := &Server{
				utilities:  mockUMap,
				ess:        mockP,
				storage:    mockS,
				listenAddr: ":8080",
				controller: controller.NewController(),
				bypassAuth: true,
			}
			req := httptest.NewRequest("GET", "/api/update", nil)
			req = req.WithContext(context.WithValue(req.Context(), siteIDContextKey, types.SiteIDNone))
			w := httptest.NewRecorder()
			srv.handleUpdate(w, req)
			assert.Equal(t, http.StatusOK, w.Result().StatusCode)

			// Verify that at least one call started from ~5 days ago (midnight)
			require.NotEmpty(t, startTimes, "GetEnergyHistory should have been called")
			earliest := startTimes[0]
			for _, t := range startTimes {
				if t.Before(earliest) {
					earliest = t
				}
			}
			now := time.Now()
			fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
			expected := time.Date(fiveDaysAgo.Year(), fiveDaysAgo.Month(), fiveDaysAgo.Day(), 0, 0, 0, 0, fiveDaysAgo.Location())
			assert.Equal(t, expected, earliest, "Backfill should start from midnight 5 days ago")
		})

		t.Run("Version Match - No Backfill", func(t *testing.T) {
			mockS := &mockStorage{}
			// Set up current version
			// Make lastTime recent enough that it would normally just resume from there
			lastTime := time.Now().Add(-2 * time.Hour).Truncate(time.Hour)
			mockS.On("GetLatestEnergyHistoryTime", mock.Anything, mock.Anything).Return(lastTime, types.CurrentEnergyStatsVersion, nil)
			mockS.On("GetLatestPriceHistoryTime", mock.Anything, mock.Anything).Return(lastTime, types.CurrentPriceHistoryVersion, nil)

			mockES := &mockESS{}
			mockES.On("ApplySettings", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{BatterySOC: 50}, nil)
			mockES.On("SetModes", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			// Capture arguments to Verify start time
			mockES.On("GetEnergyHistory", mock.Anything, mock.MatchedBy(func(start time.Time) bool {
				// Should be equal to lastTime (or close to it due to truncation logic)
				return start.Equal(lastTime)
			}), mock.Anything).Return([]types.EnergyStats{}, nil).Maybe()

			mockP := ess.NewMap()
			mockP.SetSystem(types.SiteIDNone, mockES)

			mockU := &mockUtility{}
			mockU.On("GetCurrentPrice", mock.Anything).Return(types.Price{}, nil)
			mockU.On("GetFuturePrices", mock.Anything).Return([]types.Price{}, nil)
			mockU.On("GetConfirmedPrices", mock.Anything, mock.Anything, mock.Anything).Return([]types.Price{}, nil)

			mockUMap := utility.NewMap()
			mockUMap.SetProvider("comed_hourly", mockU)

			// Other storage expectations
			mockS.On("GetSettings", mock.Anything, mock.Anything).Return(types.Settings{UtilityProvider: "comed_hourly"}, types.CurrentSettingsVersion, nil)
			mockS.On("UpsertEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mockS.On("UpsertPrice", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mockS.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
			mockS.On("InsertAction", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			srv := &Server{
				utilities:  mockUMap,
				ess:        mockP,
				storage:    mockS,
				listenAddr: ":8080",
				controller: controller.NewController(),
				bypassAuth: true,
			}
			req := httptest.NewRequest("GET", "/api/update", nil)
			req = req.WithContext(context.WithValue(req.Context(), siteIDContextKey, types.SiteIDNone))
			w := httptest.NewRecorder()
			srv.handleUpdate(w, req)
			assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		})
	})
}

func TestHandleUpdateSites(t *testing.T) {
	mockU := &mockUtility{}
	// Expect GetCurrentPrice to be called just ONCE due to caching, even if multiple sites use it
	mockU.On("GetCurrentPrice", mock.Anything).Return(types.Price{DollarsPerKWH: 0.15, TSStart: time.Now()}, nil)
	mockU.On("GetFuturePrices", mock.Anything).Return([]types.Price{}, nil)
	mockU.On("GetConfirmedPrices", mock.Anything, mock.Anything, mock.Anything).Return([]types.Price{}, nil)

	mockS := &mockStorage{}
	mockS.On("ListSites", mock.Anything).Return([]types.Site{
		{ID: "site1"},
		{ID: "site2"},
	}, nil)

	// Expect GetSettings for both sites
	mockS.On("GetSettings", mock.Anything, "site1").Return(types.Settings{UtilityProvider: "comed_hourly"}, types.CurrentSettingsVersion, nil)
	mockS.On("GetSettings", mock.Anything, "site2").Return(types.Settings{UtilityProvider: "comed_hourly"}, types.CurrentSettingsVersion, nil)

	// Other storage calls for both sites
	mockS.On("GetLatestEnergyHistoryTime", mock.Anything, mock.Anything).Return(time.Time{}, 0, nil)
	// Return recent time for PriceHistory to limit backfill to 1 call (for current partial day)
	mockS.On("GetLatestPriceHistoryTime", mock.Anything, mock.Anything).Return(time.Now().Add(-1*time.Hour), types.CurrentPriceHistoryVersion, nil)
	mockS.On("UpsertEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockS.On("UpsertPrice", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockS.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
	mockS.On("InsertAction", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockES := &mockESS{}
	// Expect calls for both sites
	mockES.On("ApplySettings", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockES.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
	mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{BatterySOC: 80}, nil)
	mockES.On("SetModes", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockP := ess.NewMap()
	mockP.SetSystem("site1", mockES)
	mockP.SetSystem("site2", mockES)

	mockUMap := utility.NewMap()
	mockUMap.SetProvider("comed_hourly", mockU)

	srv := &Server{
		utilities:  mockUMap,
		ess:        mockP,
		storage:    mockS,
		listenAddr: ":8080",
		controller: controller.NewController(),
		bypassAuth: true,
	}

	req := httptest.NewRequest("POST", "/api/updateSites", nil)
	w := httptest.NewRecorder()

	srv.handleUpdateSites(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var results map[string]string
	err := json.NewDecoder(w.Body).Decode(&results)
	require.NoError(t, err)

	assert.Equal(t, "success", results["site1"])
	assert.Equal(t, "success", results["site2"])

	// Verify caching: GetCurrentPrice should be called exactly once
	mockU.AssertNumberOfCalls(t, "GetCurrentPrice", 1)
	// Verify caching: GetConfirmedPrices should be called exactly once per provider (since both sites use "comed_hourly")
	mockU.AssertNumberOfCalls(t, "GetConfirmedPrices", 1)
	// Verify caching: GetLatestPriceHistoryTime should be called exactly once per provider
	mockS.AssertNumberOfCalls(t, "GetLatestPriceHistoryTime", 1)
}

// Helpers for Recording Mocks
type RecordingMockESS struct {
	mockESS
	status        types.SystemStatus
	setModes      bool
	setBatMode    types.BatteryMode
	setSolMode    types.SolarMode
	GetStatusFunc func(ctx context.Context) (types.SystemStatus, error)
	SetModesFunc  func(ctx context.Context, bat types.BatteryMode, sol types.SolarMode) error
}

func (m *RecordingMockESS) GetStatus(ctx context.Context) (types.SystemStatus, error) {
	if m.GetStatusFunc != nil {
		return m.GetStatusFunc(ctx)
	}
	return m.status, nil
}

func (m *RecordingMockESS) SetModes(ctx context.Context, bat types.BatteryMode, sol types.SolarMode) error {
	if m.SetModesFunc != nil {
		return m.SetModesFunc(ctx, bat, sol)
	}
	m.setModes = true
	m.setBatMode = bat
	m.setSolMode = sol
	return nil
}

type RecordingMockStorage struct {
	mockStorage
	insertedAction   *types.Action
	InsertActionFunc func(ctx context.Context, action types.Action) error
}

func (m *RecordingMockStorage) InsertAction(ctx context.Context, action types.Action) error {
	if m.InsertActionFunc != nil {
		return m.InsertActionFunc(ctx, action)
	}
	m.insertedAction = &action
	return nil
}
