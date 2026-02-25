import { useState } from 'react';
import { Link } from 'wouter';
import { Field } from '@base-ui/react/field';
import { createSite } from '../api';
import './SitePages.css';

export interface NewSitePageProps {
    onJoinSuccess: () => void;
}

const NewSitePage = ({ onJoinSuccess }: NewSitePageProps) => {
    const [createName, setCreateName] = useState('');
    const [error, setError] = useState<string | null>(null);
    const [isSubmitting, setIsSubmitting] = useState(false);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError(null);

        if (!createName.trim()) {
            setError('Site Name is required');
            return;
        }

        setIsSubmitting(true);
        try {
            await createSite(createName.trim());
            onJoinSuccess();
        } catch (err: any) {
            setError(err.message || 'Failed to create site');
            setIsSubmitting(false);
        }
    };

    return (
        <div className="join-page">
            <div className="join-card">
                <h1 className="join-title">Create a New Site</h1>
                <p className="join-subtitle">
                    A site represents a set of solar panels, batteries, and an
                    energy storage system (ESS). Each site is monitored and
                    optimized independently.
                </p>

                <form onSubmit={handleSubmit} className="join-form">
                    <Field.Root className="join-field">
                        <Field.Label htmlFor="create-name">Site Name</Field.Label>
                        <Field.Control
                            id="create-name"
                            className="join-input"
                            value={createName}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setCreateName(e.target.value)}
                            disabled={isSubmitting}
                            required
                        />
                    </Field.Root>

                    {error && <div className="join-error">{error}</div>}

                    <p className="join-consent">
                        By creating, you agree to our{' '}
                        <a href="/terms" target="_blank" rel="noopener noreferrer">Terms of Service</a> and{' '}
                        <a href="/privacy" target="_blank" rel="noopener noreferrer">Privacy Policy</a>.
                    </p>

                    <button
                        type="submit"
                        disabled={isSubmitting || !createName.trim()}
                        className="join-submit"
                    >
                        {isSubmitting ? 'Creating...' : 'Create Site'}
                    </button>
                </form>
            </div>

            <p className="join-alternate-link">
                <Link href="/join-site">Already have a site? Join it.</Link>
            </p>
        </div>
    );
};

export default NewSitePage;

