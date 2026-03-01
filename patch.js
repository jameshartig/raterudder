const fs = require('fs');
const content = fs.readFileSync('web/src/utils/dashboardUtils.ts', 'utf8');

const target = `
        case ActionReason.WaitingToCharge: {
            const delta = nowCost !== null && futureCost !== null ? futureCost - nowCost : null;
            const parts = [
                \`A cheaper charging window is coming up\${futureCostStr ? \` at \${futureCostStr}\` : ''} which is cheaper than now (\${nowCostStr}).\`,
                \`Holding off charging the batteries until then.\`,
            ];
            if (delta !== null) parts.push(\`Estimated savings: \${formatPrice(delta)}/kWh.\`);
            return parts.concat(suffixParts).join(' ');
        }
`;

const replacement = `
        case ActionReason.WaitingToCharge: {
            const delta = nowCost !== null && futureCost !== null ? nowCost - futureCost : null;

            let parts: string[] = [];
            if (delta !== null && delta < 0.01) {
                parts = [
                    \`A charging window is coming up which is similar in price or cheaper than now.\`,
                    \`Holding off charging the batteries until then.\`,
                ];
            } else {
                parts = [
                    \`A cheaper charging window is coming up\${futureCostStr ? \` at \${futureCostStr}\` : ''} which is cheaper than now (\${nowCostStr}).\`,
                    \`Holding off charging the batteries until then.\`,
                ];
                if (delta !== null) parts.push(\`Estimated savings: \${formatPrice(delta)}/kWh.\`);
            }
            return parts.concat(suffixParts).join(' ');
        }
`;

if (content.includes("futureCost - nowCost : null;") && content.includes("WaitingToCharge:")) {
    const startIdx = content.indexOf("case ActionReason.WaitingToCharge:");
    const endIdx = content.indexOf("case ActionReason.ChargeSurvivePeak:");

    if (startIdx !== -1 && endIdx !== -1) {
        const oldSection = content.slice(startIdx, endIdx);
        // Replace it
        const newSection = `case ActionReason.WaitingToCharge: {
            const delta = nowCost !== null && futureCost !== null ? nowCost - futureCost : null;

            let parts: string[] = [];
            if (delta !== null && delta < 0.01) {
                parts = [
                    \`A charging window is coming up which is similar in price or cheaper than now.\`,
                    \`Holding off charging the batteries until then.\`,
                ];
            } else {
                parts = [
                    \`A cheaper charging window is coming up\${futureCostStr ? \` at \${futureCostStr}\` : ''} which is cheaper than now (\${nowCostStr}).\`,
                    \`Holding off charging the batteries until then.\`,
                ];
                if (delta !== null) parts.push(\`Estimated savings: \${formatPrice(delta)}/kWh.\`);
            }
            return parts.concat(suffixParts).join(' ');
        }
        `;
        const updatedContent = content.slice(0, startIdx) + newSection + content.slice(endIdx);
        fs.writeFileSync('web/src/utils/dashboardUtils.ts', updatedContent, 'utf8');
        console.log('Patched dashboardUtils.ts successfully.');
    }
}
