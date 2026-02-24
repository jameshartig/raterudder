import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import SavingsHero from './SavingsHero';
import { type SavingsStats } from '../api';

describe('SavingsHero', () => {
    const defaultSavings: SavingsStats = {
        timestamp: new Date().toISOString(),
        batterySavings: 10,
        solarSavings: 20,
        cost: 5,
        credit: 2,
        avoidedCost: 12,
        chargingCost: 2,
        solarGenerated: 30,
        gridImported: 15,
        gridExported: 5,
        homeUsed: 40,
        batteryUsed: 10,
    };

    it('renders net savings correctly', () => {
        render(<SavingsHero savings={defaultSavings} />);
        // 10 + 20 + 2 = 32
        expect(screen.getByText('$ 32.00')).toBeInTheDocument();
    });

    it('renders negative savings with a minus sign', () => {
        const negativeSavings = { ...defaultSavings, batterySavings: -50 };
        render(<SavingsHero savings={negativeSavings} />);
        // -50 + 20 + 2 = -28
        expect(screen.getByText('- $ 28.00')).toBeInTheDocument();
    });

    it('renders individual metrics', () => {
        render(<SavingsHero savings={defaultSavings} />);
        expect(screen.getByText('Solar Gen')).toBeInTheDocument();
        expect(screen.getByText('30.0')).toBeInTheDocument();
        expect(screen.getByText('Home Usage')).toBeInTheDocument();
        expect(screen.getByText('40.0')).toBeInTheDocument();
    });

    it('returns null if savings is null', () => {
        const { container } = render(<SavingsHero savings={null} />);
        expect(container.firstChild).toBeNull();
    });
});
