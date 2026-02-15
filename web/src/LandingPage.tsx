import React from 'react';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, LineChart, Line } from 'recharts';
import './LandingPage.css';

const LandingPage: React.FC = () => {
    // Fake data for charts
    const solarData = Array.from({ length: 24 }, (_, i) => ({
        name: `${i}:00`,
        uv: i >= 6 && i <= 18 ? Math.sin((i - 6) * Math.PI / 12) * 4000 : 0,
    }));

    const usageData = Array.from({ length: 24 }, (_, i) => ({
        name: `${i}:00`,
        usage: 500 + Math.random() * 1000 + (i > 17 ? 1000 : 0),
    }));

    const batteryData = Array.from({ length: 24 }, (_, i) => {
        let level = 20;
        if (i > 8 && i < 16) level = 80 + Math.random() * 10;
        else if (i >= 16) level = 90 - (i - 16) * 5;
        else level = 40 - i * 2;
        return { name: `${i}:00`, level: Math.max(0, level) };
    });

    return (
        <div className="landing-page">
            <section className="hero">
                <h1>Navigate Utility Prices</h1>
                <p>Hands-off, 24/7 intelligent energy management for your home.</p>
            </section>

            <section className="features">
                <div className="feature-card">
                    <h3>Minimize Grid Use</h3>
                    <p>Maximize solar generation and battery usage.</p>
                </div>
                <div className="feature-card">
                    <h3>Arbitrage Prices</h3>
                    <p>Buy energy when it's cheap, sell it when it's expensive.</p>
                </div>
                <div className="feature-card">
                    <h3>Smart Modeling</h3>
                    <p>We model your solar generation and home usage to make intelligent decisions.</p>
                </div>
                <div className="feature-card">
                    <h3>Fully Automated</h3>
                    <p>RateRudder works 24/7 in the background. Set it and forget it.</p>
                </div>
            </section>

            <section className="graphs-section">
                <h2>Intelligent Modeling</h2>
                <div className="charts-grid">
                    <div className="chart-container">
                        <h3>Solar Generation Model</h3>
                        <ResponsiveContainer width="100%" height={200}>
                            <AreaChart data={solarData}>
                                <defs>
                                    <linearGradient id="colorSolar" x1="0" y1="0" x2="0" y2="1">
                                        <stop offset="5%" stopColor="#8884d8" stopOpacity={0.8}/>
                                        <stop offset="95%" stopColor="#8884d8" stopOpacity={0}/>
                                    </linearGradient>
                                </defs>
                                <CartesianGrid strokeDasharray="3 3" />
                                <XAxis dataKey="name" />
                                <YAxis />
                                <Tooltip />
                                <Area type="monotone" dataKey="uv" stroke="#8884d8" fillOpacity={1} fill="url(#colorSolar)" />
                            </AreaChart>
                        </ResponsiveContainer>
                    </div>

                    <div className="chart-container">
                        <h3>Home Usage Model</h3>
                        <ResponsiveContainer width="100%" height={200}>
                            <AreaChart data={usageData}>
                                <defs>
                                    <linearGradient id="colorUsage" x1="0" y1="0" x2="0" y2="1">
                                        <stop offset="5%" stopColor="#82ca9d" stopOpacity={0.8}/>
                                        <stop offset="95%" stopColor="#82ca9d" stopOpacity={0}/>
                                    </linearGradient>
                                </defs>
                                <CartesianGrid strokeDasharray="3 3" />
                                <XAxis dataKey="name" />
                                <YAxis />
                                <Tooltip />
                                <Area type="monotone" dataKey="usage" stroke="#82ca9d" fillOpacity={1} fill="url(#colorUsage)" />
                            </AreaChart>
                        </ResponsiveContainer>
                    </div>

                    <div className="chart-container">
                        <h3>Battery Status Optimization</h3>
                        <ResponsiveContainer width="100%" height={200}>
                            <LineChart data={batteryData}>
                                <CartesianGrid strokeDasharray="3 3" />
                                <XAxis dataKey="name" />
                                <YAxis />
                                <Tooltip />
                                <Line type="monotone" dataKey="level" stroke="#ff7300" strokeWidth={2} />
                            </LineChart>
                        </ResponsiveContainer>
                    </div>
                </div>
            </section>

            <footer className="landing-footer">
                <p>Open Source Project. Contribute on <a href="https://github.com/jameshartig/raterudder" target="_blank" rel="noopener noreferrer">GitHub</a>.</p>
            </footer>
        </div>
    );
};

export default LandingPage;
