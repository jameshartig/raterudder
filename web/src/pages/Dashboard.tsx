import React, { useEffect, useState, useMemo } from 'react';
import { useLocation, useSearch } from 'wouter';
import { fetchActions, fetchSavings, type Action, type SavingsStats, BatteryMode, SolarMode, ActionReason } from '../api';

const getBatteryModeLabel = (mode: number) => {
    switch (mode) {
        case BatteryMode.Standby: return 'Hold Battery';
        case BatteryMode.ChargeAny: return 'Charge From Solar+Grid';
        case BatteryMode.ChargeSolar: return 'Charge From Solar';
        case BatteryMode.Load: return 'Use Battery';
        case BatteryMode.NoChange: return 'No Change';
        default: return 'Unknown';
    }
};

const getBatteryModeClass = (mode: number) => {
    switch (mode) {
        case BatteryMode.Standby: return 'standby';
        case BatteryMode.ChargeAny: return 'charge_any';
        case BatteryMode.ChargeSolar: return 'charge_solar';
        case BatteryMode.Load: return 'load';
        case BatteryMode.NoChange: return 'no_change';
        default: return 'unknown';
    }
};

const getSolarModeLabel = (mode: number) => {
    switch (mode) {
        case SolarMode.NoExport: return 'No Export';
        case SolarMode.Any: return 'Any';
        case SolarMode.NoChange: return 'No Change';
        default: return 'Unknown';
    }
};

const getSolarModeClass = (mode: number) => {
    switch (mode) {
        case SolarMode.NoExport: return 'no_export';
        case SolarMode.Any: return 'any';
        case SolarMode.NoChange: return 'no_change';
        default: return 'unknown';
    }
};

const formatPrice = (dollars: number) => `$${dollars.toFixed(3)}/kWh`;

const formatTime = (ts: string) => {
    try {
        return new Date(ts).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
    } catch {
        return ts;
    }
};

const getReasonText = (action: Action): string => {
    const reason = action.reason;
    if (!reason) {
        return action.description;
    }
    const currentPriceStr = action.currentPrice ? formatPrice(action.currentPrice.dollarsPerKWH) : '';
    const futurePriceStr = action.futurePrice && action.futurePrice.dollarsPerKWH ? formatPrice(action.futurePrice.dollarsPerKWH) : '';
    const deficitTimeStr = action.deficitAt ? formatTime(action.deficitAt) : '';
    const capacityTimeStr = action.capacityAt ? formatTime(action.capacityAt) : '';

    switch (reason) {
        case ActionReason.AlwaysChargeBelowThreshold:
            return `Price is low (${currentPriceStr}). Charging batteries.`;
        case ActionReason.MissingBattery:
            return 'Battery configuration missing or capacity is 0. Standing by.';
        case ActionReason.DeficitCharge:
            return `Battery deficit predicted${deficitTimeStr ? ` at ${deficitTimeStr}` : ''}. Charging now at ${currentPriceStr}${futurePriceStr ? ` (peak later: ${futurePriceStr})` : ''}.`;
        case ActionReason.ArbitrageCharge:
            return `Arbitrage opportunity: charging at ${currentPriceStr}${futurePriceStr ? `, peak later at ${futurePriceStr}` : ''}.`;
        case ActionReason.DischargeBeforeCapacity:
            return `Battery will reach capacity${capacityTimeStr ? ` at ${capacityTimeStr}` : ''} before deficit${deficitTimeStr ? ` at ${deficitTimeStr}` : ''}. Using battery now.`;
        case ActionReason.DeficitSave:
            return `Battery deficit predicted${deficitTimeStr ? ` at ${deficitTimeStr}` : ''}. Saving battery for higher prices${futurePriceStr ? ` (peak: ${futurePriceStr})` : ''}.`;
        case ActionReason.ArbitrageSave:
            return `Current price is peak (${currentPriceStr}). Using battery to offset grid costs.`;
        case ActionReason.NoChange:
            return 'Battery is sufficient. Using battery normally.';
        default:
            return action.description || `Unknown reason: ${reason}`;
    }
};

