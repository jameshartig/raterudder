import React from 'react';
import { GoogleLogin } from '@react-oauth/google';
import { Link } from 'wouter';
import { Separator } from '@base-ui/react/separator';
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
                            auto_select
                            use_fedcm_for_prompt
                        />
                    ) : (
                        <div className="auth-disabled-message">
                            {!authEnabled ? "Authentication is currently disabled." : "Google login is not configured correctly."}
                        </div>
                    )}
                </div>
                <div className="login-footer">
                    <Link to="/privacy">Privacy Policy</Link>
                    <Separator className="separator" />
                    <Link to="/terms">Terms of Service</Link>
                </div>
            </div>
        </div>
    );
};

export default LoginPage;
