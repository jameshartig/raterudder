import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import NewSitePage from './NewSitePage';
import * as api from '../api';

vi.mock('../api', () => ({
    createSite: vi.fn(),
}));

describe('NewSitePage Component', () => {
    const mockOnJoinSuccess = vi.fn();

    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('renders correctly', () => {
        render(<NewSitePage onJoinSuccess={mockOnJoinSuccess} />);

        expect(screen.getByText('Create a New Site')).toBeInTheDocument();
        expect(screen.getByLabelText('Site Name')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Create Site' })).toBeInTheDocument();
        expect(screen.getByText('Already have a site? Join it.')).toBeInTheDocument();
    });

    it('calls createSite and onJoinSuccess on successful submit', async () => {
        (api.createSite as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);

        render(<NewSitePage onJoinSuccess={mockOnJoinSuccess} />);

        fireEvent.change(screen.getByLabelText('Site Name'), { target: { value: 'My Cool Site' } });
        fireEvent.click(screen.getByRole('button', { name: 'Create Site' }));

        expect(screen.getByRole('button', { name: 'Creating...' })).toBeDisabled();

        await waitFor(() => {
            expect(api.createSite).toHaveBeenCalledWith('My Cool Site');
            expect(mockOnJoinSuccess).toHaveBeenCalled();
        });
    });

    it('displays error message on failure', async () => {
        (api.createSite as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Site creation failed'));

        render(<NewSitePage onJoinSuccess={mockOnJoinSuccess} />);

        fireEvent.change(screen.getByLabelText('Site Name'), { target: { value: 'Bad Site' } });
        fireEvent.click(screen.getByRole('button', { name: 'Create Site' }));

        await waitFor(() => {
            expect(api.createSite).toHaveBeenCalledWith('Bad Site');
            expect(screen.getByText('Site creation failed')).toBeInTheDocument();
            expect(mockOnJoinSuccess).not.toHaveBeenCalled();
            expect(screen.getByRole('button', { name: 'Create Site' })).not.toBeDisabled();
        });
    });
});
