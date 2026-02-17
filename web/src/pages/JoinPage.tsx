import React, { useState } from 'react';
import { Link } from 'wouter';
import { joinSite } from '../api';
import './JoinPage.css';

interface JoinPageProps {
    onJoinSuccess: () => void;
}

const JoinPage: React.FC<JoinPageProps> = ({ onJoinSuccess }) => {
    const [siteID, setSiteID] = useState('');
    const [inviteCode, setInviteCode] = useState('');
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError('');

        if (!siteID.trim() || !inviteCode.trim()) {
            setError('Both fields are required.');
            return;
        }

        setLoading(true);
        try {
            await joinSite(siteID.trim(), inviteCode.trim());
            onJoinSuccess();
        } catch (err: any) {
            setError(err.message || 'Failed to join site');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="join-page">
            <div className="join-card">
                <h1>Join a Site</h1>
                <p>Enter the Site ID and Invite Code provided by the site owner.</p>

                <form onSubmit={handleSubmit} className="join-form">
                    <div className="join-field">
                        <label htmlFor="join-site-id">Site ID</label>
                        <input
                            id="join-site-id"
                            type="text"
                            value={siteID}
                            onChange={(e) => setSiteID(e.target.value)}
                            placeholder="e.g. my-home"
                            autoComplete="off"
                            disabled={loading}
                        />
                    </div>

                    <div className="join-field">
                        <label htmlFor="join-invite-code">Invite Code</label>
                        <input
                            id="join-invite-code"
                            type="text"
                            value={inviteCode}
                            onChange={(e) => setInviteCode(e.target.value)}
                            placeholder="Enter invite code"
                            autoComplete="off"
                            disabled={loading}
                        />
                    </div>

                    {error && <div className="join-error">{error}</div>}

                    <p className="join-consent">
                        By joining, you agree to our{' '}
                        <Link to="/terms">Terms of Service</Link> and{' '}
                        <Link to="/privacy">Privacy Policy</Link>.
                    </p>

                    <button
                        type="submit"
                        className="join-submit"
                        disabled={loading || !siteID.trim() || !inviteCode.trim()}
                    >
                        {loading ? 'Joining…' : 'Join Site'}
                    </button>
                </form>
            </div>
        </div>
    );
};

export default JoinPage;
