
import { render, screen } from '@testing-library/react';
import { Router } from 'wouter';
import LandingPage from './LandingPage';
import App from '../App';
import { describe, it, expect, vi } from 'vitest';
import * as api from '../api';

vi.mock('../api', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api')>();
    return {
        ...original,
        fetchAuthStatus: vi.fn(original.fetchAuthStatus),
    };
});

const { fetchAuthStatus } = api;

describe('LandingPage Component', () => {
    it('renders marketing copy', () => {
        render(
            <Router>
                <LandingPage />
            </Router>
        );

        // Check for new hero text
        expect(screen.getByText((content) => content.startsWith('RateRudder learns your home'))).toBeInTheDocument();

        // Check for FAQ section
        expect(screen.getByText('Frequently Asked Questions')).toBeInTheDocument();
    });

    it('does not have a login CTA button', () => {
        render(
            <Router>
                <LandingPage />
            </Router>
        );

        expect(screen.queryByText('Login / Dashboard')).not.toBeInTheDocument();
    });

    it('does not call fetchAuthStatus on initial landing page load', async () => {
        render(<App />);
        expect(fetchAuthStatus).not.toHaveBeenCalled();
    });
});
