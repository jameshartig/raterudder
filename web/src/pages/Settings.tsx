import { useEffect, useState } from 'react';
import { fetchSettings, updateSettings, type Settings as SettingsType } from '../api';
import { Field } from '@base-ui/react/field';
import { Input } from '@base-ui/react/input';
import { Switch } from '@base-ui/react/switch';
import { Collapsible } from '@base-ui/react/collapsible';
import { Select } from '@base-ui/react/select';
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
                <Field.Root className="form-group">
                    <Field.Label>Service</Field.Label>
                    <Select.Root
                        value={settings.utilityProvider}
                        onValueChange={(value) => {
                            const provider = value as string;
                            let newSettings = { ...settings, utilityProvider: provider };
                            if (provider === 'comed_besh') {
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
                        <Select.Trigger className="select-trigger" id="utilityService">
                            <Select.Value placeholder="Select a service..." />
                        </Select.Trigger>
                        <Select.Portal>
                            <Select.Positioner className="select-positioner">
                                <Select.Popup className="select-popup">
                                    <Select.Item className="select-item" value="">
                                        <Select.ItemText>Select a service...</Select.ItemText>
                                    </Select.Item>
                                    <Select.Item className="select-item" value="comed_besh">
                                        <Select.ItemText>Basic Electric Service–Hourly Energy Pricing (BESH)</Select.ItemText>
                                    </Select.Item>
                                </Select.Popup>
                            </Select.Positioner>
                        </Select.Portal>
                    </Select.Root>
                    <Field.Description>Select your utility provider plan.</Field.Description>
                </Field.Root>

                {settings.utilityProvider === 'comed_besh' && (
                    <div className="sub-section">
                        <h4>ComEd Rate Options</h4>
                        <Field.Root className="form-group">
                            <Field.Label>Rate Class</Field.Label>
                            <Select.Root
                                value={settings.utilityRateOptions?.rateClass || 'singleFamilyWithoutElectricHeat'}
                                onValueChange={(value) => {
                                    const newOpts = {
                                        ...settings.utilityRateOptions,
                                        rateClass: value as string,
                                        variableDeliveryRate: settings.utilityRateOptions?.variableDeliveryRate ?? false
                                    };
                                    handleChange('utilityRateOptions', newOpts);
                                }}
                            >
                                <Select.Trigger className="select-trigger" id="comedRateClass">
                                    <Select.Value />
                                </Select.Trigger>
                                <Select.Portal>
                                    <Select.Positioner className="select-positioner">
                                        <Select.Popup className="select-popup">
                                            <Select.Item className="select-item" value="singleFamilyWithoutElectricHeat">
                                                <Select.ItemText>Residential Single Family Without Electric Space Heat</Select.ItemText>
                                            </Select.Item>
                                            <Select.Item className="select-item" value="multiFamilyWithoutElectricHeat">
                                                <Select.ItemText>Residential Multi Family Without Electric Space Heat</Select.ItemText>
                                            </Select.Item>
                                            <Select.Item className="select-item" value="singleFamilyElectricHeat">
                                                <Select.ItemText>Residential Single Family With Electric Space Heat</Select.ItemText>
                                            </Select.Item>
                                            <Select.Item className="select-item" value="multiFamilyElectricHeat">
                                                <Select.ItemText>Residential Multi Family With Electric Space Heat</Select.ItemText>
                                            </Select.Item>
                                        </Select.Popup>
                                    </Select.Positioner>
                                </Select.Portal>
                            </Select.Root>
                        </Field.Root>
                        <Field.Root className="form-group switch-group">
                            <div className="switch-row">
                                <Switch.Root
                                    checked={settings.utilityRateOptions?.variableDeliveryRate ?? false}
                                    onCheckedChange={(checked) => {
                                        const newOpts = {
                                            ...settings.utilityRateOptions,
                                            rateClass: settings.utilityRateOptions?.rateClass || 'singleFamilyWithoutElectricHeat',
                                            variableDeliveryRate: checked
                                        };
                                        handleChange('utilityRateOptions', newOpts);
                                    }}
                                    className="switch-root"
                                >
                                    <Switch.Thumb className="switch-thumb" />
                                </Switch.Root>
                                <Field.Label>Delivery Time-of-Day (DTOD)</Field.Label>
                            </div>
                            <Field.Description>Enable if you are enrolled in ComEd's Delivery Time-of-Day pricing. 30%-47% cheaper than fixed delivery rates in off-peak hours but 2x more expensive in on-peak hours (1pm-7pm).</Field.Description>
                        </Field.Root>
                    </div>
                )}

                {/* Franklin Credentials - Primary Section */}
                <h3 id="franklin-credentials">Franklin Credentials</h3>
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

                <h3>Grid Settings</h3>
                <Field.Root className="form-group switch-group">
                    <div className="switch-row">
                        <Switch.Root
                            checked={settings.gridChargeBatteries}
                            onCheckedChange={(checked) => handleChange('gridChargeBatteries', checked)}
                            className="switch-root"
                        >
                            <Switch.Thumb className="switch-thumb" />
                        </Switch.Root>
                        <Field.Label>Grid Charge Batteries</Field.Label>
                    </div>
                    <Field.Description>Allow charging batteries from the grid.</Field.Description>
                </Field.Root>
                <Field.Root className="form-group switch-group">
                    <div className="switch-row">
                        <Switch.Root
                            checked={settings.gridExportSolar}
                            onCheckedChange={(checked) => handleChange('gridExportSolar', checked)}
                            className="switch-root"
                        >
                            <Switch.Thumb className="switch-thumb" />
                        </Switch.Root>
                        <Field.Label>Grid Export Solar</Field.Label>
                    </div>
                    <Field.Description>Allow exporting solar generation to the grid.</Field.Description>
                </Field.Root>
                <Field.Root className="form-group switch-group">
                    <div className="switch-row">
                        <Switch.Root
                            checked={settings.gridExportBatteries}
                            onCheckedChange={(checked) => handleChange('gridExportBatteries', checked)}
                            className="switch-root"
                        >
                            <Switch.Thumb className="switch-thumb" />
                        </Switch.Root>
                        <Field.Label>Grid Export Batteries</Field.Label>
                    </div>
                    <Field.Description>Allow exporting battery energy to the grid.</Field.Description>
                </Field.Root>

                <Collapsible.Root className="advanced-section">
                    <Collapsible.Trigger className="advanced-trigger">Advanced Settings</Collapsible.Trigger>
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
                                <Field.Label>Dry Run</Field.Label>
                            </div>
                            <Field.Description>Simulate actions without executing them</Field.Description>
                        </Field.Root>

                        <Field.Root className="form-group switch-group">
                            <div className="switch-row">
                                <Switch.Root
                                    checked={settings.pause}
                                    onCheckedChange={(checked) => handleChange('pause', checked)}
                                    className="switch-root"
                                >
                                    <Switch.Thumb className="switch-thumb" />
                                </Switch.Root>
                                <Field.Label>Pause Updates</Field.Label>
                            </div>
                            <Field.Description>Stop automatic updates (prices and history will still sync)</Field.Description>
                        </Field.Root>

                        <h3>Battery Settings</h3>
                        <Field.Root className="form-group">
                            <Field.Label>Min Battery SOC (%)</Field.Label>
                            <Input
                                id="minBatterySOC"
                                type="number"
                                step="1"
                                min="0"
                                max="100"
                                value={settings.minBatterySOC}
                                onChange={(e) => handleChange('minBatterySOC', parseFloat(e.target.value))}
                            />
                            <Field.Description>Minimum State of Charge to maintain.</Field.Description>
                        </Field.Root>

                        <h3>Price Settings</h3>
                        <Field.Root className="form-group">
                            <Field.Label>Always Charge Under ($/kWh)</Field.Label>
                            <Input
                                id="alwaysChargeUnder"
                                type="number"
                                step="0.01"
                                value={settings.alwaysChargeUnderDollarsPerKWH}
                                onChange={(e) => handleChange('alwaysChargeUnderDollarsPerKWH', parseFloat(e.target.value))}
                            />
                            <Field.Description>Always charge the battery if the price (after fees) is below this threshold, regardless of forecast.</Field.Description>
                        </Field.Root>
                        <Field.Root className="form-group">
                            <Field.Label>Min Arbitrage Difference ($/kWh)</Field.Label>
                            <Input
                                id="minArbitrage"
                                type="number"
                                step="0.01"
                                value={settings.minArbitrageDifferenceDollarsPerKWH}
                                onChange={(e) => handleChange('minArbitrageDifferenceDollarsPerKWH', parseFloat(e.target.value))}
                            />
                            <Field.Description>Minimum profit required to trigger charging for arbitrage.</Field.Description>
                        </Field.Root>

                        <Field.Root className="form-group">
                            <Field.Label>Min Deficit Charge Diff ($/kWh)</Field.Label>
                            <Input
                                id="minDeficit"
                                type="number"
                                step="0.01"
                                value={settings.minDeficitPriceDifferenceDollarsPerKWH}
                                onChange={(e) => handleChange('minDeficitPriceDifferenceDollarsPerKWH', parseFloat(e.target.value))}
                            />
                            <Field.Description>Minimum price difference between now and later to justify charging now when there's a predicted battery deficit in the future.</Field.Description>
                        </Field.Root>



                        <h3>Solar Settings</h3>
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

                        <h3>Power History Settings</h3>
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
