import { render, screen } from '@testing-library/react';
import LoginPage from './LoginPage';
import { describe, it, expect, vi } from 'vitest';

// Mock GoogleLogin
vi.mock('@react-oauth/google', () => ({
    GoogleLogin: () => <button>Mock Google Login</button>,
}));

describe('LoginPage Component', () => {
    it('renders correctly', () => {
        render(<LoginPage onLoginSuccess={vi.fn()} onLoginError={vi.fn()} authEnabled={true} clientID="test-id" />);

        expect(screen.getByText('Raterudder')).toBeInTheDocument();
        expect(screen.getByText('Sign in to manage your home energy.')).toBeInTheDocument();
        expect(screen.getByText('Mock Google Login')).toBeInTheDocument();
    });

    it('renders disabled message when auth is disabled', () => {
        render(<LoginPage onLoginSuccess={vi.fn()} onLoginError={vi.fn()} authEnabled={false} clientID="" />);
        expect(screen.getByText('Authentication is currently disabled.')).toBeInTheDocument();
    });
});
