package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jameshartig/autoenergy/pkg/controller"
	"github.com/jameshartig/autoenergy/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/api/idtoken"
)

func TestHandleUpdate(t *testing.T) {
	// Scenario: High price -> Should Discharge
	mockU := &mockUtility{}
	mockU.On("GetCurrentPrice", mock.Anything).Return(types.Price{DollarsPerKWH: 0.15, TSStart: time.Now()}, nil)
	mockU.On("GetFuturePrices", mock.Anything).Return([]types.Price{}, nil)
	mockU.On("GetConfirmedPrices", mock.Anything, mock.Anything, mock.Anything).Return([]types.Price{}, nil)

	mockS := &mockStorage{}
	mockS.On("GetSettings", mock.Anything).Return(types.Settings{
		DryRun:        true,
		MinBatterySOC: 5.0,
	}, types.CurrentSettingsVersion, nil)
	mockS.On("GetLatestEnergyHistoryTime", mock.Anything).Return(time.Time{}, nil)
	mockS.On("GetLatestPriceHistoryTime", mock.Anything).Return(time.Time{}, nil)
	mockS.On("UpsertEnergyHistory", mock.Anything, mock.Anything).Return(nil)
	mockS.On("UpsertPrice", mock.Anything, mock.Anything).Return(nil)
	mockS.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
	mockS.On("InsertAction", mock.Anything, mock.Anything).Return(nil)

	mockES := &mockESS{}
	mockES.On("ApplySettings", mock.Anything, mock.Anything).Return(nil)
	// Add GetEnergyHistory expectation
	mockES.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
	mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{BatterySOC: 80}, nil)
	// We might need strict matching for SetModes later, but for now:
	mockES.On("SetModes", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	srv := &Server{
		utilityProvider: mockU,
		essSystem:       mockES,
		storage:         mockS,
		listenAddr:      ":8080",
		controller:      controller.NewController(),
		bypassAuth:      true,
	}

	req := httptest.NewRequest("GET", "/api/update", nil)
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
		mockS.On("GetSettings", mock.Anything).Return(types.Settings{DryRun: true}, types.CurrentSettingsVersion, nil)
		mockS.On("GetLatestEnergyHistoryTime", mock.Anything).Return(time.Time{}, nil)
		mockS.On("GetLatestPriceHistoryTime", mock.Anything).Return(time.Time{}, nil)
		mockS.On("UpsertEnergyHistory", mock.Anything, mock.Anything).Return(nil)
		mockS.On("UpsertPrice", mock.Anything, mock.Anything).Return(nil)
		mockS.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
		// InsertAction might not be called if validation fails, so we can't strict expect it or we use .Maybe()
		// But in this test suite we are testing auth failures mostly, so handleUpdate might not reach InsertAction.
		// However, for the success cases it will.
		mockS.On("InsertAction", mock.Anything, mock.Anything).Return(nil).Maybe()

		// Helper to create server with auth config
		newAuthServer := func(audience, email string, adminEmails []string, validator TokenValidator) *Server {
			mockES := &mockESS{}
			mockES.On("ApplySettings", mock.Anything, mock.Anything).Return(nil)
			mockES.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)
			mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{BatterySOC: 50}, nil)
			mockES.On("SetModes", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			return &Server{
				utilityProvider:        mockU,
				essSystem:              mockES,
				storage:                mockS,
				controller:             controller.NewController(),
				updateSpecificAudience: audience,
				oidcAudience:           audience,
				updateSpecificEmail:    email,
				adminEmails:            adminEmails,
				tokenValidator:         validator,
			}
		}

		t.Run("Missing Authorization Header - Specific Email", func(t *testing.T) {
			srv := newAuthServer("my-audience", "check@example.com", nil, nil)
			req := httptest.NewRequest("GET", "/api/update", nil)
			w := httptest.NewRecorder()

			srv.handleUpdate(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
		})

		t.Run("Invalid Authorization Header Format", func(t *testing.T) {
			srv := newAuthServer("my-audience", "check@example.com", nil, nil)
			req := httptest.NewRequest("GET", "/api/update", nil)
			req.Header.Set("Authorization", "Basic user:pass")
			w := httptest.NewRecorder()

			srv.handleUpdate(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
		})

		t.Run("Invalid Token", func(t *testing.T) {
			validator := func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error) {
				return nil, fmt.Errorf("invalid token")
			}
			srv := newAuthServer("my-audience", "check@example.com", nil, validator)
			req := httptest.NewRequest("GET", "/api/update", nil)
			req.Header.Set("Authorization", "Bearer bad-token")
			w := httptest.NewRecorder()

			srv.handleUpdate(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
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

			srv.handleUpdate(w, req)
			assert.Equal(t, http.StatusForbidden, w.Result().StatusCode)
		})

		t.Run("Valid Token, Correct Specific Email", func(t *testing.T) {
			validator := func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Claims: map[string]interface{}{"email": "right@example.com"}}, nil
			}
			srv := newAuthServer("my-audience", "right@example.com", nil, validator)
			req := httptest.NewRequest("GET", "/api/update", nil)
			req.Header.Set("Authorization", "Bearer valid-token")
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

			srv.handleUpdate(w, req)
			assert.Equal(t, http.StatusForbidden, w.Result().StatusCode)
		})

		t.Run("No Auth Configured - Blocked", func(t *testing.T) {
			srv := newAuthServer("my-audience", "", nil, nil)
			req := httptest.NewRequest("GET", "/api/update", nil)
			w := httptest.NewRecorder()

			srv.handleUpdate(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
		})
	})

	t.Run("Paused Updates", func(t *testing.T) {
		mockS := &mockStorage{}
		mockS.On("GetSettings", mock.Anything).Return(types.Settings{
			Pause: true,
		}, types.CurrentSettingsVersion, nil)
		mockS.On("GetLatestEnergyHistoryTime", mock.Anything).Return(time.Time{}, nil)
		mockS.On("GetLatestPriceHistoryTime", mock.Anything).Return(time.Time{}, nil)
		mockS.On("UpsertEnergyHistory", mock.Anything, mock.Anything).Return(nil)
		mockS.On("UpsertPrice", mock.Anything, mock.Anything).Return(nil)

		mockES := &mockESS{}
		mockES.On("ApplySettings", mock.Anything, mock.Anything).Return(nil)
		mockES.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)

		mockU := &mockUtility{}
		mockU.On("GetCurrentPrice", mock.Anything).Return(types.Price{DollarsPerKWH: 0.10, TSStart: time.Now()}, nil)
		mockU.On("GetFuturePrices", mock.Anything).Return([]types.Price{}, nil)
		mockU.On("GetConfirmedPrices", mock.Anything, mock.Anything, mock.Anything).Return([]types.Price{}, nil)

		srv := &Server{
			utilityProvider: mockU,
			essSystem:       mockES,
			storage:         mockS,
			listenAddr:      ":8080",
			controller:      controller.NewController(),
			bypassAuth:      true,
		}

		req := httptest.NewRequest("GET", "/api/update", nil)
		w := httptest.NewRecorder()

		srv.handleUpdate(w, req)

		assert.Equal(t, http.StatusOK, w.Result().StatusCode)

		var resp map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&resp)
		assert.Equal(t, "paused", resp["status"])

		mockES.AssertNotCalled(t, "GetStatus")
		mockES.AssertNotCalled(t, "SetModes")
	})
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
