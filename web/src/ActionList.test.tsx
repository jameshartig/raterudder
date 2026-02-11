import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import userEvent from '@testing-library/user-event';
import ActionList from './ActionList';
import { BrowserRouter } from 'react-router-dom';
import { fetchActions, fetchSavings } from './api';

// Mock the API
vi.mock('./api', () => ({
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
}));

const renderWithRouter = (component: React.ReactNode) => {
    return render(
        <BrowserRouter>
            {component}
        </BrowserRouter>
    );
};

describe('ActionList', () => {
    beforeEach(() => {
        vi.resetAllMocks();
    });

    it('renders loading state initially', () => {
        (fetchActions as any).mockReturnValueOnce(new Promise(() => {}));
        renderWithRouter(<ActionList />);
        expect(screen.getByText('Loading day...')).toBeInTheDocument();
    });

    it('renders actions when loaded', async () => {
        const actions = [{
            description: 'This is a test',
            timestamp: new Date().toISOString(),
            batteryMode: 1,
            solarMode: 1,
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<ActionList />);

        await waitFor(() => {
            const standbyElements = screen.getAllByText('Hold Battery');
            expect(standbyElements.length).toBeGreaterThan(0);
            expect(screen.getByText('This is a test')).toBeInTheDocument();
        });
    });

    it('renders no actions message when empty', async () => {
        (fetchActions as any).mockResolvedValue([]);
        renderWithRouter(<ActionList />);
        await waitFor(() => {
            expect(screen.getByText('No actions recorded for this day.')).toBeInTheDocument();
        });
    });

    it('navigates to previous day', async () => {
         const user = userEvent.setup();
         (fetchActions as any).mockResolvedValue([]);
         renderWithRouter(<ActionList />);

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
            description: 'Dry run test',
            timestamp: new Date().toISOString(),
            batteryMode: 1, // Standby
            solarMode: 1, // NoExport
            dryRun: true,
        }];
        (fetchActions as any).mockResolvedValue(actions);

        renderWithRouter(<ActionList />);

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

        renderWithRouter(<ActionList />);

        await waitFor(() => {
            // Solar mode should be visible
            expect(screen.getByText('No Export')).toBeInTheDocument();
            // Battery mode (NoChange) should NOT be visible as a badge/tag
            // However, the label might be used elsewhere?
            // In ActionList.tsx: <h3>{getBatteryModeLabel(action.batteryMode)}</h3> renders the label in h3.
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

        renderWithRouter(<ActionList />);

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

        renderWithRouter(<ActionList />);

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

        renderWithRouter(<ActionList />);

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
});
