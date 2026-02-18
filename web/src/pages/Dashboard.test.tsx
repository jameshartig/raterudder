import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import userEvent from '@testing-library/user-event';
import Dashboard from './Dashboard';
import { Router } from 'wouter';
import { fetchActions, fetchSavings, fetchSettings } from '../api';

// Mock the API
vi.mock('../api', () => ({
    fetchActions: vi.fn(),
    fetchSavings: vi.fn(),
    fetchAuthStatus: vi.fn(),
    fetchSettings: vi.fn(),
    updateSettings: vi.fn(),
    login: vi.fn(),
    logout: vi.fn(),
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
    ActionReason: {
        AlwaysChargeBelowThreshold: 'alwaysChargeBelowThreshold',
        MissingBattery: 'missingBattery',
        DeficitCharge: 'deficitCharge',
        ArbitrageCharge: 'arbitrageCharge',
        DischargeBeforeCapacity: 'dischargeBeforeCapacity',
        DeficitSaveForPeak: 'deficitSaveForPeak',
        ArbitrageSave: 'dischargeAtPeak',
        NoChange: 'sufficientBattery',
        EmergencyMode: 'emergencyMode',
        DeficitSave: 'deficitSave',
    },
}));

const renderWithRouter = (component: React.ReactNode) => {
    return render(
        <Router>
            {component}
        </Router>
    );
};

