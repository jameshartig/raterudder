import { useEffect, useState } from 'react';
import { fetchSettings, updateSettings, fetchUtilities, type Settings as SettingsType, type UtilityProviderInfo, type UtilityRateOption } from '../api';
import { Field } from '@base-ui/react/field';
import { Input } from '@base-ui/react/input';
import { Switch } from '@base-ui/react/switch';
import { Collapsible } from '@base-ui/react/collapsible';
import { Select } from '@base-ui/react/select';
import './Settings.css';
import SparkMD5 from 'spark-md5';


const Settings = ({ siteID }: { siteID?: string }) => {
    const [settings, setSettings] = useState<SettingsType | null>(null);
    const [utilities, setUtilities] = useState<UtilityProviderInfo[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [successMessage, setSuccessMessage] = useState<string | null>(null);

    const [franklinUsername, setFranklinUsername] = useState("");
    const [franklinPassword, setFranklinPassword] = useState("");
    const [franklinGatewayID, setFranklinGatewayID] = useState("");

    // UI State for consolidated views
    const [editUtility, setEditUtility] = useState(false);
    const [editFranklin, setEditFranklin] = useState(false);

    useEffect(() => {
        loadData();
    }, [siteID]);

    const loadData = async () => {
        try {
            setLoading(true);
            const [settingsData, utilitiesData] = await Promise.all([
                fetchSettings(siteID),
                fetchUtilities(siteID)
            ]);
            setSettings(settingsData);
            setUtilities(utilitiesData);
            setError(null);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to load settings');
        } finally {
            setLoading(false);
        }
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!settings) return;

        try {
            setError(null);
            setSuccessMessage(null);

            let franklinCredentials = undefined;
            if (franklinUsername || franklinPassword || franklinGatewayID) {
                // If any credential field is filled, we include credentials update
                if (!franklinUsername || !franklinPassword) {
                    throw new Error("Franklin credential fields (Username, Password) must be filled to update credentials.");
                }

                franklinCredentials = {
                    username: franklinUsername,
                    md5Password: SparkMD5.hash(franklinPassword),
                    gatewayID: franklinGatewayID
                };
            }

            await updateSettings(settings, siteID, franklinCredentials);
            setSuccessMessage('Settings saved successfully');

            if (franklinCredentials) {
                setSettings({
                    ...settings,
                    hasCredentials: {
                        ...settings.hasCredentials,
                        franklin: true
                    }
                });
                setEditFranklin(false);
                // Clear password field after save
                setFranklinPassword("");
            }

            setTimeout(() => setSuccessMessage(null), 3000);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to save settings');
        }
    };

    const handleChange = (field: keyof SettingsType, value: any) => {
        if (!settings) return;
        setSettings({ ...settings, [field]: value });
    };

    if (loading) return <div>Loading settings...</div>;
    if (!settings) return <div>Error loading settings</div>;

    return (
        <div className="content-container settings-container">
            <h2>Settings</h2>
            {error && <div className="error-message">{error}</div>}
            {successMessage && <div className="success-message">{successMessage}</div>}

            <form onSubmit={handleSubmit}>
                {/* Pause Updates at the top */}
                <div className="section-header pause-section">
                    <h3>Automation Status</h3>
                </div>
                <Field.Root className="form-group switch-group">
                    <div className="switch-row">
                        <Switch.Root
                            checked={settings.pause}
                            onCheckedChange={(checked) => handleChange('pause', checked)}
                            className="switch-root"
                        >
                            <Switch.Thumb className="switch-thumb" />
                        </Switch.Root>
                        <Field.Label>Pause Automation</Field.Label>
                    </div>
                    <Field.Description>If enabled, stop changing states but continue monitoring.</Field.Description>
                </Field.Root>

                {/* Utility Service Section */}
                <div className="section-header">
                    <h3>Utility Service</h3>
                    {settings.utilityProvider && settings.utilityRate && !editUtility && (
                        <button type="button" className="text-button" onClick={() => setEditUtility(true)}>Change</button>
                    )}
                </div>

                {settings.utilityProvider && settings.utilityRate && !editUtility ? (
                    <div className="configured-summary" onClick={() => setEditUtility(true)}>
                        <div className="summary-info">
                            <span className="summary-label">
                                {utilities.find(u => u.id === settings.utilityProvider)?.name || settings.utilityProvider}
                            </span>
                            <span className="summary-sublabel">
                                {utilities.find(u => u.id === settings.utilityProvider)?.rates.find(r => r.id === settings.utilityRate)?.name || settings.utilityRate}
                            </span>
                        </div>
                        <div className="summary-status">Configured</div>
                    </div>
                ) : (
                    <div className={editUtility ? "edit-section" : ""}>
                        <Field.Root className="form-group">
                            <Field.Label>Service</Field.Label>
                            <Select.Root
                                value={settings.utilityProvider}
                                onValueChange={(value) => {
                                    setEditUtility(true);
                                    const providerID = value as string;
                                    const provider = utilities.find(u => u.id === providerID);
                                    let newSettings = {
                                        ...settings,
                                        utilityProvider: providerID,
                                        utilityRate: "",
                                        utilityRateOptions: {}
                                    };

                                    // If provider has only one rate, auto-select it
                                    if (provider?.rates.length === 1) {
                                        const rate = provider.rates[0];
                                        newSettings.utilityRate = rate.id;
                                        const newOpts: any = {};
                                        rate.options.forEach((opt: UtilityRateOption) => {
                                            newOpts[opt.field] = opt.default;
                                        });
                                        newSettings.utilityRateOptions = newOpts;
                                    }

                                    setSettings(newSettings);
                                }}
                            >
                                <Select.Trigger className="select-trigger" id="utilityService" aria-label="Service">
                                    <Select.Value>
                                        {utilities.find(u => u.id === settings.utilityProvider)?.name || 'Select a service...'}
                                    </Select.Value>
                                </Select.Trigger>
                                <Select.Portal>
                                    <Select.Positioner className="select-positioner">
                                        <Select.Popup className="select-popup">
                                            <Select.Item className="select-item" value="">
                                                <Select.ItemText>Select a service...</Select.ItemText>
                                            </Select.Item>
                                            {utilities.map(u => (
                                                <Select.Item key={u.id} className="select-item" value={u.id}>
                                                    <Select.ItemText>{u.name}</Select.ItemText>
                                                </Select.Item>
                                            ))}
                                        </Select.Popup>
                                    </Select.Positioner>
                                </Select.Portal>
                            </Select.Root>
                        </Field.Root>

                        {(() => {
                            const provider = utilities.find(u => u.id === settings.utilityProvider);
                            if (!provider) return null;

                            return (
                                <Field.Root className="form-group">
                                    <Field.Label>Rate/Plan</Field.Label>
                                    <Select.Root
                                        value={settings.utilityRate}
                                        onValueChange={(value) => {
                                            const rateID = value as string;
                                            const rate = provider.rates.find(r => r.id === rateID);
                                            let newSettings = { ...settings, utilityRate: rateID };
                                            if (rate) {
                                                const newOpts: any = {};
                                                rate.options.forEach((opt: UtilityRateOption) => {
                                                    newOpts[opt.field] = opt.default;
                                                });
                                                newSettings.utilityRateOptions = newOpts;
                                            }
                                            setSettings(newSettings);
                                        }}
                                    >
                                        <Select.Trigger className="select-trigger" id="utilityRate" aria-label="Rate/Plan">
                                            <Select.Value>
                                                {provider.rates.find(r => r.id === settings.utilityRate)?.name || 'Select a rate/plan...'}
                                            </Select.Value>
                                        </Select.Trigger>
                                        <Select.Portal>
                                            <Select.Positioner className="select-positioner">
                                                <Select.Popup className="select-popup">
                                                    <Select.Item className="select-item" value="">
                                                        <Select.ItemText>Select a rate/plan...</Select.ItemText>
                                                    </Select.Item>
                                                    {provider.rates.map(r => (
                                                        <Select.Item key={r.id} className="select-item" value={r.id}>
                                                            <Select.ItemText>{r.name}</Select.ItemText>
                                                        </Select.Item>
                                                    ))}
                                                </Select.Popup>
                                            </Select.Positioner>
                                        </Select.Portal>
                                    </Select.Root>
                                </Field.Root>
                            );
                        })()}

                        {(() => {
                            const provider = utilities.find(u => u.id === settings.utilityProvider);
                            const rate = provider?.rates.find(r => r.id === settings.utilityRate);
                            if (!rate || rate.options.length === 0) return null;

                            return (
                                <div className="sub-section">
                                    {rate.options.map((opt: UtilityRateOption) => (
                                        <Field.Root key={opt.field} className={`form-group ${opt.type === 'switch' ? 'switch-group' : ''}`}>
                                            {opt.type === 'select' && (
                                                <>
                                                    <Field.Label>{opt.name}</Field.Label>
                                                    <Select.Root
                                                        value={settings.utilityRateOptions?.[opt.field] || opt.default}
                                                        onValueChange={(value) => {
                                                            const newOpts = {
                                                                ...settings.utilityRateOptions,
                                                                [opt.field]: value
                                                            };
                                                            handleChange('utilityRateOptions', newOpts);
                                                        }}
                                                    >
                                                        <Select.Trigger className="select-trigger" id={`opt-${opt.field}`}>
                                                            <Select.Value>
                                                                {opt.choices?.find(c => c.value === (settings.utilityRateOptions?.[opt.field] || opt.default))?.name || (settings.utilityRateOptions?.[opt.field] || opt.default)}
                                                            </Select.Value>
                                                        </Select.Trigger>
                                                        <Select.Portal>
                                                            <Select.Positioner className="select-positioner">
                                                                <Select.Popup className="select-popup">
                                                                    {opt.choices?.map((choice) => (
                                                                        <Select.Item key={choice.value} className="select-item" value={choice.value}>
                                                                            <Select.ItemText>{choice.name}</Select.ItemText>
                                                                        </Select.Item>
                                                                    ))}
                                                                </Select.Popup>
                                                            </Select.Positioner>
                                                        </Select.Portal>
                                                    </Select.Root>
                                                </>
                                            )}
                                            {opt.type === 'switch' && (
                                                <>
                                                    <div className="switch-row">
                                                        <Switch.Root
                                                            id={`opt-${opt.field}`}
                                                            checked={settings.utilityRateOptions?.[opt.field] ?? !!opt.default}
                                                            onCheckedChange={(checked) => {
                                                                const newOpts = {
                                                                    ...settings.utilityRateOptions,
                                                                    [opt.field]: checked
                                                                };
                                                                handleChange('utilityRateOptions', newOpts);
                                                            }}
                                                            className="switch-root"
                                                        >
                                                            <Switch.Thumb className="switch-thumb" />
                                                        </Switch.Root>
                                                        <Field.Label htmlFor={`opt-${opt.field}`}>{opt.name}</Field.Label>
                                                    </div>
                                                </>
                                            )}
                                            {opt.description && <Field.Description>{opt.description}</Field.Description>}
                                        </Field.Root>
                                    ))}
                                </div>
                            );
                        })()}
                        {editUtility && (
                            <button type="button" className="text-button cancel-button" onClick={() => setEditUtility(false)}>Done</button>
                        )}
                    </div>
                )}

                {/* Franklin Credentials Section */}
                <div className="section-header">
                    <h3 id="franklin-credentials">Franklin Credentials</h3>
                    {settings.hasCredentials.franklin && !editFranklin && (
                        <button type="button" className="text-button" onClick={() => setEditFranklin(true)}>Update</button>
                    )}
                </div>

                {settings.hasCredentials.franklin && !editFranklin ? (
                    <div className="configured-summary" onClick={() => setEditFranklin(true)}>
                        <div className="summary-info">
                            <span className="summary-label">Franklin Energy System</span>
                        </div>
                        <div className="summary-status">Connected</div>
                    </div>
                ) : (
                    <div className={editFranklin ? "edit-section" : ""}>
                        <Field.Root className="form-group">
                            <Field.Label>Username (Email)</Field.Label>
                            <Input
                                id="franklinUsername"
                                type="email"
                                value={franklinUsername}
                                onChange={(e) => setFranklinUsername(e.target.value)}
                                placeholder="Enter FranklinWH email"
                            />
                        </Field.Root>
                        <Field.Root className="form-group">
                            <Field.Label>Password</Field.Label>
                            <Input
                                id="franklinPassword"
                                type="password"
                                value={franklinPassword}
                                onChange={(e) => setFranklinPassword(e.target.value)}
                                placeholder="Enter new password to update"
                            />
                        </Field.Root>
                        <Field.Root className="form-group">
                            <Field.Label>Gateway ID (Optional)</Field.Label>
                            <Input
                                id="franklinGatewayID"
                                type="text"
                                value={franklinGatewayID}
                                onChange={(e) => setFranklinGatewayID(e.target.value)}
                                placeholder="Enter FranklinWH Gateway ID"
                            />
                        </Field.Root>
                        {editFranklin && (
                            <button type="button" className="text-button cancel-button" onClick={() => setEditFranklin(false)}>Done</button>
                        )}
                    </div>
                )}

                <div className="section-header">
                    <h3>Automation Thresholds</h3>
                </div>
                <div className="form-grid compact-grid">
                    <Field.Root className="form-group compact">
                        <Field.Label htmlFor="minBatterySOC">Minimum Battery %</Field.Label>
                        <Input
                            id="minBatterySOC"
                            type="number"
                            step="1"
                            min="0"
                            max="100"
                            value={settings.minBatterySOC}
                            onChange={(e) => handleChange('minBatterySOC', parseFloat(e.target.value))}
                        />
                        <Field.Description>Maintain battery charge at or above this level at all costs.</Field.Description>
                    </Field.Root>

                    <Field.Root className="form-group compact">
                        <Field.Label htmlFor="alwaysChargeUnder">Always Charge Below ($/kWh)</Field.Label>
                        <Input
                            id="alwaysChargeUnder"
                            type="number"
                            step="0.01"
                            value={settings.alwaysChargeUnderDollarsPerKWH}
                            onChange={(e) => handleChange('alwaysChargeUnderDollarsPerKWH', parseFloat(e.target.value))}
                        />
                        <Field.Description>Charge battery whenever the price is less than this threshold.</Field.Description>
                    </Field.Root>

                    <Field.Root className="form-group compact">
                        <Field.Label htmlFor="minArbitrage">Min Arbitrage Profit ($/kWh)</Field.Label>
                        <Input
                            id="minArbitrage"
                            type="number"
                            step="0.01"
                            value={settings.minArbitrageDifferenceDollarsPerKWH}
                            onChange={(e) => handleChange('minArbitrageDifferenceDollarsPerKWH', parseFloat(e.target.value))}
                        />
                        <Field.Description>Required profit margin to trigger immediate charging to later use/export at a higher prices.</Field.Description>
                    </Field.Root>

                    <Field.Root className="form-group compact">
                        <Field.Label htmlFor="minDeficit">Charge for Deficit ($/kWh)</Field.Label>
                        <Input
                            id="minDeficit"
                            type="number"
                            step="0.01"
                            value={settings.minDeficitPriceDifferenceDollarsPerKWH}
                            onChange={(e) => handleChange('minDeficitPriceDifferenceDollarsPerKWH', parseFloat(e.target.value))}
                        />
                        <Field.Description>Price difference required to justify charging now to avoid a future battery depletion.</Field.Description>
                    </Field.Root>
                </div>

                <div className="section-header">
                    <h3>Grid Restrictions</h3>
                </div>

                <div className="grid-strategy-grid">
                    <Field.Root className="form-group switch-group compact">
                        <div className="switch-row">
                            <Switch.Root
                                id="gridChargeBatteries"
                                checked={settings.gridChargeBatteries}
                                onCheckedChange={(checked) => handleChange('gridChargeBatteries', checked)}
                                className="switch-root"
                            >
                                <Switch.Thumb className="switch-thumb" />
                            </Switch.Root>
                            <Field.Label htmlFor="gridChargeBatteries">Grid Can Charge Battery</Field.Label>
                        </div>
                    </Field.Root>

                    <Field.Root className="form-group switch-group compact">
                        <div className="switch-row">
                            <Switch.Root
                                id="gridExportSolar"
                                checked={settings.gridExportSolar}
                                onCheckedChange={(checked) => handleChange('gridExportSolar', checked)}
                                className="switch-root"
                            >
                                <Switch.Thumb className="switch-thumb" />
                            </Switch.Root>
                            <Field.Label htmlFor="gridExportSolar">Export Solar to Grid</Field.Label>
                        </div>
                    </Field.Root>

                    <Field.Root className="form-group switch-group compact">
                        <div className="switch-row">
                            <Switch.Root
                                id="gridExportBatteries"
                                checked={settings.gridExportBatteries}
                                onCheckedChange={(checked) => handleChange('gridExportBatteries', checked)}
                                className="switch-root"
                            >
                                <Switch.Thumb className="switch-thumb" />
                            </Switch.Root>
                            <Field.Label htmlFor="gridExportBatteries">Export Battery to Grid</Field.Label>
                        </div>
                    </Field.Root>
                </div>



                <Collapsible.Root className="advanced-section">
                    <Collapsible.Trigger className="advanced-trigger">Advanced Tuning Settings</Collapsible.Trigger>
                    <Collapsible.Panel className="advanced-panel">
                        <Field.Root className="form-group switch-group" style={{ marginTop: '1rem' }}>
                            <div className="switch-row">
                                <Switch.Root
                                    checked={settings.dryRun}
                                    onCheckedChange={(checked) => handleChange('dryRun', checked)}
                                    className="switch-root"
                                >
                                    <Switch.Thumb className="switch-thumb" />
                                </Switch.Root>
                                <Field.Label>Dry Run Mode</Field.Label>
                            </div>
                            <Field.Description>Simulate actions without executing them (useful for testing).</Field.Description>
                        </Field.Root>



                        <div className="section-header">
                            <h3>Solar Settings</h3>
                        </div>
                        <Field.Root className="form-group">
                            <Field.Label>Solar Trend Ratio Max</Field.Label>
                            <Input
                                id="solarTrendRatioMax"
                                type="number"
                                step="0.1"
                                min="1"
                                value={settings.solarTrendRatioMax}
                                onChange={(e) => handleChange('solarTrendRatioMax', parseFloat(e.target.value))}
                            />
                            <Field.Description>Maximum ratio for solar trend adjustment. Higher values allow more aggressive upward solar predictions. Default: 3.0</Field.Description>
                        </Field.Root>
                        <Field.Root className="form-group">
                            <Field.Label>Solar Bell Curve Multiplier</Field.Label>
                            <Input
                                id="solarBellCurveMultiplier"
                                type="number"
                                step="0.1"
                                min="0"
                                max="2"
                                value={settings.solarBellCurveMultiplier}
                                onChange={(e) => handleChange('solarBellCurveMultiplier', parseFloat(e.target.value))}
                            />
                            <Field.Description>Multiplier for bell curve solar smoothing. 0 disables smoothing entirely. Default: 1.0</Field.Description>
                        </Field.Root>

                        {settings.gridExportSolar && settings.solarBellCurveMultiplier > 0.7 && (
                            <div className="warning-notice">
                                ⚠️ Solar export is enabled but the bell curve multiplier is high ({settings.solarBellCurveMultiplier}). Since solar readings are less likely curtailed with export on, consider lowering it (e.g. 0.5).
                            </div>
                        )}
                        {!settings.gridExportSolar && settings.solarBellCurveMultiplier < 0.7 && settings.solarBellCurveMultiplier > 0 && (
                            <div className="warning-notice">
                                ⚠️ Solar export is disabled but the bell curve multiplier is low ({settings.solarBellCurveMultiplier}). Since solar readings may be curtailed without export, consider raising it (e.g. 1.0).
                            </div>
                        )}

                        <div className="section-header">
                            <h3>Power History Settings</h3>
                        </div>
                        <Field.Root className="form-group">
                            <Field.Label>Ignore Usage Outlier Multiple</Field.Label>
                            <Input
                                id="ignoreHourUsageOverMultiple"
                                type="number"
                                step="0.1"
                                min="1"
                                value={settings.ignoreHourUsageOverMultiple}
                                onChange={(e) => handleChange('ignoreHourUsageOverMultiple', parseFloat(e.target.value))}
                            />
                            <Field.Description>If a single hour's usage is this many times greater than the average of other data points for that hour, ignore it. Must be &ge; 1.</Field.Description>
                        </Field.Root>
                    </Collapsible.Panel>
                </Collapsible.Root>

                <button type="submit" className="save-button">
                    Save Settings
                </button>
            </form>
        </div>
    );
};
export default Settings;
