import { useEffect, useState } from 'react';
import { fetchSettings, updateSettings, type Settings as SettingsType } from '../api';
import './Settings.css';
import SparkMD5 from 'spark-md5';

const Settings = ({ siteID }: { siteID?: string }) => {
    const [settings, setSettings] = useState<SettingsType | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [successMessage, setSuccessMessage] = useState<string | null>(null);

    // Credentials State
    const [franklinUsername, setFranklinUsername] = useState("");
    const [franklinPassword, setFranklinPassword] = useState("");
    const [franklinGatewayID, setFranklinGatewayID] = useState("");

    useEffect(() => {
        loadSettings();
    }, [siteID]);

    const loadSettings = async () => {
        try {
            setLoading(true);
            const data = await fetchSettings(siteID);
            setSettings(data);
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

            let franklinHeaders = undefined;
            if (franklinUsername || franklinPassword || franklinGatewayID) {
                // If any credential field is filled, we include credentials update
                if (!franklinUsername || !franklinPassword) {
                    throw new Error("Franklin credential fields (Username, Password) must be filled to update credentials.");
                }

                franklinHeaders = {
                    username: franklinUsername,
                    md5Password: SparkMD5.hash(franklinPassword),
                    gatewayID: franklinGatewayID
                };
            }

            await updateSettings(settings, siteID, franklinHeaders);
            setSuccessMessage('Settings saved successfully');

            // Clear password field after save
            setFranklinPassword("");

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
                {/* Utility Service - Primary Section */}
                <h3>Utility Service</h3>
                <div className="form-group">
                    <label htmlFor="utilityService">Service</label>
                    <select
                        id="utilityService"
                        value={settings.utilityProvider}
                        onChange={(e) => {
                            const provider = e.target.value;
                            let newSettings = { ...settings, utilityProvider: provider };
                            if (provider === 'comed_besh') {
                                // Ensure rate options are initialized if not present
                                if (!newSettings.utilityRateOptions.rateClass) {
                                    newSettings = {
                                        ...newSettings,
                                        utilityRateOptions: {
                                            ...newSettings.utilityRateOptions,
                                            rateClass: 'singleFamilyWithoutElectricHeat',
                                            variableDeliveryRate: false
                                        }
                                    };
                                }
                            }
                            setSettings(newSettings);
                        }}
                    >
                        <option value="">Select a service...</option>
                        <option value="comed_besh">Basic Electric Service–Hourly Energy Pricing (BESH)</option>
                    </select>
                    <span className="help-text">Select your utility provider plan.</span>
                </div>

                {settings.utilityProvider === 'comed_besh' && (
                    <div className="sub-section">
                        <h4>ComEd Rate Options</h4>
                        <div className="form-group">
                            <label htmlFor="comedRateClass">Rate Class</label>
                            <select
                                id="comedRateClass"
                                value={settings.utilityRateOptions?.rateClass || 'singleFamilyWithoutElectricHeat'}
                                onChange={(e) => {
                                    const newOpts = {
                                        ...settings.utilityRateOptions,
                                        rateClass: e.target.value,
                                        // maintain other props if any
                                        variableDeliveryRate: settings.utilityRateOptions?.variableDeliveryRate ?? false
                                    };
                                    handleChange('utilityRateOptions', newOpts);
                                }}
                            >
                                <option value="singleFamilyWithoutElectricHeat">Residential Single Family Without Electric Space Heat</option>
                                <option value="multiFamilyWithoutElectricHeat">Residential Multi Family Without Electric Space Heat</option>
                                <option value="singleFamilyElectricHeat">Residential Single Family With Electric Space Heat</option>
                                <option value="multiFamilyElectricHeat">Residential Multi Family With Electric Space Heat</option>
                            </select>
                        </div>
                        <div className="form-group checkbox-group">
                            <label>
                                <input
                                    type="checkbox"
                                    checked={settings.utilityRateOptions?.variableDeliveryRate ?? false}
                                    onChange={(e) => {
                                        const newOpts = {
                                            ...settings.utilityRateOptions,
                                            rateClass: settings.utilityRateOptions?.rateClass || 'singleFamilyWithoutElectricHeat',
                                            variableDeliveryRate: e.target.checked
                                        };
                                        handleChange('utilityRateOptions', newOpts);
                                    }}
                                />
                                Delivery Time-of-Day (DTOD)
                            </label>
                            <span className="help-text">Enable if you are enrolled in ComEd's Delivery Time-of-Day pricing. 30%-47% cheaper than fixed delivery rates in off-peak hours but 2x more expensive in on-peak hours (1pm-7pm).</span>
                        </div>
                    </div>
                )}

                {/* Franklin Credentials - Primary Section */}
                <h3 id="franklin-credentials">Franklin Credentials</h3>
                <div className="form-group">
                    <label htmlFor="franklinUsername">Username (Email)</label>
                    <input
                        id="franklinUsername"
                        type="email"
                        value={franklinUsername}
                        onChange={(e) => setFranklinUsername(e.target.value)}
                        placeholder="Enter FranklinWH email"
                    />
                </div>
                <div className="form-group">
                    <label htmlFor="franklinPassword">Password</label>
                    <input
                        id="franklinPassword"
                        type="password"
                        value={franklinPassword}
                        onChange={(e) => setFranklinPassword(e.target.value)}
                        placeholder="Enter new password to update"
                    />
                </div>
                <div className="form-group">
                    <label htmlFor="franklinGatewayID">Gateway ID (Optional)</label>
                    <input
                        id="franklinGatewayID"
                        type="text"
                        value={franklinGatewayID}
                        onChange={(e) => setFranklinGatewayID(e.target.value)}
                        placeholder="Enter FranklinWH Gateway ID"
                    />
                </div>

                <h3>Grid Settings</h3>
                <div className="form-group checkbox-group">
                    <label>
                        <input
                            type="checkbox"
                            checked={settings.gridChargeBatteries}
                            onChange={(e) => handleChange('gridChargeBatteries', e.target.checked)}
                        />
                        Grid Charge Batteries
                    </label>
                    <span className="help-text">Allow charging batteries from the grid.</span>
                </div>
                <div className="form-group checkbox-group">
                    <label>
                        <input
                            type="checkbox"
                            checked={settings.gridExportSolar}
                            onChange={(e) => handleChange('gridExportSolar', e.target.checked)}
                        />
                        Grid Export Solar
                    </label>
                    <span className="help-text">Allow exporting solar generation to the grid.</span>
                </div>
                <div className="form-group checkbox-group">
                    <label>
                        <input
                            type="checkbox"
                            checked={settings.gridExportBatteries}
                            onChange={(e) => handleChange('gridExportBatteries', e.target.checked)}
                        />
                        Grid Export Batteries
                    </label>
                    <span className="help-text">Allow exporting battery energy to the grid.</span>
                </div>

                <details>
                    <summary>Advanced Settings</summary>

                    <div className="form-group checkbox-group" style={{ marginTop: '1rem' }}>
                        <label>
                            <input
                                type="checkbox"
                                checked={settings.dryRun}
                                onChange={(e) => handleChange('dryRun', e.target.checked)}
                            />
                            Dry Run
                        </label>
                        <span className="help-text">Simulate actions without executing them</span>
                    </div>

                    <div className="form-group checkbox-group">
                        <label>
                            <input
                                type="checkbox"
                                checked={settings.pause}
                                onChange={(e) => handleChange('pause', e.target.checked)}
                            />
                            Pause Updates
                        </label>
                        <span className="help-text">Stop automatic updates (prices and history will still sync)</span>
                    </div>

                    <h3>Battery Settings</h3>
                    <div className="form-group">
                        <label htmlFor="minBatterySOC">Min Battery SOC (%)</label>
                        <input
                            id="minBatterySOC"
                            type="number"
                            step="1"
                            min="0"
                            max="100"
                            value={settings.minBatterySOC}
                            onChange={(e) => handleChange('minBatterySOC', parseFloat(e.target.value))}
                        />
                        <span className="help-text">Minimum State of Charge to maintain.</span>
                    </div>

                    <h3>Price Settings</h3>
                    <div className="form-group">
                        <label htmlFor="alwaysChargeUnder">Always Charge Under ($/kWh)</label>
                        <input
                            id="alwaysChargeUnder"
                            type="number"
                            step="0.01"
                            value={settings.alwaysChargeUnderDollarsPerKWH}
                            onChange={(e) => handleChange('alwaysChargeUnderDollarsPerKWH', parseFloat(e.target.value))}
                        />
                        <span className="help-text">Always charge the battery if the price (after fees) is below this threshold, regardless of forecast.</span>
                    </div>
                    <div className="form-group">
                        <label htmlFor="minArbitrage">Min Arbitrage Difference ($/kWh)</label>
                        <input
                            id="minArbitrage"
                            type="number"
                            step="0.01"
                            value={settings.minArbitrageDifferenceDollarsPerKWH}
                            onChange={(e) => handleChange('minArbitrageDifferenceDollarsPerKWH', parseFloat(e.target.value))}
                        />
                        <span className="help-text">Minimum profit required to trigger charging for arbitrage.</span>
                    </div>

                    <div className="form-group">
                        <label htmlFor="minDeficit">Min Deficit Charge Diff ($/kWh)</label>
                        <input
                            id="minDeficit"
                            type="number"
                            step="0.01"
                            value={settings.minDeficitPriceDifferenceDollarsPerKWH}
                            onChange={(e) => handleChange('minDeficitPriceDifferenceDollarsPerKWH', parseFloat(e.target.value))}
                        />
                        <span className="help-text">Minimum price difference between now and later to justify charging now when there's a predicted battery deficit in the future.</span>
                    </div>



                    <h3>Solar Settings</h3>
                    <div className="form-group">
                        <label htmlFor="solarTrendRatioMax">Solar Trend Ratio Max</label>
                        <input
                            id="solarTrendRatioMax"
                            type="number"
                            step="0.1"
                            min="1"
                            value={settings.solarTrendRatioMax}
                            onChange={(e) => handleChange('solarTrendRatioMax', parseFloat(e.target.value))}
                        />
                        <span className="help-text">Maximum ratio for solar trend adjustment. Higher values allow more aggressive upward solar predictions. Default: 3.0</span>
                    </div>
                    <div className="form-group">
                        <label htmlFor="solarBellCurveMultiplier">Solar Bell Curve Multiplier</label>
                        <input
                            id="solarBellCurveMultiplier"
                            type="number"
                            step="0.1"
                            min="0"
                            max="2"
                            value={settings.solarBellCurveMultiplier}
                            onChange={(e) => handleChange('solarBellCurveMultiplier', parseFloat(e.target.value))}
                        />
                        <span className="help-text">Multiplier for bell curve solar smoothing. 0 disables smoothing entirely. Default: 1.0</span>
                    </div>

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

                    <h3>Power History Settings</h3>
                    <div className="form-group">
                        <label htmlFor="ignoreHourUsageOverMultiple">Ignore Usage Outlier Multiple</label>
                        <input
                            id="ignoreHourUsageOverMultiple"
                            type="number"
                            step="0.1"
                            min="1"
                            value={settings.ignoreHourUsageOverMultiple}
                            onChange={(e) => handleChange('ignoreHourUsageOverMultiple', parseFloat(e.target.value))}
                        />
                        <span className="help-text">If a single hour's usage is this many times greater than the average of other data points for that hour, ignore it. Must be &ge; 1.</span>
                    </div>
                </details>

                <button type="submit" className="save-button">
                    Save Settings
                </button>
            </form>
        </div>
    );
};
export default Settings;
