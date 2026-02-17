import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { Router } from 'wouter';
import Forecast from './Forecast';
import { fetchModeling } from '../api';
import type { ModelingHour } from '../api';

// Mock the API
vi.mock('../api', () => ({
    fetchModeling: vi.fn(),
    fetchActions: vi.fn().mockResolvedValue([]),
    fetchSavings: vi.fn().mockResolvedValue({}),
    fetchAuthStatus: vi.fn().mockResolvedValue({ loggedIn: false, authRequired: false, clientID: '' }),
    fetchSettings: vi.fn().mockResolvedValue({}),
    updateSettings: vi.fn(),
    login: vi.fn(),
    logout: vi.fn(),
    BatteryMode: { NoChange: 0, Standby: 1, ChargeAny: 2, ChargeSolar: 3, Load: -1 },
    SolarMode: { NoChange: 0, NoExport: 1, Any: 2 },
}));

// Mock ResizeObserver for recharts
class ResizeObserverMock {
    observe() {}
    unobserve() {}
    disconnect() {}
}
(globalThis as any).ResizeObserver = ResizeObserverMock;

function makeSimHours(): ModelingHour[] {
    const hours: ModelingHour[] = [];
    const base = new Date('2026-02-11T14:00:00Z');
    for (let i = 0; i < 24; i++) {
        const ts = new Date(base);
        ts.setHours(ts.getHours() + i);
        hours.push({
            ts: ts.toISOString(),
            hour: ts.getHours(),
            netLoadSolarKWH: 1.0 - i * 0.05,
            gridChargeDollarsPerKWH: 0.10 + i * 0.005,
            solarOppDollarsPerKWH: 0.08,
            avgHomeLoadKWH: 1.5,
            predictedSolarKWH: Math.max(0, 3.0 * Math.sin((i / 24) * Math.PI)),
            batteryKWH: 5.0 - i * 0.2,
            batteryCapacityKWH: 10.0,
            batteryReserveKWH: 0.5,
            todaySolarTrend: 1.0,
        });
    }
    return hours;
}

const renderForecast = () => render(<Router><Forecast /></Router>);

describe('Forecast Page', () => {
    beforeEach(() => {
        vi.resetAllMocks();
    });

    it('shows loading state initially', () => {
        (fetchModeling as any).mockReturnValue(new Promise(() => {}));
        renderForecast();
        expect(screen.getByText(/Loading simulation/)).toBeInTheDocument();
    });

    it('calls fetchModeling and renders 4 charts', async () => {
        const data = makeSimHours();
        (fetchModeling as any).mockResolvedValue(data);

        renderForecast();

        await waitFor(() => {
            expect(fetchModeling).toHaveBeenCalledTimes(1);
        });

        await waitFor(() => {
            expect(screen.getByText('Battery (%)')).toBeInTheDocument();
            expect(screen.getByText('Predicted Solar (kWh)')).toBeInTheDocument();
            expect(screen.getByText('Avg Home Load (kWh)')).toBeInTheDocument();
            expect(screen.getByText('Grid Charge Cost ($/kWh)')).toBeInTheDocument();
        });
    });

    it('shows page heading and subtitle', async () => {
        (fetchModeling as any).mockResolvedValue(makeSimHours());

        renderForecast();

        await waitFor(() => {
            expect(screen.getByText('24-Hour Simulation')).toBeInTheDocument();
            expect(screen.getByText(/Predicted energy state starting from/)).toBeInTheDocument();
        });
    });

    it('shows error state when fetch fails', async () => {
        (fetchModeling as any).mockRejectedValue(new Error('Network error'));

        renderForecast();

        await waitFor(() => {
            expect(screen.getByText(/Error: Network error/)).toBeInTheDocument();
        });
    });

    it('shows empty state when no data', async () => {
        (fetchModeling as any).mockResolvedValue([]);

        renderForecast();

        await waitFor(() => {
            expect(screen.getByText('No simulation data available.')).toBeInTheDocument();
        });
    });

});
