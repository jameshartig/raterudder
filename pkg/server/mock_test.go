package server

import (
	"context"
	"time"

	"github.com/jameshartig/autoenergy/pkg/types"
	"github.com/stretchr/testify/mock"
)

type mockStorage struct {
	mock.Mock
}

func (m *mockStorage) GetSettings(ctx context.Context) (types.Settings, int, error) {
	args := m.Called(ctx)
	// return empty if not specified, or checks args
	if len(args) > 0 {
		return args.Get(0).(types.Settings), args.Int(1), args.Error(2)
	}
	return types.Settings{}, 0, nil
}

func (m *mockStorage) SetSettings(ctx context.Context, settings types.Settings, version int) error {
	args := m.Called(ctx, settings, version)
	return args.Error(0)
}

func (m *mockStorage) UpsertPrice(ctx context.Context, price types.Price, version int) error {
	args := m.Called(ctx, price, version)
	return args.Error(0)
}

func (m *mockStorage) InsertAction(ctx context.Context, action types.Action) error {
	args := m.Called(ctx, action)
	return args.Error(0)
}

func (m *mockStorage) UpsertEnergyHistory(ctx context.Context, stats types.EnergyStats, version int) error {
	args := m.Called(ctx, stats, version)
	return args.Error(0)
}

func (m *mockStorage) GetPriceHistory(ctx context.Context, start, end time.Time) ([]types.Price, error) {
	args := m.Called(ctx, start, end)
	if len(args) > 0 {
		return args.Get(0).([]types.Price), args.Error(1)
	}
	return nil, nil
}

func (m *mockStorage) GetActionHistory(ctx context.Context, start, end time.Time) ([]types.Action, error) {
	args := m.Called(ctx, start, end)
	if len(args) > 0 {
		return args.Get(0).([]types.Action), args.Error(1)
	}
	return nil, nil
}

func (m *mockStorage) GetEnergyHistory(ctx context.Context, start, end time.Time) ([]types.EnergyStats, error) {
	args := m.Called(ctx, start, end)
	if len(args) > 0 {
		return args.Get(0).([]types.EnergyStats), args.Error(1)
	}
	return nil, nil
}

func (m *mockStorage) GetLatestEnergyHistoryTime(ctx context.Context) (time.Time, int, error) {
	args := m.Called(ctx)
	if len(args) > 0 {
		return args.Get(0).(time.Time), args.Int(1), args.Error(2)
	}
	return time.Time{}, 0, nil
}

func (m *mockStorage) GetLatestPriceHistoryTime(ctx context.Context) (time.Time, int, error) {
	args := m.Called(ctx)
	if len(args) > 0 {
		return args.Get(0).(time.Time), args.Int(1), args.Error(2)
	}
	return time.Time{}, 0, nil
}

func (m *mockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

type mockUtility struct {
	mock.Mock
}

func (m *mockUtility) GetCurrentPrice(ctx context.Context) (types.Price, error) {
	args := m.Called(ctx)
	if len(args) > 0 {
		return args.Get(0).(types.Price), args.Error(1)
	}
	return types.Price{}, nil
}
func (m *mockUtility) LastConfirmedPrice(ctx context.Context) (types.Price, error) {
	args := m.Called(ctx)
	if len(args) > 0 {
		return args.Get(0).(types.Price), args.Error(1)
	}
	return types.Price{}, nil
}
func (m *mockUtility) GetFuturePrices(ctx context.Context) ([]types.Price, error) {
	args := m.Called(ctx)
	if len(args) > 0 {
		return args.Get(0).([]types.Price), args.Error(1)
	}
	return nil, nil
}
func (m *mockUtility) GetConfirmedPrices(ctx context.Context, start, end time.Time) ([]types.Price, error) {
	args := m.Called(ctx, start, end)
	if len(args) > 0 {
		return args.Get(0).([]types.Price), args.Error(1)
	}
	return nil, nil
}
func (m *mockUtility) Validate() error {
	args := m.Called()
	return args.Error(0)
}

type mockESS struct {
	mock.Mock
}

func (m *mockESS) GetStatus(ctx context.Context) (types.SystemStatus, error) {
	args := m.Called(ctx)
	if len(args) > 0 {
		return args.Get(0).(types.SystemStatus), args.Error(1)
	}
	return types.SystemStatus{}, nil
}
func (m *mockESS) SetModes(ctx context.Context, bat types.BatteryMode, sol types.SolarMode) error {
	args := m.Called(ctx, bat, sol)
	return args.Error(0)
}
func (m *mockESS) ApplySettings(ctx context.Context, settings types.Settings) error {
	args := m.Called(ctx, settings)
	return args.Error(0)
}
func (m *mockESS) SetPowerControl(ctx context.Context, cfg types.PowerControlConfig) error {
	args := m.Called(ctx, cfg)
	return args.Error(0)
}
func (m *mockESS) GetEnergyHistory(ctx context.Context, start, end time.Time) ([]types.EnergyStats, error) {
	args := m.Called(ctx, start, end)
	if len(args) > 0 {
		return args.Get(0).([]types.EnergyStats), args.Error(1)
	}
	return nil, nil
}
func (m *mockESS) Validate() error {
	args := m.Called()
	return args.Error(0)
}
