import React, { useState } from 'react';
import { submitFeedback } from '../api';
import './FeedbackButton.css';

interface FeedbackButtonProps {
    siteID: string;
}

const FeedbackButton: React.FC<FeedbackButtonProps> = ({ siteID }) => {
    const [isOpen, setIsOpen] = useState(false);
    const [sentiment, setSentiment] = useState<string>('');
    const [comment, setComment] = useState('');
    const [submitting, setSubmitting] = useState(false);
    const [submitted, setSubmitted] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!sentiment) {
            setError('Please select a sentiment');
            return;
        }

        setSubmitting(true);
        setError(null);
        try {
            await submitFeedback(sentiment, comment, siteID);
            setSubmitted(true);
            setTimeout(() => {
                setIsOpen(false);
                setSubmitted(false);
                setSentiment('');
                setComment('');
            }, 3000);
        } catch (err: any) {
            setError(err.message || 'Failed to submit feedback');
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <div className="feedback-container">
            {isOpen && (
                <div className="feedback-popup card">
                    <button className="close-btn" onClick={() => setIsOpen(false)}>&times;</button>
                    <h3>Feedback</h3>
                    {submitted ? (
                        <div className="feedback-success">Thank you for your feedback!</div>
                    ) : (
                        <form onSubmit={handleSubmit} className="feedback-form">
                            <div className="sentiment-group">
                                <button
                                    type="button"
                                    className={`sentiment-btn ${sentiment === 'happy' ? 'selected' : ''}`}
                                    onClick={() => setSentiment('happy')}
                                >
                                    😀
                                </button>
                                <button
                                    type="button"
                                    className={`sentiment-btn ${sentiment === 'neither' ? 'selected' : ''}`}
                                    onClick={() => setSentiment('neither')}
                                >
                                    😐
                                </button>
                                <button
                                    type="button"
                                    className={`sentiment-btn ${sentiment === 'sad' ? 'selected' : ''}`}
                                    onClick={() => setSentiment('sad')}
                                >
                                    😞
                                </button>
                            </div>
                            <textarea
                                placeholder="Tell us more..."
                                value={comment}
                                onChange={(e) => setComment(e.target.value)}
                                className="feedback-textarea"
                                rows={4}
                            />
                            {error && <div className="feedback-error">{error}</div>}
                            <button type="submit" disabled={submitting || !sentiment} className="btn primary-btn submit-btn">
                                {submitting ? 'Submitting...' : 'Submit'}
                            </button>
                        </form>
                    )}
                </div>
            )}
            <button className="feedback-floating-btn" onClick={() => setIsOpen(!isOpen)}>
                💬
            </button>
        </div>
    );
};

export default FeedbackButton;