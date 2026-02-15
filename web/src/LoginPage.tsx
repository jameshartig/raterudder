import React from 'react';
import { GoogleLogin } from '@react-oauth/google';
import './LoginPage.css';

interface LoginPageProps {
    onLoginSuccess: (credentialResponse: any) => void;
    onLoginError?: () => void;
    authEnabled: boolean;
    clientID: string;
}

const LoginPage: React.FC<LoginPageProps> = ({ onLoginSuccess, onLoginError, authEnabled, clientID }) => {
    return (
        <div className="login-page">
            <div className="login-card">
                <h1>Raterudder</h1>
                <p>Sign in to manage your home energy.</p>
                <div className="google-btn-wrapper">
                    {authEnabled && clientID ? (
                        <GoogleLogin
                            onSuccess={onLoginSuccess}
                            onError={onLoginError}
                            theme="filled_blue"
                            size="large"
                            text="signin_with"
                            shape="pill"
                        />
                    ) : (
                        <div className="auth-disabled-message">
                            {!authEnabled ? "Authentication is currently disabled." : "Google login is not configured correctly."}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};

export default LoginPage;
