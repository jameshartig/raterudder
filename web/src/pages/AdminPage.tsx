import React, { useEffect, useState } from 'react';
import { listSites, listFeedback } from '../api';
import type { AdminSite, Feedback } from '../api';
import { Separator } from '@base-ui/react/separator';
import './AdminPage.css';

const AdminPage: React.FC = () => {
    const [sites, setSites] = useState<AdminSite[]>([]);
    const [feedbacks, setFeedbacks] = useState<Feedback[]>([]);
    const [loadingSites, setLoadingSites] = useState(true);
    const [loadingFeedback, setLoadingFeedback] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [feedbackError, setFeedbackError] = useState<string | null>(null);

    useEffect(() => {
        listSites()
            .then((data) => {
                setSites(data || []);
                setError(null);
            })
            .catch((err) => {
                console.error("Failed to list sites:", err);
                setError(err.message || 'Failed to list sites. Ensure you have admin access.');
            })
            .finally(() => {
                setLoadingSites(false);
            });

        listFeedback()
            .then((data) => {
                setFeedbacks(data || []);
                setFeedbackError(null);
            })
            .catch((err) => {
                console.error("Failed to list feedback:", err);
                setFeedbackError(err.message || 'Failed to list feedback.');
            })
            .finally(() => {
                setLoadingFeedback(false);
            });
    }, []);

    if (loadingSites || loadingFeedback) {
        return <div className="loading-screen">Loading Admin Data...</div>;
    }

    if (error) {
        return (
            <div className="content-container admin-page">
                <div className="admin-error">{error}</div>
            </div>
        );
    }

    return (
        <div className="content-container admin-page">
            <div className="admin-header">
                <h1>Site List</h1>
            </div>

            <Separator className="admin-separator" />

            <div className="admin-list">
                {sites.map((site) => (
                    <div key={site.id} className="card admin-site-card">
                        <div className="admin-site-info">
                            <h3 className="admin-site-id">{site.id}</h3>
                            {site.lastAction && (
                                <div className="admin-site-action">
                                    Last Action: {site.lastAction.description} @ {new Date(site.lastAction.timestamp).toLocaleString()}<br/>
                                    Battery SOC: {site.lastAction.systemStatus?.batterySOC?.toFixed(1) || '0'}%
                                </div>
                            )}
                        </div>
                        <a href={`/dashboard?viewSite=${site.id}`} className="btn admin-primary-btn">
                            View Dashboard
                        </a>
                    </div>
                ))}
            </div>

            <div className="admin-header" style={{ marginTop: '2rem' }}>
                <h1>Feedback</h1>
            </div>

            <Separator className="admin-separator" />

            {feedbackError ? (
                <div className="admin-error">{feedbackError}</div>
            ) : (
                <div className="admin-list">
                    {feedbacks.map((fb, idx) => (
                        <div key={idx} className="card admin-site-card">
                            <div className="admin-site-info">
                                <h3 className="admin-site-id">
                                    {fb.sentiment === 'happy' ? '😀' : fb.sentiment === 'sad' ? '😞' : '😐'} {fb.siteID}
                                </h3>
                                <div className="admin-site-action">
                                    User: {fb.userID}<br/>
                                    Time: {new Date(fb.time).toLocaleString()}<br/>
                                    Comment: {fb.comment}
                                </div>
                            </div>
                        </div>
                    ))}
                    {feedbacks.length === 0 && <div>No feedback yet.</div>}
                </div>
            )}

        </div>
    );
};

export default AdminPage;