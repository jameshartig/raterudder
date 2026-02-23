import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import App from '../App';
import * as api from '../api';
import { setupDefaultApiMocks, defaultAuthStatus } from '../test/apiMocks';

const { fetchAuthStatus, fetchSettings, updateSettings, login, logout } = api;

// Mock the API
vi.mock('../api', async (importOriginal) => {
    const actual = await importOriginal<typeof import('../api')>();
    return {
        ...actual,
        fetchActions: vi.fn(),
        fetchSavings: vi.fn(),
        fetchAuthStatus: vi.fn(),
        fetchSettings: vi.fn(),
        updateSettings: vi.fn(),
        login: vi.fn(),
        logout: vi.fn(),
        fetchModeling: vi.fn(),
    };
});

// Mock Google OAuth
vi.mock('@react-oauth/google', () => ({
    GoogleOAuthProvider: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
    GoogleLogin: ({ onSuccess }: { onSuccess: (res: any) => void }) => (
        <button onClick={() => onSuccess({ credential: 'test-token' })}>
            Google Sign In
        </button>
    ),
}));

// Helper to navigate to settings page
const navigateToSettings = async () => {
    (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus });
    render(<App />);
    fireEvent.click(screen.getByText('Login'));
    await waitFor(() => expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument());
    fireEvent.click(screen.getByRole('link', { name: 'Settings' }));
};