describe('Dashboard', () => {
    beforeEach(() => {
        vi.resetAllMocks();
    });

    it('renders loading state initially', () => {
        (fetchActions as any).mockReturnValueOnce(new Promise(() => {}));
        renderWithRouter(<Dashboard />);
        expect(screen.getByText('Loading day...')).toBeInTheDocument();
    });

    it('renders actions with reason-based text', async () => {
        const actions = [{
            reason: 'alwaysChargeBelowThreshold',
            description: 'This is a legacy description',
            timestamp: new Date().toISOString(),
            batteryMode: 2, // ChargeAny
            solarMode: 0,
            currentPrice: { dollarsPerKWH: 0.04, tsStart: '', tsEnd: '' },
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            // Should show reason-based text, not description
            expect(screen.getByText(/Price is low.*Charging batteries/)).toBeInTheDocument();
            // Legacy description should NOT be shown
            expect(screen.queryByText('This is a legacy description')).not.toBeInTheDocument();
        });
    });

    it('falls back to description when reason is empty (legacy actions)', async () => {
        const actions = [{
            description: 'This is a test',
            timestamp: new Date().toISOString(),
            batteryMode: 1,
            solarMode: 1,
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            const standbyElements = screen.getAllByText('Hold Battery');
            expect(standbyElements.length).toBeGreaterThan(0);
            expect(screen.getByText('This is a test')).toBeInTheDocument();
        });
    });

    it('renders no actions message when empty', async () => {
        (fetchActions as any).mockResolvedValue([]);
        renderWithRouter(<Dashboard />);
        await waitFor(() => {
            expect(screen.getByText('No actions recorded for this day.')).toBeInTheDocument();
        });
    });

    it('navigates to previous day', async () => {
         const user = userEvent.setup();
         (fetchActions as any).mockResolvedValue([]);
         renderWithRouter(<Dashboard />);

         await waitFor(() => {
             expect(screen.getByText(/Prev/)).toBeInTheDocument();
             expect(screen.getByText(/Prev/)).not.toBeDisabled();
         });

         const prevButton = screen.getByText(/Prev/);
         await user.click(prevButton);

         await waitFor(() => {
             const calls = (fetchActions as any).mock.calls;
             if (calls.length < 2) throw new Error('fetchActions not called twice');
             const lastCall = calls[calls.length - 1];
             const startArg = lastCall[0] as Date;
             const now = new Date();
             const expectedDate = new Date(now);
             expectedDate.setDate(expectedDate.getDate() - 1);
             expect(startArg.getDate()).toBe(expectedDate.getDate());
         });
    });

    it('renders dry run badge', async () => {
        const actions = [{
            reason: 'alwaysChargeBelowThreshold',
            description: 'Dry run test',
            timestamp: new Date().toISOString(),
            batteryMode: 1, // Standby
            solarMode: 1, // NoExport
            dryRun: true,
            currentPrice: { dollarsPerKWH: 0.01, tsStart: '', tsEnd: '' },
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            expect(screen.getByText('Dry Run')).toBeInTheDocument();
            expect(screen.getByText('Dry Run')).toHaveClass('tag', 'dry-run');
        });
    });

    it('hides no change badges', async () => {
        const actions = [{
            description: 'Mixed modes test',
            timestamp: new Date().toISOString(),
            batteryMode: 0, // NoChange
            solarMode: 1, // NoExport
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            // Solar mode should be visible
            expect(screen.getByText('No Export')).toBeInTheDocument();
            // Battery mode (NoChange) should NOT be visible as a badge/tag
            // However, the label might be used elsewhere?
            // In Dashboard.tsx:
            // <h3>{getBatteryModeLabel(action.batteryMode)}</h3> renders the label in h3.
            // But the badges are in .tags span.

            // Let's check specifically for the badge
            const badges = screen.queryAllByText((content, element) => {
                return element !== null && element.classList.contains('tag') && content === 'No Change';
            });
            expect(badges.length).toBe(0);
        });
    });

    it('groups consecutive fault actions into summary', async () => {
        const actions = [
            {
                description: 'Fault 1',
                timestamp: new Date('2023-01-01T10:00:00').toISOString(),
                batteryMode: 0,
                solarMode: 0,
                fault: true,
                systemStatus: {
                    alarms: [{ name: 'GridFault' }]
                },
                currentPrice: { dollarsPerKWH: 0.10, tsStart: '', tsEnd: '' }
            },
            {
                description: 'Fault 2',
                timestamp: new Date('2023-01-01T10:30:00').toISOString(),
                batteryMode: 0,
                solarMode: 0,
                fault: true,
                systemStatus: {
                    alarms: [{ name: 'InverterFault' }]
                },
                currentPrice: { dollarsPerKWH: 0.20, tsStart: '', tsEnd: '' }
            }
        ];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            // Should show "System Fault" title/header
            expect(screen.getByRole('heading', { name: /System Fault/ })).toBeInTheDocument();
            // Should show alarm names - the order depends on Set iteration, usually insertion order
            // Since we add GridFault then InverterFault, it should be "GridFault, InverterFault"
            // However, regex is safer if order is not guaranteed, but usually it is for Sets of strings added in order.
            expect(screen.getByText(/Alarms: GridFault, InverterFault/)).toBeInTheDocument();
            // Should show count
            expect(screen.getByText('(2x)')).toBeInTheDocument();
        });
    });

    it('groups consecutive no change actions into summary', async () => {
        const actions = [
            {
                description: 'No change 1',
                timestamp: new Date('2023-01-01T10:00:00').toISOString(),
                batteryMode: 0, // NoChange
                solarMode: 0, // NoChange
                currentPrice: { dollarsPerKWH: 0.10, tsStart: '', tsEnd: '' }
            },
            {
                description: 'No change 2',
                timestamp: new Date('2023-01-01T10:30:00').toISOString(),
                batteryMode: 0,
                solarMode: 0,
                currentPrice: { dollarsPerKWH: 0.20, tsStart: '', tsEnd: '' }
            }
        ];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            // Should show "No Change" title/header
            expect(screen.getByRole('heading', { name: /No Change/ })).toBeInTheDocument();
            // Should show average price: (0.10 + 0.20) / 2 = 0.15
            expect(screen.getByText(/Avg Price:/)).toBeInTheDocument();
            expect(screen.getByText(/\$0.150\/kWh/)).toBeInTheDocument();
            // Should show range: 0.10 - 0.20
            expect(screen.getByText(/Range: \$0.100 - \$0.200/)).toBeInTheDocument();
            // Should show count in title
            expect(screen.getByText('(2x)')).toBeInTheDocument();
        });
    });

    it('renders daily savings summary', async () => {
        (fetchActions as any).mockResolvedValue([]);
        (fetchSavings as any).mockResolvedValue({
            batterySavings: 5.50,
            solarSavings: 5.00,
            cost: 2.00,
            credit: 1.00,
            avoidedCost: 6.00,
            chargingCost: 0.50,
            solarGenerated: 20,
            gridImported: 10,
            gridExported: 5,
            homeUsed: 25,
            batteryUsed: 10
        });

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            expect(screen.getByText('Daily Overview')).toBeInTheDocument();
            expect(screen.getByText('Net Savings')).toBeInTheDocument();
            expect(screen.getByText('$10.50')).toBeInTheDocument();
            expect(screen.getByText('Solar Savings')).toBeInTheDocument();
            expect(screen.getByText('$5.00')).toBeInTheDocument();
            expect(screen.getByText('Battery Savings')).toBeInTheDocument();
            expect(screen.getByText('$5.50')).toBeInTheDocument();
        });
    });

    it('renders deficit charge reason with future price and deficit time', async () => {
        const deficitTime = new Date('2026-02-15T14:00:00').toISOString();
        const actions = [{
            reason: 'deficitCharge',
            description: 'Charging Optimized: Projected Deficit...',
            timestamp: new Date().toISOString(),
            batteryMode: 2, // ChargeAny
            solarMode: 0,
            currentPrice: { dollarsPerKWH: 0.10, tsStart: '', tsEnd: '' },
            futurePrice: { dollarsPerKWH: 0.50, tsStart: '', tsEnd: '' },
            deficitAt: deficitTime,
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            // Should show the templatized deficit charge text
            expect(screen.getByText(/Battery deficit predicted/)).toBeInTheDocument();
            expect(screen.getByText(/Charging now/)).toBeInTheDocument();
            expect(screen.getByText(/peak later/)).toBeInTheDocument();
            // Should show the future price in footer
            expect(screen.getByText(/Peak:.*\$0.500/)).toBeInTheDocument();
        });
    });

    it('renders sufficient battery reason text', async () => {
        const actions = [{
            reason: 'sufficientBattery',
            description: 'Sufficient battery.',
            timestamp: new Date().toISOString(),
            batteryMode: -1, // Load
            solarMode: 0,
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            expect(screen.getByText('Battery is sufficient. Using battery normally.')).toBeInTheDocument();
        });
    });

    it('renders arbitrage charge reason text', async () => {
        const actions = [{
            reason: 'arbitrageCharge',
            description: 'Charging Optimized: Arbitrage...',
            timestamp: new Date().toISOString(),
            batteryMode: 2, // ChargeAny
            solarMode: 0,
            currentPrice: { dollarsPerKWH: 0.10, tsStart: '', tsEnd: '' },
            futurePrice: { dollarsPerKWH: 0.50, tsStart: '', tsEnd: '' },
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            expect(screen.getByText(/Arbitrage opportunity.*charging at.*\$0.100/)).toBeInTheDocument();
            expect(screen.getByText(/peak later at.*\$0.500/)).toBeInTheDocument();
        });
    });

    it('shows banner when Franklin credentials are missing', async () => {
        (fetchActions as any).mockResolvedValue([]);
        (fetchSavings as any).mockResolvedValue(null);
        (fetchSettings as any).mockResolvedValue({
            minBatterySOC: 10,
            hasCredentials: { franklin: false }
        });

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            expect(screen.getByText(/FranklinWH credentials are not configured/i)).toBeInTheDocument();
            expect(screen.getByText(/Configure them in Settings/i)).toBeInTheDocument();
        });
    });

    it('does not show banner when Franklin credentials are present', async () => {
        (fetchActions as any).mockResolvedValue([]);
        (fetchSavings as any).mockResolvedValue(null);
        (fetchSettings as any).mockResolvedValue({
            minBatterySOC: 10,
            hasCredentials: { franklin: true },
            utilityProvider: 'comed_besh'
        });

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            // Need to wait for loading to finish
            expect(screen.queryByText('Loading day...')).not.toBeInTheDocument();
        });

        expect(screen.queryByText(/FranklinWH credentials are not configured/i)).not.toBeInTheDocument();
    });

    it('shows banner when Utility Provider is missing', async () => {
        (fetchActions as any).mockResolvedValue([]);
        (fetchSavings as any).mockResolvedValue(null);
        (fetchSettings as any).mockResolvedValue({
            minBatterySOC: 10,
            hasCredentials: { franklin: true },
            utilityProvider: ''
        });

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            expect(screen.getByText(/Utility Provider is not configured/i)).toBeInTheDocument();
            expect(screen.getByText(/Configure it in Settings/i)).toBeInTheDocument();
        });
    });

    it('does not show banner when Utility Provider is present', async () => {
        (fetchActions as any).mockResolvedValue([]);
        (fetchSavings as any).mockResolvedValue(null);
        (fetchSettings as any).mockResolvedValue({
            minBatterySOC: 10,
            hasCredentials: { franklin: true },
            utilityProvider: 'comed_besh'
        });

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            expect(screen.queryByText('Loading day...')).not.toBeInTheDocument();
        });

        expect(screen.queryByText(/Utility Provider is not configured/i)).not.toBeInTheDocument();

    });

    it('renders manual emergency mode correctly', async () => {
        const actions = [{
            reason: 'emergencyMode',
            description: 'Emergency Mode Active',
            timestamp: new Date().toISOString(),
            batteryMode: 0,
            solarMode: 0,
            fault: true,
            systemStatus: {
                alarms: [],
                storm: []
            },
            currentPrice: { dollarsPerKWH: 0.10, tsStart: '', tsEnd: '' }
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            expect(screen.getByRole('heading', { name: /Emergency Mode/ })).toBeInTheDocument();
            expect(screen.getByText('System manually put into emergency mode. Skipping automation.')).toBeInTheDocument();
        });
    });

    it('renders storm protection mode correctly with times', async () => {
        const stormStart = new Date('2023-01-01T12:00:00');
        const stormEnd = new Date('2023-01-01T15:00:00');
        const actions = [{
            reason: 'emergencyMode',
            description: 'Storm Prep',
            timestamp: new Date('2023-01-01T10:00:00').toISOString(),
            batteryMode: 0,
            solarMode: 0,
            fault: true,
            systemStatus: {
                alarms: [],
                storms: [{
                    description: 'Thunderstorm',
                    tsStart: stormStart.toISOString(),
                    tsEnd: stormEnd.toISOString(),
                }]
            },
            currentPrice: { dollarsPerKWH: 0.10, tsStart: '', tsEnd: '' }
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            expect(screen.getByRole('heading', { name: /Storm Protection Mode/ })).toBeInTheDocument();
            expect(screen.getByText('Franklin is charging the battery to prepare for the storm.')).toBeInTheDocument();
            expect(screen.getByText(/Storm Duration: 12:00 PM - 3:00 PM/)).toBeInTheDocument();
        });
    });

    it('hides price footer in summary when no price data is available', async () => {
        const actions = [{
            reason: 'emergencyMode',
            description: 'Emergency Mode Active',
            timestamp: new Date().toISOString(),
            batteryMode: 0,
            solarMode: 0,
            fault: true,
            systemStatus: {
                alarms: [],
                storms: []
            },
            // currentPrice is undefined
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<Dashboard />);

        await waitFor(() => {
            expect(screen.getByRole('heading', { name: /Emergency Mode/ })).toBeInTheDocument();
            // Price label should not be present
            expect(screen.queryByText('Avg Price:')).not.toBeInTheDocument();
        });
    });
});



