export const defaultAuthStatus = {
    loggedIn: true,
    authRequired: true,
    clientID: 'test-client-id',
    email: 'user@example.com',
    siteIDs: ['site1']
};

export const defaultSavings = {
    batterySavings: 0,
    solarSavings: 0,
    cost: 0,
    credit: 0,
    avoidedCost: 0,
    chargingCost: 0,
    solarGenerated: 0,
    gridImported: 0,
    gridExported: 0,
    homeUsed: 0,
    batteryUsed: 0,
    lastPrice: 0,
    lastCost: 0,
};

export const defaultSettings = {
    dryRun: false,
    pause: false,
    release: 'production',
    minBatterySOC: 10,
    gridExportSolar: false,
    gridExportBatteries: false,
    gridChargeBatteries: true,
    solarTrendRatioMax: 3.0,
    solarBellCurveMultiplier: 1.0,
    ignoreHourUsageOverMultiple: 2,
    alwaysChargeUnderDollarsPerKWH: 0.05,
    minArbitrageDifferenceDollarsPerKWH: 0.03,
    minDeficitPriceDifferenceDollarsPerKWH: 0.02,
    utilityProvider: 'comed_besh',
    utilityRateOptions: {
        rateClass: 'singleFamilyWithoutElectricHeat',
        variableDeliveryRate: false,
    },
    hasCredentials: {
        franklin: false
    }
};

export const setupDefaultApiMocks = (api: any) => {
    api.fetchActions.mockResolvedValue([]);
    api.fetchSavings.mockResolvedValue(defaultSavings);
    api.fetchAuthStatus.mockResolvedValue(defaultAuthStatus);
    api.fetchSettings.mockResolvedValue(defaultSettings);
    api.updateSettings.mockResolvedValue(undefined);
    api.login.mockResolvedValue(undefined);
    api.logout.mockResolvedValue(undefined);
    api.fetchModeling.mockResolvedValue([]);
};
