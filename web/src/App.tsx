
import React, { useEffect, useState } from 'react';
import { Route, Switch, Redirect, useLocation, Router } from 'wouter';
import { GoogleOAuthProvider } from '@react-oauth/google';
import Header from './components/Header';
import Footer from './components/Footer';
import './App.css';
import { fetchAuthStatus, login, logout } from './api';

import LandingPage from './pages/LandingPage';
import Dashboard from './pages/Dashboard';
import Settings from './pages/Settings';
import Forecast from './pages/Forecast';
import LoginPage from './pages/LoginPage';
import JoinPage from './pages/JoinPage';
import PrivacyPolicy from './pages/PrivacyPolicy';
import TermsOfService from './pages/TermsOfService';

// Protected Route Wrapper
const ProtectedRoute = ({ children, loggedIn, loading }: { children: React.ReactElement, loggedIn: boolean, loading: boolean }) => {

    if (loading) {
        return <div className="loading-screen">Loading...</div>; // Could be a nicer spinner
    }

    if (!loggedIn) {
         // Redirect them to the login page, but save the current location they were trying to go to
        const [location] = useLocation();
        return <Redirect to={`/login?from=${encodeURIComponent(location)}`} replace />;
    }

    return children;
};

function AppContent() {
    const [authRequired, setAuthRequired] = useState(false);
    const [loggedIn, setLoggedIn] = useState(false);
    const [clientID, setClientID] = useState("");
    const [siteIDs, setSiteIDs] = useState<string[]>([]);
    const [selectedSiteID, setSelectedSiteID] = useState<string>("");
    const [loading, setLoading] = useState(true);
    const [hasAttemptedFetch, setHasAttemptedFetch] = useState(false);

    // const location = useLocation();
    const [, navigate] = useLocation();

    const checkStatus = (redirectOnLogin = false) => {
        setLoading(true);
        fetchAuthStatus()
            .then(status => {
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

    const [location] = useLocation();
    const isHome = location === '/';

    useEffect(() => {
        if (isHome) {
            setLoading(false);
            return;
        }
        setLoading(true);
        checkStatus();
    }, [location, isHome]);

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

    const showLoading = (loading || (!isHome && !hasAttemptedFetch)) && !isHome;

    return (
        <AuthWrapper clientID={clientID}>
            <div className={isHome ? "app-container-home" : "app-container"}>
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
                        <Switch>
                            <Route path="/" component={LandingPage} />
                            <Route path="/privacy" component={PrivacyPolicy} />
                            <Route path="/terms" component={TermsOfService} />
                            <Route path="/login">
                                {loggedIn ? <Redirect to="/dashboard" replace /> :
                                <LoginPage
                                    onLoginSuccess={handleLoginSuccess}
                                    onLoginError={() => console.log('Login Failed')}
                                    authEnabled={authRequired}
                                    clientID={clientID}
                                />}
                            </Route>

                            {/* Protected Routes */}
                            <Route path="/dashboard">
                                <ProtectedRoute loggedIn={loggedIn} loading={loading}>
                                    {siteIDs.length === 0 ? (
                                        <JoinPage onJoinSuccess={() => checkStatus(true)} />
                                    ) : (
                                        <Dashboard siteID={selectedSiteID} />
                                    )}
                                </ProtectedRoute>
                            </Route>
                            <Route path="/forecast">
                                <ProtectedRoute loggedIn={loggedIn} loading={loading}>
                                    {siteIDs.length === 0 ? (
                                        <JoinPage onJoinSuccess={() => checkStatus(true)} />
                                    ) : (
                                        <Forecast siteID={selectedSiteID} />
                                    )}
                                </ProtectedRoute>
                            </Route>
                            <Route path="/settings">
                                <ProtectedRoute loggedIn={loggedIn} loading={loading}>
                                    {siteIDs.length === 0 ? (
                                        <JoinPage onJoinSuccess={() => checkStatus(true)} />
                                    ) : (
                                        <Settings siteID={selectedSiteID} />
                                    )}
                                </ProtectedRoute>
                            </Route>

                            {/* Fallback */}
                            <Route>
                                <Redirect to="/" replace />
                            </Route>
                        </Switch>
                    )}
                </main>

                <Footer />
            </div>
        </AuthWrapper>
    );
}

// Wrapper to provide GoogleOAuth context only when we have a ClientID
const AuthWrapper = ({ children, clientID }: { children: React.ReactNode, clientID: string }) => {
    if (clientID) {
        return (
            <GoogleOAuthProvider clientId={clientID}>
                {children}
            </GoogleOAuthProvider>
        );
    }
    return <>{children}</>;
};

function App() {
  return (
    <Router>
        <AppContent />
    </Router>
  );
}

export default App;
