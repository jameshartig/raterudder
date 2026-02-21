import React from 'react';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, ResponsiveContainer, LineChart, Line } from 'recharts';
import './LandingPage.css';

const LandingPage: React.FC = () => {
    // Fake data for charts
    const solarData = Array.from({ length: 24 }, (_, i) => ({
        name: `${i}:00`,
        uv: i >= 6 && i <= 18 ? Math.sin((i - 6) * Math.PI / 12) * 7 : 0,
    }));

    const usageData = Array.from({ length: 24 }, (_, i) => ({
        name: `${i}:00`,
        usage: .5 + Math.random() + (i > 17 ? 1 : 0),
    }));

    const batteryData = Array.from({ length: 24 }, (_, i) => {
        let level = 20;
        if (i > 8 && i < 16) level = 80 + Math.random() * 10;
        else if (i >= 16) level = 90 - (i - 16) * 5;
        else level = 40 - i * 2;
        return { name: `${i}:00`, level: Math.max(0, level) };
    });

    const [isMobile, setIsMobile] = React.useState(window.innerWidth < 768);

    React.useEffect(() => {
        const handleResize = () => setIsMobile(window.innerWidth < 768);
        window.addEventListener('resize', handleResize);
        return () => window.removeEventListener('resize', handleResize);
    }, []);

    const [activeAccordion, setActiveAccordion] = React.useState<number | null>(null);

    const toggleAccordion = (index: number) => {
        setActiveAccordion(activeAccordion === index ? null : index);
    };

    const faqData = [
        {
            question: "How does RateRudder save me money?",
            answer: "RateRudder intelligently manages your battery to only charge when electricity is cheapest and only when charging is necessary."
        },
        {
            question: "Do I need specific hardware?",
            answer: "Currently, only FranklinWH aPower batteries are supported. We're looking for testers to help us add support for more battery types soon."
        },
        {
            question: "Which utilities are supported?",
            answer: "Currently only ComEd is supported but new utilities are being added quickly based on demand."
        },
        {
            question: "How much does it cost?",
            answer: "RateRudder is currently free for early adopters."
        }
    ];

    const JOIN_FORM_URL = import.meta.env.VITE_JOIN_FORM_URL;

    const chartMargin = { top: 10, right: 10, left: 0, bottom: 0 };
    const axisStyle = { fontSize: isMobile ? 10 : 12, fontFamily: 'Inter, sans-serif' };
    const yAxisWidth = isMobile ? 30 : 40;

    return (
        <div className="landing-page">
            <section className="hero-section">
                <div className="content-container hero-layout">
                    <div className="hero-content">
                        {JOIN_FORM_URL && (
                            <div className="badge">Limited Beta Now Open</div>
                        )}
                        <h1>Your Battery, Just <span className="highlight">Smarter.</span></h1>
                        <p>
                            RateRudder transforms your home battery into a powerful financial asset.
                            Intelligently managing your energy to buy low, sell high, and slash your bill—all while you sleep.
                        </p>
                        {JOIN_FORM_URL && (
                            <div className="cta-wrapper">
                                <a href={JOIN_FORM_URL} target="_blank" rel="noopener noreferrer" className="cta-button">
                                    Request Early Access
                                </a>
                                <span className="cta-note">Tell us about your battery and utility to help skip the queue.</span>

                            </div>
                        )}
                    </div>
                    <div className="hero-visual">
                        <div className="pulse-circle"></div>
                        <div className="floating-card">
                            <span>Estimated Savings</span>
                            <strong>$12.84</strong>
                            <small>This Month*</small>
                            <div className="status-indicator">
                                <span className="dot"></span> Optimized by RateRudder
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            <section className="features-strip">
                <div className="content-container">
                    <div className="features-grid">
                        <div className="feature-item">
                            <div className="icon">⚡</div>
                            <h3>Automated Arbitrage</h3>
                            <p>Our algorithms track utility rates in real-time, charging your battery when prices bottom out and discharging when they peak.</p>
                        </div>
                        <div className="feature-item">
                            <div className="icon">🛡️</div>
                            <h3>Grid Independence</h3>
                            <p>Maximize your solar self-consumption and insulate your home from rising grid costs and peak-hour surcharges.</p>
                        </div>
                        <div className="feature-item">
                            <div className="icon">🧠</div>
                            <h3>Predictive Intelligence</h3>
                            <p>RateRudder learns your home's unique energy footprint and solar generation patterns to optimize for the days ahead.</p>
                        </div>
                        <div className="feature-item">
                            <div className="icon">🚀</div>
                            <h3>Set & Forget</h3>
                            <p>Once configured, RateRudder works 24/7 in the background to secure your savings automatically.</p>
                        </div>
                    </div>
                </div>
            </section>

            <section className="live-demo-section">
                <div className="content-container">
                    <div className="section-header">
                        <h2>Intelligent Energy Forecast</h2>
                    </div>

                    <div className="charts-grid">
                        <div className="chart-card">
                            <h3>Solar Generation</h3>
                            <div className="chart-wrapper">
                                <ResponsiveContainer width="100%" height="100%">
                                    <AreaChart data={solarData} margin={chartMargin}>
                                        <defs>
                                            <linearGradient id="colorSolar" x1="0" y1="0" x2="0" y2="1">
                                                <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.8}/>
                                                <stop offset="95%" stopColor="#f59e0b" stopOpacity={0}/>
                                            </linearGradient>
                                        </defs>
                                        <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e5e7eb" />
                                        <XAxis dataKey="name" tick={axisStyle} stroke="#9ca3af" axisLine={false} tickLine={false} />
                                        <YAxis tick={axisStyle} width={yAxisWidth} stroke="#9ca3af" axisLine={false} tickLine={false} />
                                        <Area type="monotone" dataKey="uv" stroke="#f59e0b" strokeWidth={2} fillOpacity={1} fill="url(#colorSolar)" />
                                    </AreaChart>
                                </ResponsiveContainer>
                            </div>
                        </div>

                        <div className="chart-card">
                            <h3>Home Usage</h3>
                            <div className="chart-wrapper">
                                <ResponsiveContainer width="100%" height="100%">
                                    <AreaChart data={usageData} margin={chartMargin}>
                                        <defs>
                                            <linearGradient id="colorUsage" x1="0" y1="0" x2="0" y2="1">
                                                <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.8}/>
                                                <stop offset="95%" stopColor="#3b82f6" stopOpacity={0}/>
                                            </linearGradient>
                                        </defs>
                                        <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e5e7eb" />
                                        <XAxis dataKey="name" tick={axisStyle} stroke="#9ca3af" axisLine={false} tickLine={false} />
                                        <YAxis tick={axisStyle} width={yAxisWidth} stroke="#9ca3af" axisLine={false} tickLine={false} />
                                        <Area type="monotone" dataKey="usage" stroke="#3b82f6" strokeWidth={2} fillOpacity={1} fill="url(#colorUsage)" />
                                    </AreaChart>
                                </ResponsiveContainer>
                            </div>
                        </div>

                        <div className="chart-card full-width">
                            <h3>Battery Capacity</h3>
                            <div className="chart-wrapper">
                                <ResponsiveContainer width="100%" height="100%">
                                    <LineChart data={batteryData} margin={chartMargin}>
                                        <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e5e7eb" />
                                        <XAxis dataKey="name" tick={axisStyle} stroke="#9ca3af" axisLine={false} tickLine={false} />
                                        <YAxis tick={axisStyle} width={yAxisWidth} stroke="#9ca3af" axisLine={false} tickLine={false} />
                                        <Line type="monotone" dataKey="level" stroke="#10b981" strokeWidth={3} dot={false} activeDot={{ r: 6 }} />
                                    </LineChart>
                                </ResponsiveContainer>
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            <section className="faq-section">
                <div className="content-container">
                    <div className="section-header">
                        <h2>Frequently Asked Questions</h2>
                    </div>
                    <div className="faq-container">
                        {faqData.map((item, index) => (
                            <div
                                key={index}
                                className={`faq-item ${activeAccordion === index ? 'active' : ''}`}
                                onClick={() => toggleAccordion(index)}
                            >
                                <div className="faq-question">
                                    <span>{item.question}</span>
                                    <span className="toggle-icon">{activeAccordion === index ? '−' : '+'}</span>
                                </div>
                                <div className="faq-answer">
                                    <p>{item.answer}</p>
                                </div>
                            </div>
                        ))}
                    </div>
                    <p className="marketing-disclaimer">
                        *Actual savings vary by utility plan, battery capacity, and household usage.
                    </p>
                </div>
            </section>
        </div>

    );
};

export default LandingPage;
