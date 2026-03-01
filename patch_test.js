const fs = require('fs');
const content = fs.readFileSync('web/src/utils/dashboardUtils.test.ts', 'utf8');

const target = "it('appends NoExport suffix for arbitrage', () => {";
const insertion = `
        it('handles WaitingToCharge with significant savings', () => {
            const action = {
                ...baseAction,
                reason: ActionReason.WaitingToCharge,
                currentPrice: { dollarsPerKWH: 0.15, gridUseDollarsPerKWH: 0, tsStart: '', tsEnd: '' },
                futurePrice: { dollarsPerKWH: 0.05, gridUseDollarsPerKWH: 0, tsStart: '', tsEnd: '' }
            };
            const text = getReasonText(action);
            expect(text).toContain('A cheaper charging window is coming up');
            expect(text).toContain('savings: $ 0.100/kWh');
        });

        it('handles WaitingToCharge with < $0.01 savings', () => {
            const action = {
                ...baseAction,
                reason: ActionReason.WaitingToCharge,
                currentPrice: { dollarsPerKWH: 0.092, gridUseDollarsPerKWH: 0, tsStart: '', tsEnd: '' },
                futurePrice: { dollarsPerKWH: 0.091, gridUseDollarsPerKWH: 0, tsStart: '', tsEnd: '' }
            };
            const text = getReasonText(action);
            expect(text).toContain('A charging window is coming up which is similar in price or cheaper than now');
            expect(text).not.toContain('savings:');
            expect(text).not.toContain('$ 0.091');
            expect(text).not.toContain('$ 0.092');
        });

`;

if (content.includes(target)) {
    const updatedContent = content.replace(target, insertion + "        " + target);
    fs.writeFileSync('web/src/utils/dashboardUtils.test.ts', updatedContent, 'utf8');
    console.log('Patched dashboardUtils.test.ts successfully.');
} else {
    console.log('Target not found.');
}
