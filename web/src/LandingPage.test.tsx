import { render, screen } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import LandingPage from './LandingPage';
import { describe, it, expect, vi } from 'vitest';

// Global mock for ResizeObserver
if (typeof window !== 'undefined') {
    (window as any).ResizeObserver = vi.fn().mockImplementation(() => ({
        observe: vi.fn(),
        unobserve: vi.fn(),
        disconnect: vi.fn(),
    }));
}

// Mock recharts to avoid rendering issues in jsdom
vi.mock('recharts', () => ({
    ResponsiveContainer: ({ children }: any) => <div data-testid="responsive-container">{children}</div>,
    AreaChart: ({ children }: any) => <div data-testid="area-chart"><svg>{children}</svg></div>,
    Area: () => <g data-testid="area" />,
    LineChart: ({ children }: any) => <div data-testid="line-chart"><svg>{children}</svg></div>,
    Line: () => <g data-testid="line" />,
    XAxis: () => <g data-testid="x-axis" />,
    YAxis: () => <g data-testid="y-axis" />,
    CartesianGrid: () => <g data-testid="cartesian-grid" />,
    Tooltip: () => <g data-testid="tooltip" />,
    ReferenceLine: () => <g data-testid="reference-line" />,
}));

describe('LandingPage Component', () => {
    it('renders marketing copy', () => {
        render(
            <BrowserRouter>
                <LandingPage />
            </BrowserRouter>
        );

        expect(screen.getByText('Navigate Utility Prices')).toBeInTheDocument();
        expect(screen.getByText(/Maximize solar generation/)).toBeInTheDocument();
        expect(screen.getByText('Solar Generation Model')).toBeInTheDocument();
    });

    it('does not have a login CTA button', () => {
        render(
            <BrowserRouter>
                <LandingPage />
            </BrowserRouter>
        );

        expect(screen.queryByText('Login / Dashboard')).not.toBeInTheDocument();
    });
});
