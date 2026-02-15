import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import './Header.css';

interface HeaderProps {
    loggedIn: boolean;
    siteIDs: string[];
    selectedSiteID: string;
    onSiteChange: (siteID: string) => void;
    onLogout: () => void;
}

const Header: React.FC<HeaderProps> = ({ loggedIn, siteIDs, selectedSiteID, onSiteChange, onLogout }) => {
    const location = useLocation();
    const isHomePage = location.pathname === '/';

    return (
        <header className="raterudder-header">
            <div className="header-left">
                <Link to="/" className="brand-logo">RateRudder</Link>
                {!isHomePage && loggedIn && siteIDs.length > 1 && (
                    <select
                        value={selectedSiteID}
                        onChange={(e) => onSiteChange(e.target.value)}
                        className="site-selector-header"
                    >
                        {siteIDs.map(id => (
                            <option key={id} value={id}>{id}</option>
                        ))}
                    </select>
                )}
                {!isHomePage && loggedIn && siteIDs.length === 1 && siteIDs[0] !== "none" && (
                     <span className="site-name-header">({siteIDs[0]})</span>
                )}
            </div>

            <nav className="header-nav">
                {!isHomePage && loggedIn ? (
                    <>
                        <Link to="/dashboard" className="nav-link">History</Link>
                        <Link to="/modeling" className="nav-link">Model</Link>
                        <Link to="/settings" className="nav-link">Settings</Link>
                    </>
                ) : (
                    <span className="nav-empty-spacer"></span>
                )}
            </nav>

            <div className="header-right">
                {loggedIn ? (
                    <button onClick={onLogout} className="logout-link">Log Out</button>
                ) : (
                    <Link to="/login" className="login-link">Login</Link>
                )}
            </div>
        </header>
    );
};

export default Header;