const Dashboard: React.FC<{ siteID?: string }> = ({ siteID }) => {
    const [location, navigate] = useLocation();
    const search = useSearch();
    const searchParams = useMemo(() => new URLSearchParams(search), [search]);

    const setSearchParams = (params: Record<string, string>) => {
        const p = new URLSearchParams(search);
        Object.entries(params).forEach(([k, v]) => p.set(k, v));
        navigate(location + "?" + p.toString());
    };

    const dateQuery = searchParams.get('date');
    const [actions, setActions] = useState<Action[]>([]);
    const [savings, setSavings] = useState<SavingsStats | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const currentDate = useMemo(() => {
        if (dateQuery) {
            const parts = dateQuery.split('-');
            if (parts.length === 3) {
                const year = parseInt(parts[0], 10);
                const month = parseInt(parts[1], 10) - 1;
                const day = parseInt(parts[2], 10);
                return new Date(year, month, day);
            }
        }
        return new Date();
    }, [dateQuery]);

    useEffect(() => {
        const loadData = async () => {
            setLoading(true);
            setError(null);
            try {
                // Calculate start and end of the day in local time
                const start = new Date(currentDate);
                start.setHours(0, 0, 0, 0);
                const end = new Date(currentDate);
                end.setHours(23, 59, 59, 999);

                // Fetch both actions and savings in parallel
                const [actionsData, savingsData] = await Promise.all([
                    fetchActions(start, end, siteID),
                    fetchSavings(start, end, siteID)
                ]);

                setActions(actionsData || []);
                setSavings(savingsData);
            } catch (err) {
                console.error(err);
                setError('Failed to load data');
            } finally {
                setLoading(false);
            }
        };

        loadData();
    }, [currentDate, siteID]);

    const handleDateChange = (days: number) => {
        const newDate = new Date(currentDate);
        newDate.setDate(newDate.getDate() + days);
        const year = newDate.getFullYear();
        const month = String(newDate.getMonth() + 1).padStart(2, '0');
        const day = String(newDate.getDate()).padStart(2, '0');
        setSearchParams({ date: `${year}-${month}-${day}` });
    };

    // Format date for display
    const formattedDate = currentDate.toLocaleDateString(undefined, {
        weekday: 'long',
        year: 'numeric',
        month: 'long',
        day: 'numeric'
    });

    const netSavings = savings ? savings.batterySavings + savings.solarSavings : 0;

    const groupedActions = useMemo(() => {
        type SummaryType = 'no_change' | 'fault';

        interface ActionSummary {
            isSummary: true;
            type: SummaryType;
            startTime: string;
            avgPrice: number;
            min: number;
            max: number;
            count: number;
            alarms: Set<string>;
        }

        interface ActionSummaryAccumulator extends Omit<ActionSummary, 'avgPrice'> {
            total: number;
        }

        const accumulator: (Action | ActionSummaryAccumulator)[] = [];
        let currentSummary: ActionSummaryAccumulator | null = null;

        for (const action of actions) {
            const isFault = !!action.fault;
            const isNoChange = !isFault && action.batteryMode === BatteryMode.NoChange && action.solarMode === SolarMode.NoChange;

            const price = action.currentPrice ? action.currentPrice.dollarsPerKWH : 0;

            if (isFault) {
                if (currentSummary && currentSummary.type === 'fault') {
                    currentSummary.count++;
                    currentSummary.total += price;
                    currentSummary.min = Math.min(currentSummary.min, price);
                    currentSummary.max = Math.max(currentSummary.max, price);
                } else {
                    if (currentSummary) {
                        accumulator.push(currentSummary);
                    }
                    currentSummary = {
                        isSummary: true,
                        type: 'fault',
                        startTime: action.timestamp,
                        count: 1,
                        total: price,
                        min: price,
                        max: price,
                        alarms: new Set<string>()
                    };
                }

                if (action.systemStatus && action.systemStatus.alarms) {
                    action.systemStatus.alarms.forEach(alarm => {
                       if (currentSummary) currentSummary.alarms.add(alarm.name);
                    });
                }
            } else if (isNoChange) {
                if (currentSummary && currentSummary.type === 'no_change') {
                    currentSummary.count++;
                    currentSummary.total += price;
                    currentSummary.min = Math.min(currentSummary.min, price);
                    currentSummary.max = Math.max(currentSummary.max, price);
                } else {
                    if (currentSummary) {
                        accumulator.push(currentSummary);
                    }
                    currentSummary = {
                        isSummary: true,
                        type: 'no_change',
                        startTime: action.timestamp,
                        count: 1,
                        total: price,
                        min: price,
                        max: price,
                        alarms: new Set<string>()
                    };
                }
            } else {
                if (currentSummary) {
                    accumulator.push(currentSummary);
                    currentSummary = null;
                }
                accumulator.push(action);
            }
        }

        if (currentSummary) {
            accumulator.push(currentSummary);
        }

        return accumulator.map(item => {
            if ('isSummary' in item) {
                const summary = item as ActionSummaryAccumulator;
                const { total, ...rest } = summary;
                return {
                    ...rest,
                    avgPrice: total / summary.count
                } as ActionSummary;
            }
            return item;
        });
    }, [actions]);

    return (
        <div className="content-container action-list-container">
            <header className="header">
                <div className="date-controls">
                    <button onClick={() => handleDateChange(-1)} disabled={loading}>&lt; Prev</button>
                    <h2>{formattedDate}</h2>
                    <button onClick={() => handleDateChange(1)} disabled={loading || currentDate.toDateString() === new Date().toDateString()}>Next &gt;</button>
                </div>
            </header>

            {loading && <p>Loading day...</p>}
            {error && <p className="error">{error}</p>}

            {!loading && !error && (
                <>
                    {savings && (
                        <div className="savings-summary">
                            <div className="savings-header">
                                <h3>Daily Overview</h3>
                            </div>
                            <div className="savings-grid">
                                <div className="savings-item">
                                    <span className="savings-label">Net Savings</span>
                                    <span className={`savings-value ${netSavings > 0 ? 'positive' : netSavings < 0 ? 'negative' : 'neutral'}`}>
                                        ${netSavings.toFixed(2)}
                                    </span>
                                </div>
                                <div className="savings-item">
                                    <span className="savings-label">Solar Savings</span>
                                    <span className="savings-value positive">
                                        ${savings.solarSavings.toFixed(2)}
                                    </span>
                                </div>
                                <div className="savings-item">
                                    <span className="savings-label">Battery Savings</span>
                                    <span className={`savings-value ${savings.batterySavings > 0 ? 'positive' : savings.batterySavings < 0 ? 'negative' : 'neutral'}`}>
                                        ${savings.batterySavings.toFixed(2)}
                                    </span>
                                </div>
                                <div className="savings-item">
                                    <span className="savings-label">Total Cost</span>
                                    <span className="savings-value">
                                        ${savings.cost.toFixed(2)}
                                    </span>
                                </div>
                                <div className="savings-item">
                                    <span className="savings-label">Credit</span>
                                    <span className="savings-value">
                                        ${savings.credit.toFixed(2)}
                                    </span>
                                </div>
                            </div>
                            <div className="savings-details">
                                <span><strong>Home:</strong> {savings.homeUsed.toFixed(2)} kWh</span>
                                <span><strong>Solar:</strong> {savings.solarGenerated.toFixed(2)} kWh</span>
                                <span><strong>Grid Import:</strong> {savings.gridImported.toFixed(2)} kWh</span>
                                <span><strong>Grid Export:</strong> {savings.gridExported.toFixed(2)} kWh</span>
                                <span><strong>Battery Use:</strong> {savings.batteryUsed.toFixed(2)} kWh</span>
                            </div>
                        </div>
                    )}

                    {actions && actions.length === 0 && <p className="no-actions">No actions recorded for this day.</p>}

                    <ul className="action-list">
                        {groupedActions.map((item, index) => {
                            if ('isSummary' in item) {
                                const summary = item as any;
                                const showRange = summary.min !== summary.max;
                                const isFault = summary.type === 'fault';
                                const title = isFault ? 'System Fault' : 'No Change';
                                const alarms = isFault ? Array.from(summary.alarms).join(', ') : '';

                                return (
                                    <li key={index} className={`action-item summary-item ${isFault ? 'fault-item' : ''}`}>
                                        <div className="action-time">
                                            {new Date(summary.startTime).toLocaleTimeString()}
                                        </div>
                                        <div className="action-details">
                                            <h3>{title} {summary.count > 1 && <span>({summary.count}x)</span>}</h3>
                                            {isFault && alarms && (
                                                <p className="fault-alarms">Alarms: {alarms}</p>
                                            )}
                                            <div className="action-footer">
                                                <span className="price-label">Avg Price:</span> ${summary.avgPrice.toFixed(3)}/kWh
                                                {showRange && <span className="price-range"> (Range: ${summary.min.toFixed(3)} - ${summary.max.toFixed(3)})</span>}
                                            </div>
                                        </div>
                                    </li>
                                );
                            }
                            const action = item as Action;
                            const reasonText = getReasonText(action);
                            return (
                                <li key={index} className="action-item">
                                    <div className="action-time">
                                        {new Date(action.timestamp).toLocaleTimeString()}
                                    </div>
                                    <div className="action-details">
                                        <h3>{getBatteryModeLabel(action.batteryMode)}</h3>
                                        <p>{reasonText}</p>
                                        <div className="tags">
                                            {action.batteryMode !== BatteryMode.NoChange && (
                                                <span className={`tag mode-${getBatteryModeClass(action.batteryMode)}`}>{getBatteryModeLabel(action.batteryMode)}</span>
                                            )}
                                            {action.solarMode !== SolarMode.NoChange && (
                                                <span className={`tag solar-${getSolarModeClass(action.solarMode)}`}>{getSolarModeLabel(action.solarMode)}</span>
                                            )}
                                            {action.dryRun && (
                                                <span className="tag dry-run">Dry Run</span>
                                            )}
                                            {action.deficitAt && action.deficitAt !== '0001-01-01T00:00:00Z' && (
                                                <span className="tag tag-info">Deficit: {formatTime(action.deficitAt)}</span>
                                            )}
                                            {action.capacityAt && action.capacityAt !== '0001-01-01T00:00:00Z' && (
                                                <span className="tag tag-info">Capacity: {formatTime(action.capacityAt)}</span>
                                            )}
                                        </div>
                                        {action.currentPrice && (
                                            <div className="action-footer">
                                                <span className="price-label">Price:</span> ${action.currentPrice.dollarsPerKWH.toFixed(3)}/kWh
                                                {action.futurePrice && action.futurePrice.dollarsPerKWH > 0 && (
                                                    <span className="price-future"> · Peak: ${action.futurePrice.dollarsPerKWH.toFixed(3)}/kWh</span>
                                                )}
                                            </div>
                                        )}
                                    </div>
                                </li>
                            );
                        })}
                    </ul>
                </>
            )}
        </div>
    );
};

export default Dashboard;
