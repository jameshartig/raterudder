package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jameshartig/autoenergy/pkg/controller"
	"github.com/jameshartig/autoenergy/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandleModeling(t *testing.T) {
	t.Run("Returns 24 SimHours", func(t *testing.T) {
		mockU := &mockUtility{}
		mockU.On("GetCurrentPrice", mock.Anything).Return(types.Price{DollarsPerKWH: 0.10, TSStart: time.Now()}, nil)
		mockU.On("GetFuturePrices", mock.Anything).Return([]types.Price{}, nil)

		mockS := &mockStorage{}
		mockS.On("GetSettings", mock.Anything).Return(types.Settings{
			MinBatterySOC: 5.0,
		}, types.CurrentSettingsVersion, nil)
		mockS.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)

		mockES := &mockESS{}
		mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{
			BatterySOC:         50,
			BatteryCapacityKWH: 10.0,
			Timestamp:          time.Now(),
		}, nil)

		srv := &Server{
			utilityProvider: mockU,
			essSystem:       mockES,
			storage:         mockS,
			controller:      controller.NewController(),
			bypassAuth:      true,
		}

		req := httptest.NewRequest("GET", "/api/modeling", nil)
		w := httptest.NewRecorder()

		srv.handleModeling(w, req)

		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "public, max-age=300", resp.Header.Get("Cache-Control"))

		var hours []controller.SimHour
		err := json.NewDecoder(resp.Body).Decode(&hours)
		require.NoError(t, err)
		assert.Len(t, hours, 24, "should return exactly 24 simulated hours")

		// Verify all expected mocks were called
		mockU.AssertCalled(t, "GetCurrentPrice", mock.Anything)
		mockU.AssertCalled(t, "GetFuturePrices", mock.Anything)
		mockES.AssertCalled(t, "GetStatus", mock.Anything)
		mockS.AssertCalled(t, "GetSettings", mock.Anything)
		mockS.AssertCalled(t, "GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Settings Error Returns 500", func(t *testing.T) {
		mockS := &mockStorage{}
		mockS.On("GetSettings", mock.Anything).Return(types.Settings{}, 0, assert.AnError)

		srv := &Server{
			storage:    mockS,
			bypassAuth: true,
		}

		req := httptest.NewRequest("GET", "/api/modeling", nil)
		w := httptest.NewRecorder()

		srv.handleModeling(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
		assert.Contains(t, w.Body.String(), "failed to get settings")
	})

	t.Run("ESS Status Error Returns 500", func(t *testing.T) {
		mockS := &mockStorage{}
		mockS.On("GetSettings", mock.Anything).Return(types.Settings{}, types.CurrentSettingsVersion, nil)

		mockES := &mockESS{}
		mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{}, assert.AnError)

		srv := &Server{
			storage:    mockS,
			essSystem:  mockES,
			bypassAuth: true,
		}

		req := httptest.NewRequest("GET", "/api/modeling", nil)
		w := httptest.NewRecorder()

		srv.handleModeling(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
		assert.Contains(t, w.Body.String(), "failed to get ess status")
	})

	t.Run("Price Error Returns 500", func(t *testing.T) {
		mockS := &mockStorage{}
		mockS.On("GetSettings", mock.Anything).Return(types.Settings{}, types.CurrentSettingsVersion, nil)

		mockES := &mockESS{}
		mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{Timestamp: time.Now()}, nil)

		mockU := &mockUtility{}
		mockU.On("GetCurrentPrice", mock.Anything).Return(types.Price{}, assert.AnError)

		srv := &Server{
			storage:         mockS,
			essSystem:       mockES,
			utilityProvider: mockU,
			bypassAuth:      true,
		}

		req := httptest.NewRequest("GET", "/api/modeling", nil)
		w := httptest.NewRecorder()

		srv.handleModeling(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
		assert.Contains(t, w.Body.String(), "failed to get price")
	})

	t.Run("No Backfill Called", func(t *testing.T) {
		mockU := &mockUtility{}
		mockU.On("GetCurrentPrice", mock.Anything).Return(types.Price{DollarsPerKWH: 0.10, TSStart: time.Now()}, nil)
		mockU.On("GetFuturePrices", mock.Anything).Return([]types.Price{}, nil)

		mockS := &mockStorage{}
		mockS.On("GetSettings", mock.Anything).Return(types.Settings{
			MinBatterySOC: 5.0,
		}, types.CurrentSettingsVersion, nil)
		mockS.On("GetEnergyHistory", mock.Anything, mock.Anything, mock.Anything).Return([]types.EnergyStats{}, nil)

		mockES := &mockESS{}
		mockES.On("GetStatus", mock.Anything).Return(types.SystemStatus{
			BatterySOC:         80,
			BatteryCapacityKWH: 10.0,
			Timestamp:          time.Now(),
		}, nil)

		srv := &Server{
			utilityProvider: mockU,
			essSystem:       mockES,
			storage:         mockS,
			controller:      controller.NewController(),
			bypassAuth:      true,
		}

		req := httptest.NewRequest("GET", "/api/modeling", nil)
		w := httptest.NewRecorder()

		srv.handleModeling(w, req)

		require.Equal(t, http.StatusOK, w.Result().StatusCode)

		// Verify no backfill-related calls were made
		mockS.AssertNotCalled(t, "GetLatestEnergyHistoryTime")
		mockS.AssertNotCalled(t, "GetLatestPriceHistoryTime")
		mockS.AssertNotCalled(t, "UpsertEnergyHistory")
		mockS.AssertNotCalled(t, "UpsertPrice")
		mockS.AssertNotCalled(t, "InsertAction")
		mockES.AssertNotCalled(t, "GetEnergyHistory")
		mockES.AssertNotCalled(t, "SetModes")
		mockU.AssertNotCalled(t, "GetConfirmedPrices")
	})
}
