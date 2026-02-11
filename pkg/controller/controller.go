package controller

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/jameshartig/autoenergy/pkg/types"
)

// Decision represents the result of the decision logic.
type Decision struct {
	Action      types.Action
	Explanation string
}

// Controller handles the decision-making logic for the ESS.
type Controller struct {
}

// NewController creates a new Controller.
func NewController() *Controller {
	return &Controller{}
}

// Decide determines the best action to take based on current state and history.
func (c *Controller) Decide(
	ctx context.Context,
	currentStatus types.SystemStatus,
	currentPrice types.Price,
	futurePrices []types.Price,
	history []types.EnergyStats,
	settings types.Settings,
) (Decision, error) {
	slog.DebugContext(ctx, "controller decide started",
		slog.Float64("soc", currentStatus.BatterySOC),
		slog.Float64("batteryKW", currentStatus.BatteryKW),
		slog.Float64("solarKW", currentStatus.SolarKW),
		slog.Float64("homeKW", currentStatus.HomeKW),
		slog.Float64("currentPrice", currentPrice.DollarsPerKWH),
	)

	now := time.Now().In(currentStatus.Timestamp.Location())

	solarMode := types.SolarModeAny
	if !settings.GridExportSolar {
		solarMode = types.SolarModeNoExport
	}

	// Rule 1: If the price is negative, then don't export anything to the grid.
	if currentPrice.DollarsPerKWH < 0 {
		solarMode = types.SolarModeNoExport
		slog.DebugContext(ctx, "price is negative, disabling solar export", slog.Float64("price", currentPrice.DollarsPerKWH))
		// We do NOT return here. We fall through to allow charging logic to trigger.
	}

	// Helper to determine final action with "No Change" optimizations
	finalizeAction := func(batteryMode types.BatteryMode, modeReason string, explanation string) Decision {
		finalBatMode := batteryMode
		switch batteryMode {
		case types.BatteryModeChargeAny:
			// If we want to charge, and we are already charging (negative BatteryKW),
			// then don't change anything.
			// we might not be charging if Battery is already full
			// also make sure we've elevated the min SOC to force charging
			if (currentStatus.BatteryKW < 0 || currentStatus.BatterySOC >= 99) && currentStatus.ElevatedMinBatterySOC && (!settings.GridChargeBatteries || currentStatus.CanImportBattery) {
				finalBatMode = types.BatteryModeNoChange
			}
		case types.BatteryModeChargeSolar:
			// If we want to charge from solar, and we are already charging from
			// only solar (negative BatteryKW), then don't change anything.
			// we might not be charging if Battery is already full
			// also make sure we've elevated the min SOC to force charging
			if (currentStatus.BatteryKW < 0 || currentStatus.BatterySOC >= 99) && currentStatus.ElevatedMinBatterySOC && !currentStatus.CanImportBattery {
				finalBatMode = types.BatteryModeNoChange
			}
		case types.BatteryModeStandby:
			// If we want to standby:
			// 1. If charging (BatteryKW < 0), we must change to Stop charging.
			// 2. If effectively charging from grid, we want to stop
			// 3. If charging from solar, we can't stop that so assume standby
			// 4. If Idle (BatteryKW == 0), return NoChange.

			// battery is charging from the grid if the battery charge rate exceeds
			// the solar surplus (solar generation minus home consumption)
			// give a little bit of tolerance to account for energy losses/floats/etc
			isChargingFromGrid := false
			if currentStatus.BatteryKW < -0.1 && currentStatus.GridKW > 0 {
				solarSurplus := currentStatus.SolarKW - currentStatus.HomeKW
				// remember BatteryKW is negative when charging
				// give a little bit of tolerance to account for energy losses/floats/etc
				if solarSurplus < 0 || solarSurplus+currentStatus.BatteryKW > 0.1 {
					isChargingFromGrid = true
				}
			}

			slog.DebugContext(
				ctx,
				"determined if we are charging from grid for standby calculation",
				slog.Float64("batteryKW", currentStatus.BatteryKW),
				slog.Float64("gridKW", currentStatus.GridKW),
				slog.Float64("solarKW", currentStatus.SolarKW),
				slog.Float64("homeKW", currentStatus.HomeKW),
				slog.Bool("isChargingFromGrid", isChargingFromGrid),
				slog.Float64("batterySOC", currentStatus.BatterySOC),
				slog.Bool("batteryAboveMinSOC", currentStatus.BatteryAboveMinSOC),
				slog.Bool("elevatedMinBatterySOC", currentStatus.ElevatedMinBatterySOC),
			)

			if currentStatus.BatteryKW > 0 {
				// we're using the battery but it might be because we're greater than
				// the elevated reserve SOC and maybe solar was charging us up
				if currentStatus.BatteryAboveMinSOC && currentStatus.ElevatedMinBatterySOC {
					// we're already above reserve SOC and we've elevated the reserve SOC
					// probably because of a previous standby request
					finalBatMode = types.BatteryModeNoChange
				}
				// discharging, switch to standby
			} else if isChargingFromGrid {
				// charging from grid, switch to standby
			} else if currentStatus.BatteryKW < 0 {
				// charging from solar (not grid), ignore
				finalBatMode = types.BatteryModeNoChange
			} else {
				// already standby, ignore
				finalBatMode = types.BatteryModeNoChange
			}
		case types.BatteryModeNoChange:
			// nothing to do
		case types.BatteryModeLoad:
			slog.DebugContext(
				ctx,
				"determined if we are using the battery as much as possible",
				slog.Float64("batterySOC", currentStatus.BatterySOC),
				slog.Float64("minBatterySOC", settings.MinBatterySOC),
				slog.Bool("elevatedMinBatterySOC", currentStatus.ElevatedMinBatterySOC),
				slog.Bool("gridChargeBatteries", settings.GridChargeBatteries),
				slog.Bool("canImportBattery", currentStatus.CanImportBattery),
			)
			// if the minimum SOC is not elevated then we're already using the battery
			// as much as possible
			if !currentStatus.ElevatedMinBatterySOC && (!settings.GridChargeBatteries || currentStatus.CanImportBattery) {
				finalBatMode = types.BatteryModeNoChange
			}
		default:
		}

		// Check Solar Mode
		finalSolarMode := solarMode
		switch solarMode {
		case types.SolarModeNoExport:
			if !currentStatus.CanExportSolar {
				finalSolarMode = types.SolarModeNoChange
			}
		case types.SolarModeAny:
			if currentStatus.CanExportSolar {
				finalSolarMode = types.SolarModeNoChange
			}
		case types.SolarModeNoChange:
			// nothing to do
		}

		return Decision{
			Action: types.Action{
				Timestamp:    now.UTC(),
				BatteryMode:  finalBatMode,
				SolarMode:    finalSolarMode,
				Description:  modeReason,
				CurrentPrice: currentPrice,
			},
			Explanation: explanation,
		}
	}

	// Rule 2: If the price is below the Always Charge Threshold, then charge the
	// battery.
	if currentPrice.DollarsPerKWH < settings.AlwaysChargeUnderDollarsPerKWH {
		desc := fmt.Sprintf(
			"Price Low (%.3f < %.3f). Charging.",
			currentPrice.DollarsPerKWH,
			settings.AlwaysChargeUnderDollarsPerKWH,
		)
		if solarMode == types.SolarModeNoExport {
			desc += " (Export Disabled due to Negative Price)"
		}
		// If negative, we charge.
		slog.DebugContext(ctx, "price below always charge threshold", slog.Float64("price", currentPrice.DollarsPerKWH), slog.Float64("threshold", settings.AlwaysChargeUnderDollarsPerKWH))
		return finalizeAction(types.BatteryModeChargeAny, desc, "Always Charge Threshold"), nil
	}

	// Rule 3: Charge now if its cheaper than later, if we will run out of energy
	// or if we can make more money buying now and selling later (arbitrage)

	capacityKWH := currentStatus.BatteryCapacityKWH
	if capacityKWH <= 0 {
		return finalizeAction(types.BatteryModeStandby, "Battery Config Missing or Capacity 0. Standby.", "Zero Battery Capacity"), nil
	}

	chargeKW := currentStatus.MaxBatteryChargeKW
	if chargeKW <= 0 {
		// conservatively assume it takes 3 hours to charge the battery from 0->100
		chargeKW = capacityKWH / 3.0
	}

	simData := c.SimulateState(ctx, now, currentStatus, currentPrice, futurePrices, history, settings)

	chargeNowCost := currentPrice.DollarsPerKWH + settings.AdditionalFeesDollarsPerKWH
	shouldCharge := false
	chargeReason := ""

	maxFuturePrice := currentPrice.DollarsPerKWH
	for _, slot := range simData {
		if slot.GridChargeDollarsPerKWH > maxFuturePrice {
			maxFuturePrice = slot.GridChargeDollarsPerKWH
		}
	}

	// track simulated energy
	minKWH := capacityKWH * (settings.MinBatterySOC / 100.0)
	var hitDeficitAt time.Time
	var hitCapacityAt time.Time
	minEnergy := -1.0
	maxEnergy := -1.0

	// track the costs to charge until/including the simulated hour
	chargeCosts := make([]float64, 0, len(simData))

	for _, slot := range simData {
		chargeCosts = append(chargeCosts, slot.GridChargeDollarsPerKWH)

		// update simulated energy state
		// if we ever hit the capacity of the battery, we can't store any more power
		// so we set hitCapacity to true so we never try to charge since that power
		// would be meaningless to pull from the grid since we end up filling up
		// the batteries without the grid in the simulation anyways
		if slot.HitCapacity && hitCapacityAt.IsZero() {
			slog.DebugContext(
				ctx,
				"simulated energy hit capacity",
				slog.Float64("batteryKWH", slot.BatteryKWH),
				slog.Float64("capacityKWH", capacityKWH),
				slog.Int("simHour", slot.Hour),
			)
			hitCapacityAt = slot.TS
		}

		if minEnergy == -1 || slot.BatteryKWH < minEnergy {
			minEnergy = slot.BatteryKWH
		}
		if maxEnergy == -1 || slot.BatteryKWH > maxEnergy {
			maxEnergy = slot.BatteryKWH
		}

		// check if we are below the minimum SOC and when we need to charge
		if slot.HitDeficit && hitDeficitAt.IsZero() {
			slog.DebugContext(
				ctx,
				"simulated energy below minimum SOC causing a deficit",
				slog.Float64("batteryKWH", slot.BatteryKWH),
				slog.Float64("minKWH", minKWH),
				slog.Int("simHour", slot.Hour),
			)
			hitDeficitAt = slot.TS
		}

		if slot.BatteryDeficitKWH > 0 {
			deficitAmount := slot.BatteryDeficitKWH

			// only consider charging if GridCharging is enabled
			if settings.GridChargeBatteries {
				sort.Float64s(chargeCosts)
				var cheapestChargeCost float64

				// factor in the cost of charging for the duration of the charge which
				// means we need to look at the nth cheapest charge cost
				// round up the hours we need to charge except for a little buffer
				chargeDurationHours := max(1, int((float64(deficitAmount)/chargeKW + 0.84)))
				if chargeDurationHours > len(chargeCosts) {
					cheapestChargeCost = chargeCosts[len(chargeCosts)-1]
				} else {
					cheapestChargeCost = chargeCosts[chargeDurationHours-1]
				}

				// if we have determined we'll run out of energy and it's cheaper to
				// charge now than later, charge now
				if chargeNowCost+settings.MinDeficitPriceDifferenceDollarsPerKWH <= cheapestChargeCost {
					shouldCharge = true
					chargeReason = fmt.Sprintf("Projected Deficit at %s. Charge Now ($%.3f) <= Later ($%.3f) - Delta ($%.3f).", slot.TS.Format(time.Kitchen), chargeNowCost, cheapestChargeCost, settings.MinDeficitPriceDifferenceDollarsPerKWH)
					slog.DebugContext(
						ctx,
						"deficit predicted, charging now",
						slog.Float64("deficit", deficitAmount),
						slog.Time("deficitAt", hitDeficitAt),
						slog.Float64("chargeCost", chargeNowCost),
						slog.Float64("cheapestFutureCost", cheapestChargeCost),
					)
					break
				} else {
					slog.DebugContext(
						ctx,
						"deficit predicted, charging later",
						slog.Float64("deficit", deficitAmount),
						slog.Time("deficitAt", hitDeficitAt),
						slog.Float64("chargeCost", chargeNowCost),
						slog.Float64("cheapestFutureCost", cheapestChargeCost),
						slog.Int("chargeDurationHours", chargeDurationHours),
					)
				}
			}
		}

		// at this point it's opportunity cost because we either have enough energy
		// or it'll be cheaper later to charge

		// assume we need to charge for at least 10 minutes for it to be worth it
		chargeDurationHours := 10.0 / 60.0
		simEnergyAfterCharge := slot.BatteryKWH + chargeKW*chargeDurationHours

		// make sure we can charge the batteries, we can export solar, and we have
		// enough headroom to charge
		if settings.GridChargeBatteries && settings.GridExportSolar && simEnergyAfterCharge < capacityKWH && hitCapacityAt.IsZero() {
			var value float64
			// if we are importing, we avoid the import cost
			// if we are exporting, we get the export value
			if slot.NetLoadSolarKWH > 0 {
				value = slot.GridChargeDollarsPerKWH
			} else {
				value = slot.SolarOppDollarsPerKWH
			}

			// if the value we get later minus our cost to charge now is greater than
			// the minimum arbitrage difference, we should charge now
			if value-chargeNowCost > settings.MinArbitrageDifferenceDollarsPerKWH {
				shouldCharge = true
				chargeReason = fmt.Sprintf("Arbitrage Opportunity at %s. Buy@%.3f -> Sell/Save@%.3f.", slot.TS.Format(time.Kitchen), chargeNowCost, value)
				slog.DebugContext(
					ctx,
					"arbitrage opportunity found",
					slog.Float64("buyAt", chargeNowCost),
					slog.Float64("sellAt", value),
					slog.Float64("diff", value-chargeNowCost),
				)
				break
			} else {
				slog.DebugContext(
					ctx,
					"arbitrage opportunity too small",
					slog.Float64("buyAt", chargeNowCost),
					slog.Float64("sellAt", value),
					slog.Float64("minDiff", settings.MinArbitrageDifferenceDollarsPerKWH),
				)
			}
		}
	}

	// if we should charge, return now.
	if shouldCharge {
		desc := fmt.Sprintf("Charging Optimized: %s", chargeReason)
		return finalizeAction(types.BatteryModeChargeAny, desc, "Simulation Optimized Charge"), nil
	}

	// Rule 4: Logic for Battery Usage vs Standby
	// If we have plenty of battery (no deficit), Use it (Load).
	// If we have a deficit, but we are at the Highest Price, Use it (Load).
	// If we have a deficit, and cheaper now than later, Standby (Save for later).

	if !hitDeficitAt.IsZero() {
		// Optimization: If we hit full capacity BEFORE we hit the deficit, then
		// the current energy we have in the battery is "use it or lose it" effectively,
		// because we will refill to 100% anyway. So we should NOT Standby to save THIS energy.
		if !hitCapacityAt.IsZero() && hitCapacityAt.Before(hitDeficitAt) {
			slog.DebugContext(
				ctx,
				"deficit predicted but will refill to capacity before then",
				slog.Time("hitCapacityAt", hitCapacityAt),
				slog.Time("hitDeficitAt", hitDeficitAt),
			)
			loadReason := fmt.Sprintf("Capacity hit at %s before deficit at %s.", hitCapacityAt.Format(time.Kitchen), hitDeficitAt.Format(time.Kitchen))
			return finalizeAction(types.BatteryModeLoad, loadReason, "Use battery before capacity hit"), nil
		}

		// We are going to run out. Should we save it?
		// Check if there is a significantly more expensive time later.
		// If current price is lower than maxFuturePrice, we should probably save it.
		currentChargeCost := currentPrice.DollarsPerKWH + settings.AdditionalFeesDollarsPerKWH
		if currentChargeCost < maxFuturePrice {
			standbyReason := fmt.Sprintf("Deficit predicted at %s and higher prices later ($%.3f < $%.3f).", hitDeficitAt.Format(time.Kitchen), currentChargeCost, maxFuturePrice)
			slog.DebugContext(
				ctx,
				"deficit predicted, saving for peak",
				slog.Float64("currentPrice", currentPrice.DollarsPerKWH),
				slog.Float64("maxFuturePrice", maxFuturePrice),
			)
			return finalizeAction(types.BatteryModeStandby, standbyReason, "Deficit + Save for Peak"), nil
		}
		// If we are at the peak (or flat), use it until empty.
		slog.DebugContext(
			ctx,
			"deficit predicted but at peak price",
			slog.Float64("currentPrice", currentPrice.DollarsPerKWH),
		)
		return finalizeAction(types.BatteryModeLoad, "Deficit predicted but Current Price is Peak.", "Use Battery at Peak"), nil
	}

	// No deficit predicted, use battery.
	slog.DebugContext(
		ctx,
		"no deficit predicted, using battery",
		slog.Float64("minEnergy", minEnergy),
		slog.Float64("maxEnergy", maxEnergy),
	)
	return finalizeAction(types.BatteryModeLoad, "Sufficient battery.", "No deficit predicted"), nil
}
