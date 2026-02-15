

import { BrowserRouter as Router, Routes, Route, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { GoogleOAuthProvider } from '@react-oauth/google';
import ActionList from './ActionList';
import Settings from './Settings';
import Modeling from './Modeling';
import LandingPage from './LandingPage';
import LoginPage from './LoginPage';
import JoinPage from './JoinPage';
import Header from './Header'; // Renamed import
import './App.css';
import React, { useEffect, useState } from 'react';
import { fetchAuthStatus, login, logout } from './api';

// Protected Route Wrapper
const ProtectedRoute = ({ children, loggedIn, loading }: { children: React.ReactElement, loggedIn: boolean, loading: boolean }) => {

    if (loading) {
        return <div className="loading-screen">Loading...</div>; // Could be a nicer spinner
    }

    if (!loggedIn) {
         // Redirect them to the login page, but save the current location they were trying to go to
        const location = useLocation();
        return <Navigate to="/login" state={{ from: location }} replace />;
    }

    return children;
};

function AppContent() {
    const [isAdmin, setIsAdmin] = useState(false);
    const [authRequired, setAuthRequired] = useState(false);
    const [loggedIn, setLoggedIn] = useState(false);
    const [clientID, setClientID] = useState("");
    const [siteIDs, setSiteIDs] = useState<string[]>([]);
    const [selectedSiteID, setSelectedSiteID] = useState<string>("");
    const [loading, setLoading] = useState(true);
    const [hasAttemptedFetch, setHasAttemptedFetch] = useState(false);

    // const location = useLocation();
    const navigate = useNavigate();

    const checkStatus = (redirectOnLogin = false) => {
        setLoading(true);
        fetchAuthStatus()
            .then(status => {
                setIsAdmin(status.isAdmin);
                setAuthRequired(status.authRequired);
                setLoggedIn(status.loggedIn);
                setClientID(status.clientID);

                const sites = status.siteIDs || [];
                setSiteIDs(sites);

                // Default select first site if not selected or invalid
                if (sites.length > 0 && (!selectedSiteID || !sites.includes(selectedSiteID))) {
                    setSelectedSiteID(sites[0]);
                }

                setLoading(false);
                setHasAttemptedFetch(true);

                if (redirectOnLogin && status.loggedIn) {
                    navigate('/dashboard');
                }
            })
            .catch(err => {
                console.error(err);
                setLoading(false);
                setHasAttemptedFetch(true);
            });
    };

    const location = useLocation();

    useEffect(() => {
        if (location.pathname === '/') {
            setLoading(false);
            return;
        }
        setLoading(true);
        checkStatus();
    }, [location.pathname]);

    const handleLoginSuccess = async (credentialResponse: any) => {
        try {
            if (credentialResponse.credential) {
                await login(credentialResponse.credential);
                checkStatus(true); // Redirect to dashboard on success
            }
        } catch (err) {
            console.error("Login failed", err);
        }
    };

    const handleLogout = async () => {
        try {
            await logout();
            checkStatus();
            setSiteIDs([]);
            setSelectedSiteID("");
            navigate('/'); // Go back to landing page on logout
        } catch (err) {
            console.error("Logout failed", err);
        }
    };

     // Wrapper to provide GoogleOAuth context only when we have a ClientID
     const AuthWrapper = ({ children }: { children: React.ReactNode }) => {
        if (clientID) {
            return (
                <GoogleOAuthProvider clientId={clientID}>
                    {children}
                </GoogleOAuthProvider>
            );
        }
        return <>{children}</>;
    };

    const isHome = location.pathname === '/';
    const showLoading = (loading || (!isHome && !hasAttemptedFetch)) && !isHome;

    return (
        <AuthWrapper>
            <div className="app-container">
                {/* Header is always visible, but adapts based on login state */}
                <Header
                    loggedIn={loggedIn}
                    siteIDs={siteIDs}
                    selectedSiteID={selectedSiteID}
                    onSiteChange={setSelectedSiteID}
                    onLogout={handleLogout}
                />

                <main className="main-content">
                    {showLoading ? (
                        <div className="loading-screen">Loading...</div>
                    ) : (
                        <Routes>
                            <Route path="/" element={<LandingPage />} />
                            <Route path="/login" element={
                                loggedIn ? <Navigate to="/dashboard" replace /> :
                                <LoginPage
                                    onLoginSuccess={handleLoginSuccess}
                                    onLoginError={() => console.log('Login Failed')}
                                    authEnabled={authRequired}
                                    clientID={clientID}
                                />
                            } />

                            {/* Protected Routes */}
                            <Route path="/dashboard" element={
                                <ProtectedRoute loggedIn={loggedIn} loading={loading}>
                                    {siteIDs.length === 0 ? (
                                        <JoinPage onJoinSuccess={() => checkStatus(true)} />
                                    ) : (
                                        <ActionList siteID={selectedSiteID} />
                                    )}
                                </ProtectedRoute>
                            } />
                            <Route path="/modeling" element={
                                <ProtectedRoute loggedIn={loggedIn} loading={loading}>
                                    {siteIDs.length === 0 ? (
                                        <JoinPage onJoinSuccess={() => checkStatus(true)} />
                                    ) : (
                                        <Modeling siteID={selectedSiteID} />
                                    )}
                                </ProtectedRoute>
                            } />
                            <Route path="/settings" element={
                                <ProtectedRoute loggedIn={loggedIn} loading={loading}>
                                    {siteIDs.length === 0 ? (
                                        <JoinPage onJoinSuccess={() => checkStatus(true)} />
                                    ) : (
                                        <Settings isAdmin={isAdmin} siteID={selectedSiteID} />
                                    )}
                                </ProtectedRoute>
                            } />

                            {/* Fallback */}
                            <Route path="*" element={<Navigate to="/" replace />} />
                        </Routes>
                    )}
                </main>
            </div>
        </AuthWrapper>
    );
}

function App() {
  return (
    <Router>
        <AppContent />
    </Router>
  );
}

export default App;
