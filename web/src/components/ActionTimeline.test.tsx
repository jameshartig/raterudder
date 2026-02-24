import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import ActionTimeline from './ActionTimeline';
import { BatteryMode, SolarMode, ActionReason, type Action } from '../api';
import { type ActionSummary } from '../utils/dashboardUtils';

describe('ActionTimeline', () => {
    it('renders regular action items', () => {
        const actions: Action[] = [{
            timestamp: new Date().toISOString(),
            batteryMode: BatteryMode.Standby,
            solarMode: SolarMode.NoExport,
            description: 'Test action'
        }];
        render(<ActionTimeline groupedActions={actions} />);
        expect(screen.getAllByText('Hold Battery').length).toBeGreaterThan(0);
        expect(screen.getByText('Test action')).toBeInTheDocument();
    });

    it('renders action summaries', () => {
        const summary: ActionSummary = {
            isSummary: true,
            type: 'no_change',
            startTime: new Date().toISOString(),
            latestAction: {
                timestamp: new Date().toISOString(),
                batteryMode: BatteryMode.NoChange,
                solarMode: SolarMode.NoChange,
                reason: ActionReason.SufficientBattery
            } as Action,
            count: 5,
            alarms: new Set(),
            storms: new Set(),
            hasPrice: false,
            hasSOC: false,
            avgPrice: 0,
            min: 0,
            max: 0,
            avgSOC: 0,
            minSOC: 0,
            maxSOC: 0
        };
        render(<ActionTimeline groupedActions={[summary]} />);
        expect(screen.getByText('No Change')).toBeInTheDocument();
        expect(screen.getByText('(5x)')).toBeInTheDocument();
        expect(screen.getByText(/battery has enough stored energy/)).toBeInTheDocument();
    });
});
