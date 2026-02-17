
import { render, screen } from '@testing-library/react';
import { Router } from 'wouter';
import LandingPage from './LandingPage';
import { describe, it, expect } from 'vitest';

describe('LandingPage Component', () => {
    it('renders marketing copy', () => {
        render(
            <Router>
                <LandingPage />
            </Router>
        );

        // Check for new hero text
        expect(screen.getByText((content) => content.startsWith('Cut Your Electric Bill'))).toBeInTheDocument();

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
});
