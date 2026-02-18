export interface SystemAlarm {
    name: string;
    description: string;
    time: string;
    code: string;
}

export interface SystemStorm {
    description: string;
    tsStart: string;
    tsEnd: string;
}

export interface SystemStatus {
    alarms?: SystemAlarm[];
    storms?: SystemStorm[];
    // Add other fields from backend if useful, but alarms is what we need now
    [key: string]: any;
}

export const ActionReason = {
    AlwaysChargeBelowThreshold: 'alwaysChargeBelowThreshold',
    MissingBattery: 'missingBattery',
    DeficitCharge: 'deficitCharge',
    ArbitrageCharge: 'arbitrageCharge',
    DischargeBeforeCapacity: 'dischargeBeforeCapacity',
    DeficitSaveForPeak: 'deficitSaveForPeak',
    ArbitrageSave: 'dischargeAtPeak',
    NoChange: 'sufficientBattery',
    EmergencyMode: 'emergencyMode',
    HasAlarms: 'hasAlarms',
    WaitingToCharge: 'waitingToCharge',
    // deprecated
    DeficitSave: 'deficitSave',
} as const;

export type ActionReason = typeof ActionReason[keyof typeof ActionReason];

export interface PriceInfo {
    tsStart: string;
    tsEnd: string;
    dollarsPerKWH: number;
}

export interface Action {
    timestamp: string;
    batteryMode: number;
    solarMode: number;
    reason?: ActionReason;
    description: string;
    currentPrice?: PriceInfo;
    futurePrice?: PriceInfo;
    deficitAt?: string;
    capacityAt?: string;
    systemStatus?: SystemStatus;
    dryRun?: boolean;
    fault?: boolean;
}

export const BatteryMode = {
    NoChange: 0,
    Standby: 1,
    ChargeAny: 2,
    ChargeSolar: 3,
    Load: -1,
} as const;

export type BatteryMode = typeof BatteryMode[keyof typeof BatteryMode];

export const SolarMode = {
    NoChange: 0,
    NoExport: 1,
    Any: 2,
} as const;

export type SolarMode = typeof SolarMode[keyof typeof SolarMode];

async function extractError(response: Response, fallback: string): Promise<string> {
    try {
        const body = await response.json();
        if (body && typeof body.error === 'string') {
            return body.error;
        }
    } catch { /* ignore parse failures */ }
    return fallback;
}

export const fetchActions = async (start: Date, end: Date, siteID?: string): Promise<Action[]|null> => {
    const startStr = start.toISOString();
    const endStr = end.toISOString();
    const query = new URLSearchParams({
        start: startStr,
        end: endStr,
    });
    if (siteID) {
        query.append('siteID', siteID);
    }
    const response = await fetch(`/api/history/actions?${query.toString()}`);
    if (!response.ok) {
        throw new Error(await extractError(response, 'Failed to fetch actions'));
    }
    return response.json();
};

export interface SavingsStats {
    timestamp: string;
    cost: number;
    credit: number;
    batterySavings: number;
    solarSavings: number;
    avoidedCost: number;
    chargingCost: number;
    solarGenerated: number;
    gridImported: number;
    gridExported: number;
    homeUsed: number;
    batteryUsed: number;
}

export const fetchSavings = async (start: Date, end: Date, siteID?: string): Promise<SavingsStats|null> => {
    const startStr = start.toISOString();
    const endStr = end.toISOString();
    const query = new URLSearchParams({
        start: startStr,
        end: endStr,
    });
    if (siteID) {
        query.append('siteID', siteID);
    }
    const response = await fetch(`/api/history/savings?${query.toString()}`);
    if (!response.ok) {
        throw new Error(await extractError(response, 'Failed to fetch savings'));
    }
    return response.json();
};

export interface UtilityRateOptions {
    rateClass: string;
    variableDeliveryRate: boolean;
}

export interface Settings {
    dryRun: boolean;
    pause: boolean;
    alwaysChargeUnderDollarsPerKWH: number;
    minArbitrageDifferenceDollarsPerKWH: number;
    minDeficitPriceDifferenceDollarsPerKWH: number;
    minBatterySOC: number;
    ignoreHourUsageOverMultiple: number;
    gridChargeBatteries: boolean;
    gridExportSolar: boolean;
    solarTrendRatioMax: number;
    solarBellCurveMultiplier: number;
    utilityProvider: string;
    utilityRateOptions: UtilityRateOptions;
    hasCredentials: {
        franklin: boolean;
    };
}

export interface FranklinCredentials {
    username: string;
    md5Password: string;
    gatewayID: string;
}

export interface SettingsUpdate {
    settings: Settings;
    franklin?: FranklinCredentials;
    siteID?: string;
}

export const fetchSettings = async (siteID?: string): Promise<Settings> => {
    const query = new URLSearchParams();
    if (siteID) {
        query.append('siteID', siteID);
    }
    const response = await fetch(`/api/settings?${query.toString()}`);
    if (!response.ok) {
        throw new Error(await extractError(response, 'Failed to fetch settings'));
    }
    return response.json();
};

export const updateSettings = async (settings: Settings, siteID?: string, franklin?: FranklinCredentials): Promise<void> => {
    const payload: any = {
        ...settings,
        siteID: siteID,
    };

    if (siteID) {
        payload.siteID = siteID;
    }

    if (franklin) {
        payload.franklin = franklin;
    }

    const response = await fetch('/api/settings', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
    });
    if (!response.ok) {
        throw new Error(await extractError(response, 'Failed to update settings'));
    }
};

export interface AuthStatus {
    loggedIn: boolean;
    email: string;
    authRequired: boolean;
    clientID: string;
    siteIDs?: string[];
}

export const fetchAuthStatus = async (): Promise<AuthStatus> => {
    const response = await fetch('/api/auth/status');
    if (!response.ok) {
        throw new Error(await extractError(response, 'Failed to fetch auth status'));
    }
    return response.json();
};

export const login = async (token: string): Promise<void> => {
    const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ token }),
    });
    if (!response.ok) {
        throw new Error(await extractError(response, 'Login failed'));
    }
};

export const logout = async (): Promise<void> => {
    const response = await fetch('/api/auth/logout', {
        method: 'POST',
    });
    if (!response.ok) {
        throw new Error(await extractError(response, 'Logout failed'));
    }
};

export interface ModelingHour {
    ts: string;
    hour: number;
    netLoadSolarKWH: number;
    gridChargeDollarsPerKWH: number;
    solarOppDollarsPerKWH: number;
    avgHomeLoadKWH: number;
    predictedSolarKWH: number;
    batteryKWH: number;
    batteryCapacityKWH: number;
    batteryReserveKWH: number;
    todaySolarTrend: number;
}

export const fetchModeling = async (siteID?: string): Promise<ModelingHour[]> => {
    const query = new URLSearchParams();
    if (siteID) {
        query.append('siteID', siteID);
    }
    const response = await fetch(`/api/modeling?${query.toString()}`);
    if (!response.ok) {
        throw new Error(await extractError(response, 'Failed to fetch modeling data'));
    }
    return response.json();
};

export const joinSite = async (joinSiteID: string, inviteCode: string): Promise<void> => {
    const response = await fetch('/api/join', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ joinSiteID, inviteCode }),
    });
    if (!response.ok) {
        throw new Error(await extractError(response, 'Failed to join site'));
    }
};
