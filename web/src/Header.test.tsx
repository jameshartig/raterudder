import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import Header from './Header';
import { describe, it, expect, vi } from 'vitest';

describe('Header Component', () => {
    const mockOnSiteChange = vi.fn();
    const mockOnLogout = vi.fn();

    const renderHeader = (path: string, loggedIn: boolean) => {
        return render(
            <MemoryRouter initialEntries={[path]}>
                <Routes>
                    <Route path="*" element={
                        <Header
                            loggedIn={loggedIn}
                            siteIDs={['site1']}
                            selectedSiteID="site1"
                            onSiteChange={mockOnSiteChange}
                            onLogout={mockOnLogout}
                        />
                    } />
                </Routes>
            </MemoryRouter>
        );
    };

    it('renders correctly when logged out on homepage', () => {
        renderHeader('/', false);

        expect(screen.getByText('RateRudder')).toBeInTheDocument();
        expect(screen.queryByText('History')).not.toBeInTheDocument();
        expect(screen.getByText('Login')).toBeInTheDocument();
    });

    it('hides nav links even when logged in on homepage', () => {
        renderHeader('/', true);

        expect(screen.queryByText('History')).not.toBeInTheDocument();
        expect(screen.queryByText('Model')).not.toBeInTheDocument();
        expect(screen.queryByText('Settings')).not.toBeInTheDocument();

        // Should show Logout though
        expect(screen.getByText('Log Out')).toBeInTheDocument();
    });

    it('shows nav links when logged in on dashboard', () => {
        renderHeader('/dashboard', true);

        expect(screen.getByText('History')).toBeInTheDocument();
        expect(screen.getByText('Model')).toBeInTheDocument();
        expect(screen.getByText('Settings')).toBeInTheDocument();
    });

    it('calls onLogout when logout button is clicked', () => {
        renderHeader('/dashboard', true);
        fireEvent.click(screen.getByText('Log Out'));
        expect(mockOnLogout).toHaveBeenCalledTimes(1);
    });
});