describe('App & Settings', () => {
    beforeEach(() => {
        vi.resetAllMocks();

        const originalError = console.error;
        vi.spyOn(console, 'error').mockImplementation((msg, ...args) => {
            if (typeof msg === 'string' && msg.includes('was not wrapped in act')) return;
            originalError(msg, ...args);
        });

        // Default mocks
        setupDefaultApiMocks(api);
    });

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

    it('shows settings link when logged in', async () => {
        (fetchAuthStatus as any).mockResolvedValue({ ...defaultAuthStatus });

        render(<App />);
        fireEvent.click(screen.getByText('Login'));

        await waitFor(() => {
            expect(screen.getByText('Settings')).toBeInTheDocument();
        });
    });

    it('navigates to settings and loads data', async () => {
        await navigateToSettings();

        // Advanced inputs are inside a Collapsible panel. Click to expand first.
        const advancedBtn = await screen.findByText('Advanced Settings');
        fireEvent.click(advancedBtn);

        await waitFor(() => {
            expect(screen.getByLabelText(/Min Battery SOC/i)).toBeInTheDocument();
            expect(screen.getByDisplayValue('10')).toBeInTheDocument();
        });
    });

    it('can update settings', async () => {
         await navigateToSettings();

         // Expand advanced settings first
         const advancedBtn = await screen.findByText('Advanced Settings');
         fireEvent.click(advancedBtn);

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
         await navigateToSettings();

         // Expand advanced settings to find Pause
         const advancedBtn = await screen.findByText('Advanced Settings');
         fireEvent.click(advancedBtn);

         // Toggle Pause switch — find the switch button by its accessible name
         await waitFor(() => expect(screen.getByRole('switch', { name: /Pause Updates/i })).toBeInTheDocument());
         const switchEl = screen.getByRole('switch', { name: /Pause Updates/i });
         fireEvent.click(switchEl);

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

    it('can toggle grid export batteries setting', async () => {
         await navigateToSettings();

         // Toggle Grid Export Batteries switch
         await waitFor(() => expect(screen.getByRole('switch', { name: /Grid Export Batteries/i })).toBeInTheDocument());
         const switchEl = screen.getByRole('switch', { name: /Grid Export Batteries/i });
         fireEvent.click(switchEl);

         // Mock update success
         (updateSettings as any).mockResolvedValue(undefined);

         // Helper to click save
         const saveBtn = screen.getByText('Save Settings');
         fireEvent.click(saveBtn);

         await waitFor(() => {
             expect(screen.getByText('Settings saved successfully')).toBeInTheDocument();
             expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
                 gridExportBatteries: true
             }), expect.any(String), undefined);
         });
    });

    it('renders solar settings inputs on settings page', async () => {
        await navigateToSettings();

        // Expand advanced settings
        const advancedBtn = await screen.findByText('Advanced Settings');
        fireEvent.click(advancedBtn);

        await waitFor(() => {
            expect(screen.getByLabelText(/Solar Trend Ratio Max/i)).toBeInTheDocument();
            expect(screen.getByLabelText(/Solar Bell Curve Multiplier/i)).toBeInTheDocument();
            expect(screen.getByDisplayValue('3')).toBeInTheDocument();
            expect(screen.getByDisplayValue('1')).toBeInTheDocument();
        });
    });

    it('can update solar bell curve multiplier', async () => {
        await navigateToSettings();

        // Expand advanced settings
        const advancedBtn = await screen.findByText('Advanced Settings');
        fireEvent.click(advancedBtn);

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
            gridExportBatteries: false,
            gridChargeBatteries: true,
            solarTrendRatioMax: 3.0,
            solarBellCurveMultiplier: 1.0,
            ignoreHourUsageOverMultiple: 2,
            alwaysChargeUnderDollarsPerKWH: 0.05,
            minArbitrageDifferenceDollarsPerKWH: 0.03,
            minDeficitPriceDifferenceDollarsPerKWH: 0.02,
            utilityProvider: 'comed_besh',
            utilityRateOptions: {},
            hasCredentials: {
                franklin: false
            }
        });
        await navigateToSettings();

        // Expand advanced settings to see the warning
        const advancedBtn = await screen.findByText('Advanced Settings');
        fireEvent.click(advancedBtn);

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
            gridExportBatteries: false,
            gridChargeBatteries: true,
            solarTrendRatioMax: 3.0,
            solarBellCurveMultiplier: 0.3,
            ignoreHourUsageOverMultiple: 2,
            alwaysChargeUnderDollarsPerKWH: 0.05,
            minArbitrageDifferenceDollarsPerKWH: 0.03,
            minDeficitPriceDifferenceDollarsPerKWH: 0.02,
            utilityProvider: 'comed_besh',
            utilityRateOptions: {},
            hasCredentials: {
                franklin: false
            }
        });
        await navigateToSettings();

        // Expand advanced settings to see the warning
        const advancedBtn = await screen.findByText('Advanced Settings');
        fireEvent.click(advancedBtn);

        await waitFor(() => {
            expect(screen.getByText(/Solar export is disabled but the bell curve multiplier is low/)).toBeInTheDocument();
        });
    });

    it('can update ComEd rate options', async () => {
        await navigateToSettings();

        // Wait for ComEd section
        await waitFor(() => expect(screen.getByText('ComEd Rate Options')).toBeInTheDocument());

        // Toggle Delivery Time-of-Day switch
        const dtodSwitch = screen.getByRole('switch', { name: /Delivery Time-of-Day/i });
        fireEvent.click(dtodSwitch);

        // Save
        const saveBtn = screen.getByText('Save Settings');
        fireEvent.click(saveBtn);

        await waitFor(() => {
            expect(screen.getByText('Settings saved successfully')).toBeInTheDocument();
            expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
                utilityRateOptions: expect.objectContaining({
                    variableDeliveryRate: true
                })
            }), expect.any(String), undefined);
        });
    });

    it('can expand advanced settings and update fields', async () => {
        await navigateToSettings();

        // Find collapsible trigger button
        const advancedBtn = await screen.findByText('Advanced Settings');

        // Click to open
        fireEvent.click(advancedBtn);

        // Check fields inside are accessible
        const priceInput = screen.getByLabelText(/Always Charge Under/i);
        fireEvent.change(priceInput, { target: { value: '0.10' } });

        // Save
        const saveBtn = screen.getByText('Save Settings');
        fireEvent.click(saveBtn);

        await waitFor(() => {
            expect(screen.getByText('Settings saved successfully')).toBeInTheDocument();
            expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
                alwaysChargeUnderDollarsPerKWH: 0.10
            }), expect.any(String), undefined);
        });
    });
});
