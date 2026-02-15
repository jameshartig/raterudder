import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import App from './App';
import { fetchActions, fetchAuthStatus, fetchSettings, updateSettings, login, logout } from './api';

// Mock the API
vi.mock('./api', () => ({
    fetchActions: vi.fn(),
    fetchSavings: vi.fn(),
    fetchAuthStatus: vi.fn(),
    fetchSettings: vi.fn(),
    updateSettings: vi.fn(),
    login: vi.fn(),
    logout: vi.fn(),
    fetchModeling: vi.fn(),
    BatteryMode: {
        NoChange: 0,
        Standby: 1,
        ChargeAny: 2,
        ChargeSolar: 3,
        Load: -1,
    },
    SolarMode: {
        NoChange: 0,
        NoExport: 1,
        Any: 2,
    },
}));

// Mock Google OAuth
vi.mock('@react-oauth/google', () => ({
    GoogleOAuthProvider: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
    GoogleLogin: ({ onSuccess }: { onSuccess: (res: any) => void }) => (
        <button onClick={() => onSuccess({ credential: 'test-token' })}>
            Google Sign In
        </button>
    ),
}));

describe('App & Settings', () => {
    beforeEach(() => {
        vi.resetAllMocks();
        // Default mocks
        (fetchActions as any).mockResolvedValue([]);
        (fetchSettings as any).mockResolvedValue({
            dryRun: false,
            pause: false,
            minBatterySOC: 10,
            gridExportSolar: false,
            gridChargeBatteries: true,
            solarTrendRatioMax: 3.0,
            solarBellCurveMultiplier: 1.0,
            ignoreHourUsageOverMultiple: 2,
            alwaysChargeUnderDollarsPerKWH: 0.05,
            additionalFeesDollarsPerKWH: 0.02,
            minArbitrageDifferenceDollarsPerKWH: 0.03,
            minDeficitPriceDifferenceDollarsPerKWH: 0.02,
            utilityProvider: 'comed_hourly',
        });
    });

    const defaultAuthStatus = {
        isAdmin: false,
        loggedIn: true,
        authRequired: true,
        clientID: 'test-client-id',
        email: 'user@example.com',
        siteIDs: ['site1']
    };

    it('shows login button when auth required and not logged in', async () => {
        (fetchAuthStatus as any).mockResolvedValue({
            ...defaultAuthStatus,
            loggedIn: false
        });

        render(<App />);

        // On LandingPage, click Login link in header
        fireEvent.click(screen.getByText('Login'));

        await waitFor(() => {
            expect(screen.getByText('Google Sign In')).toBeInTheDocument();
        });
    });

    it('calls login api on successful google login', async () => {
         (fetchAuthStatus as any).mockResolvedValueOnce({
            ...defaultAuthStatus,
            loggedIn: false
        }).mockResolvedValueOnce({
            ...defaultAuthStatus,
            loggedIn: true
        });

        (login as any).mockResolvedValue(undefined);

        render(<App />);
        fireEvent.click(screen.getByText('Login'));

        await waitFor(() => {
            expect(screen.getByText('Google Sign In')).toBeInTheDocument();
        });

        fireEvent.click(screen.getByText('Google Sign In'));

        await waitFor(() => {
            expect(login).toHaveBeenCalledWith('test-token');
        });
    });

    it('shows logout button when logged in and calls logout on click', async () => {
        (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus, loggedIn: true });
        (logout as any).mockResolvedValue(undefined);

        render(<App />);
        fireEvent.click(screen.getByText('Login'));

        await waitFor(() => {
            expect(screen.getByText('Log Out')).toBeInTheDocument();
        });

        fireEvent.click(screen.getByText('Log Out'));

        await waitFor(() => {
            expect(logout).toHaveBeenCalled();
        });
    });

    it('shows settings link when not admin', async () => {
        (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus, isAdmin: false });

        render(<App />);
        fireEvent.click(screen.getByText('Login'));

        await waitFor(() => {
            expect(screen.getByText('Settings')).toBeInTheDocument();
        });
    });

    it('settings page is read-only when not admin', async () => {
        (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus, isAdmin: false });
        render(<App />);
        fireEvent.click(screen.getByText('Login'));

        // Navigate
        await waitFor(() => expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument());
        fireEvent.click(screen.getByRole('link', { name: 'Settings' }));

        // Check button
        await waitFor(() => {
            const btn = screen.getByText('Read Only');
            expect(btn).toBeInTheDocument();
            expect(btn).toBeDisabled();
        });
    });

    it('shows settings link when admin', async () => {
        (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus, isAdmin: true });

        render(<App />);
        fireEvent.click(screen.getByText('Login'));

        await waitFor(() => {
            expect(screen.getByText('Settings')).toBeInTheDocument();
        });
    });

    it('navigates to settings and loads data', async () => {
        (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus, isAdmin: true });

        render(<App />);
        fireEvent.click(screen.getByText('Login'));

        // Wait for link to appear
        await waitFor(() => {
            expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument();
        });

        // Click settings
        fireEvent.click(screen.getByRole('link', { name: 'Settings' }));

        // Check if settings component loaded and fetched data
        await waitFor(() => {
            expect(screen.getByLabelText(/Min Battery SOC/i)).toBeInTheDocument();
            expect(screen.getByDisplayValue('10')).toBeInTheDocument();
        });
    });

    it('can update settings', async () => {
         (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus, isAdmin: true });
         render(<App />);
         fireEvent.click(screen.getByText('Login'));

         // Navigate
         await waitFor(() => expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument());
         fireEvent.click(screen.getByRole('link', { name: 'Settings' }));

         // Change input
         await waitFor(() => expect(screen.getByLabelText(/Min Battery SOC/i)).toBeInTheDocument());
         const input = screen.getByLabelText(/Min Battery SOC/i);
         fireEvent.change(input, { target: { value: '20' } });

         // Mock update success
         (updateSettings as any).mockResolvedValue(undefined);

         // Helper to click save
         const saveBtn = screen.getByText('Save Settings');
         fireEvent.click(saveBtn);

         await waitFor(() => {
             expect(screen.getByText('Settings saved successfully')).toBeInTheDocument();
             expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
                 minBatterySOC: 20
             }), expect.any(String), undefined);
         });
    });

    it('can toggle pause setting', async () => {
         (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus, isAdmin: true });
         render(<App />);
         fireEvent.click(screen.getByText('Login'));

         // Navigate
         await waitFor(() => expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument());
         fireEvent.click(screen.getByRole('link', { name: 'Settings' }));

         // Toggle Pause
         await waitFor(() => expect(screen.getByLabelText(/Pause Updates/i)).toBeInTheDocument());
         const input = screen.getByLabelText(/Pause Updates/i);
         fireEvent.click(input);

         // Mock update success
         (updateSettings as any).mockResolvedValue(undefined);

         // Helper to click save
         const saveBtn = screen.getByText('Save Settings');
         fireEvent.click(saveBtn);

         await waitFor(() => {
             expect(screen.getByText('Settings saved successfully')).toBeInTheDocument();
             expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
                 pause: true
             }), expect.any(String), undefined);
         });
    });

    it('renders solar settings inputs on settings page', async () => {
        (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus, isAdmin: true });
        render(<App />);
        fireEvent.click(screen.getByText('Login'));

        await waitFor(() => expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument());
        fireEvent.click(screen.getByRole('link', { name: 'Settings' }));

        await waitFor(() => {
            expect(screen.getByLabelText(/Solar Trend Ratio Max/i)).toBeInTheDocument();
            expect(screen.getByLabelText(/Solar Bell Curve Multiplier/i)).toBeInTheDocument();
            expect(screen.getByDisplayValue('3')).toBeInTheDocument();
            expect(screen.getByDisplayValue('1')).toBeInTheDocument();
        });
    });

    it('can update solar bell curve multiplier', async () => {
        (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus, isAdmin: true });
        render(<App />);
        fireEvent.click(screen.getByText('Login'));

        await waitFor(() => expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument());
        fireEvent.click(screen.getByRole('link', { name: 'Settings' }));

        await waitFor(() => expect(screen.getByLabelText(/Solar Bell Curve Multiplier/i)).toBeInTheDocument());
        const input = screen.getByLabelText(/Solar Bell Curve Multiplier/i);
        fireEvent.change(input, { target: { value: '0.5' } });

        (updateSettings as any).mockResolvedValue(undefined);
        fireEvent.click(screen.getByText('Save Settings'));

        await waitFor(() => {
            expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
                solarBellCurveMultiplier: 0.5
            }), expect.any(String), undefined);
        });
    });

    it('shows warning when export enabled and bell curve multiplier is high', async () => {
        (fetchSettings as any).mockResolvedValue({
            dryRun: false,
            pause: false,
            minBatterySOC: 10,
            gridExportSolar: true,
            gridChargeBatteries: true,
            solarTrendRatioMax: 3.0,
            solarBellCurveMultiplier: 1.0,
            ignoreHourUsageOverMultiple: 2,
            alwaysChargeUnderDollarsPerKWH: 0.05,
            additionalFeesDollarsPerKWH: 0.02,
            minArbitrageDifferenceDollarsPerKWH: 0.03,
            minDeficitPriceDifferenceDollarsPerKWH: 0.02,
            utilityProvider: 'comed_hourly',
        });
        (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus, isAdmin: true });
        render(<App />);
        fireEvent.click(screen.getByText('Login'));

        await waitFor(() => expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument());
        fireEvent.click(screen.getByRole('link', { name: 'Settings' }));

        await waitFor(() => {
            expect(screen.getByText(/Solar export is enabled but the bell curve multiplier is high/)).toBeInTheDocument();
        });
    });

    it('shows warning when export disabled and bell curve multiplier is low', async () => {
        (fetchSettings as any).mockResolvedValue({
            dryRun: false,
            pause: false,
            minBatterySOC: 10,
            gridExportSolar: false,
            gridChargeBatteries: true,
            solarTrendRatioMax: 3.0,
            solarBellCurveMultiplier: 0.3,
            ignoreHourUsageOverMultiple: 2,
            alwaysChargeUnderDollarsPerKWH: 0.05,
            additionalFeesDollarsPerKWH: 0.02,
            minArbitrageDifferenceDollarsPerKWH: 0.03,
            minDeficitPriceDifferenceDollarsPerKWH: 0.02,
            utilityProvider: 'comed_hourly',
        });
        (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus, isAdmin: true });
        render(<App />);
        fireEvent.click(screen.getByText('Login'));

        await waitFor(() => expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument());
        fireEvent.click(screen.getByRole('link', { name: 'Settings' }));

        await waitFor(() => {
            expect(screen.getByText(/Solar export is disabled but the bell curve multiplier is low/)).toBeInTheDocument();
        });
    });
});
