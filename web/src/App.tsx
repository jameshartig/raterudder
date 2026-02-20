
import React, { useEffect, useState } from 'react';
import { Route, Switch, Redirect, useLocation, Router } from 'wouter';
import { GoogleOAuthProvider } from '@react-oauth/google';
import Header from './components/Header';
import Footer from './components/Footer';
import './App.css';
import { fetchAuthStatus, login, logout, type AuthStatus } from './api';

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
    const [viewSiteOverride, setViewSiteOverride] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);
    const [hasAttemptedFetch, setHasAttemptedFetch] = useState(false);

    const [, navigate] = useLocation();

    useEffect(() => {
        const queryParams = new URLSearchParams(window.location.search);
        const override = queryParams.get('viewSite');
        if (override) {
            setViewSiteOverride(override);
        }
    }, []);

    const applyStatus = (status: AuthStatus, redirectOnLogin: boolean) => {
        setAuthRequired(status.authRequired);
        setLoggedIn(status.loggedIn);
        setClientID(status.clientID);

        const sites = status.siteIDs || [];
        setSiteIDs(sites);

        // Default select first site if not selected or invalid
        if (sites.length > 0 && (!selectedSiteID || !sites.includes(selectedSiteID))) {
            setSelectedSiteID(sites[0]);
        }

        setHasAttemptedFetch(true);

        if (redirectOnLogin && status.loggedIn) {
            navigate('/dashboard');
        }
    };

    // Initial auth check — runs once on mount. Sets loading=true to gate
    // the first render until we know whether the user is authenticated.
    useEffect(() => {
        fetchAuthStatus()
            .then(status => {
                applyStatus(status, false);
            })
            .catch(err => {
                console.error(err);
                setHasAttemptedFetch(true);
            })
            .finally(() => {
                setLoading(false);
            });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    // Re-check auth after login/logout without toggling loading, so child
    // components stay mounted and don't re-fire their own data fetches.
    const checkStatus = (redirectOnLogin = false) => {
        fetchAuthStatus()
            .then(status => {
                applyStatus(status, redirectOnLogin);
            })
            .catch(err => {
                console.error(err);
            });
    };

    const [location] = useLocation();
    const isHome = location === '/';

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

    const effectiveSiteID = viewSiteOverride || selectedSiteID;
    const effectiveSiteIDs = viewSiteOverride && !siteIDs.includes(viewSiteOverride)
        ? [...siteIDs, viewSiteOverride]
        : siteIDs;

    const handleSiteChange = (id: string) => {
        if (viewSiteOverride) setViewSiteOverride(null);
        setSelectedSiteID(id);
    };

    return (
        <AuthWrapper clientID={clientID}>
            <div className={isHome ? "app-container-home" : "app-container"}>
                <Header
                    loggedIn={loggedIn}
                    siteIDs={effectiveSiteIDs}
                    selectedSiteID={effectiveSiteID}
                    onSiteChange={handleSiteChange}
                    onLogout={handleLogout}
                />

                <main className="main-content">
                    {viewSiteOverride && (
                        <div className="admin-site-banner" style={{ background: '#f59e0b', color: '#fff', padding: '0.5rem', textAlign: 'center', fontWeight: 'bold' }}>
                            Admin Mode: Viewing Site {viewSiteOverride}
                        </div>
                    )}
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
                                    {!effectiveSiteID && effectiveSiteIDs.length === 0 ? (
                                        <JoinPage onJoinSuccess={() => checkStatus(true)} />
                                    ) : (
                                        <Dashboard siteID={effectiveSiteID} />
                                    )}
                                </ProtectedRoute>
                            </Route>
                            <Route path="/forecast">
                                <ProtectedRoute loggedIn={loggedIn} loading={loading}>
                                    {!effectiveSiteID && effectiveSiteIDs.length === 0 ? (
                                        <JoinPage onJoinSuccess={() => checkStatus(true)} />
                                    ) : (
                                        <Forecast siteID={effectiveSiteID} />
                                    )}
                                </ProtectedRoute>
                            </Route>
                            <Route path="/settings">
                                <ProtectedRoute loggedIn={loggedIn} loading={loading}>
                                    {!effectiveSiteID && effectiveSiteIDs.length === 0 ? (
                                        <JoinPage onJoinSuccess={() => checkStatus(true)} />
                                    ) : (
                                        <Settings siteID={effectiveSiteID} />
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
