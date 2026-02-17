import React from 'react';
import { Link } from 'wouter';
import './Header.css';

interface HeaderProps {
    loggedIn: boolean;
    siteIDs: string[];
    selectedSiteID: string;
    onSiteChange: (siteID: string) => void;
    onLogout: () => void;
}

const Header: React.FC<HeaderProps> = ({ loggedIn, siteIDs, selectedSiteID, onSiteChange, onLogout }) => {
    const [isMenuOpen, setIsMenuOpen] = React.useState(false);

    const toggleMenu = () => {
        setIsMenuOpen(!isMenuOpen);
    };

    return (
        <header className={`raterudder-header ${loggedIn ? 'logged-in' : 'logged-out'}`}>
            <div className="content-container header-inner">
                <div className="header-left">
                    <Link to="/" className="brand-logo" onClick={() => setIsMenuOpen(false)}>
                        <img src="/logo.svg" alt="RateRudder Logo" className="header-logo-img" />
                        RateRudder
                    </Link>
                    {loggedIn && siteIDs.length > 1 && (
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
                </div>

                {loggedIn && (
                    <button className="mobile-menu-toggle" onClick={toggleMenu} aria-label="Toggle navigation">
                        <span className="hamburger-line"></span>
                        <span className="hamburger-line"></span>
                        <span className="hamburger-line"></span>
                    </button>
                )}

                <div className={`header-content ${isMenuOpen ? 'open' : ''}`}>
                    <nav className="header-nav">
                        {loggedIn ? (
                            <>
                                <Link to="/dashboard" className="nav-link" onClick={() => setIsMenuOpen(false)}>Dashboard</Link>
                                <Link to="/forecast" className="nav-link" onClick={() => setIsMenuOpen(false)}>Forecast</Link>
                                <Link to="/settings" className="nav-link" onClick={() => setIsMenuOpen(false)}>Settings</Link>
                            </>
                        ) : (
                            <span className="nav-empty-spacer"></span>
                        )}
                    </nav>

                    <div className="header-right">
                        {loggedIn ? (
                            <button onClick={() => { onLogout(); setIsMenuOpen(false); }} className="logout-link">Log Out</button>
                        ) : (
                            <Link to="/login" className="login-link" onClick={() => setIsMenuOpen(false)}>Login</Link>
                        )}
                    </div>
                </div>
            </div>
        </header>
    );
};

export default Header;
