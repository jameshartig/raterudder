
import { render, screen } from '@testing-library/react';
import { Router } from 'wouter';
import LandingPage from './LandingPage';
import App from '../App';
import { describe, it, expect } from 'vitest';
import { fetchAuthStatus } from '../api';

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
