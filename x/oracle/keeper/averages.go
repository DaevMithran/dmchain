package keeper

import (
	"errors"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"
	"github.com/DaevMithran/dmchain/util"
	"github.com/DaevMithran/dmchain/x/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type WmaStrategy string

const (
	WmaStrategyOldest   WmaStrategy = "OLDEST"
	WmaStrategyRecent   WmaStrategy = "RECENT"
	WmaStrategyBalanced WmaStrategy = "BALANCED"
	WmaStrategyCustom   WmaStrategy = "CUSTOM"
)

const (
	AvgTypeSMA = "sma"
	AvgTypeEMA = "ema"
	AvgTypeWMA = "wma"
)

func (k Keeper) ComputeAverages(ctx sdk.Context, denom string) error {
	currentBlock := util.SafeInt64ToUint64(ctx.BlockHeight())

	// Collect last N prices
	prices, err := CalculateHistoricPrices(ctx, denom, currentBlock, k)
	if err != nil {
		ctx.Logger().Error("Failed to calculate historic prices", "denom", denom, "error", err)
		return nil
	}
	if len(prices) == 0 {
		ctx.Logger().Error("No historic prices for denom", "denom", denom)
		return nil
	}
	// calculate sma and store it
	sma := CalculateSMA(prices)
	k.SetSMA(ctx, denom, sma)

	// Calculate WMA for all strategies
	strategies := []string{string(WmaStrategyBalanced), string(WmaStrategyOldest), string(WmaStrategyRecent)}

	for _, strategy := range strategies {
		if !IsValidWmaStrategy(strategy) {
			return types.ErrInvalidWmaStrategy.Wrapf("invalid WMA strategy: %s", strategy)
		}
		wma := CalculateWMA(prices, string(strategy), nil)
		// Key: "denom", "wma:BALANCED"
		key := collections.Join(denom, fmt.Sprintf("%s:%s", AvgTypeWMA, strategy))
		_ = k.Averages.Set(ctx, key, wma)
	}

	// 3. EMA (smoothing factor α = 2 / (N + 1))
	prevEMA, present := k.GetEMA(ctx, denom)

	ema := CalculateEMA(prevEMA, present, prices)
	k.SetEMA(ctx, denom, ema)
	return nil
}

func CalculateHistoricPrices(ctx sdk.Context, denom string, currentBlock uint64, k Keeper) ([]math.LegacyDec, error) {
	var prices []math.LegacyDec
	// Get the last recorded block for this denom
	lastBlock, err := k.LastHistoricPriceBlock.Get(ctx, denom)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			k.Logger(ctx).Error("No historic prices recorded", "denom", denom)
			return []math.LegacyDec{}, nil
		}
		return nil, err
	}

	params := k.GetParams(ctx)
	averagingWindow := params.AveragingWindow
	stampPeriod := types.DefaultParams().HistoricStampPeriod

	// Iterate through the window and fetch prices
	for i := uint64(0); i < averagingWindow; i++ {
		// Calculate the specific block height we are looking for
		targetBlock := lastBlock - (i * stampPeriod)
		// Use the Pair key: Pair(denom, targetBlock)
		price, err := k.HistoricPrices.Get(ctx, collections.Join(denom, targetBlock))
		if err != nil {
			// If a specific stamp is missing, we continue (matching original logic)
			if errors.Is(err, collections.ErrNotFound) {
				continue
			}
			return nil, err
		}

		prices = append(prices, price)
	}
	return prices, nil
}

func CalculateEMA(previousEMA math.LegacyDec, present bool, prices []math.LegacyDec) math.LegacyDec {
	if len(prices) == 0 {
		return math.LegacyZeroDec()
	}

	// Initialize EMA with previous value or first price
	ema := previousEMA
	if !present {
		ema = prices[0]
	}

	alpha := math.LegacyNewDecWithPrec(2, 0).QuoInt64(int64(len(prices) + 1))

	for i := 1; i < len(prices); i++ {
		ema = prices[i].Mul(alpha).Add(ema.Mul(math.LegacyOneDec().Sub(alpha)))
	}

	return ema
}

func (k Keeper) SetSMA(ctx sdk.Context, denom string, value math.LegacyDec) {
	_ = k.Averages.Set(ctx, collections.Join(denom, AvgTypeSMA), value)
}

func (k Keeper) SetEMA(ctx sdk.Context, denom string, value math.LegacyDec) {
	_ = k.Averages.Set(ctx, collections.Join(denom, AvgTypeEMA), value)
}

func (k Keeper) SetWMA(ctx sdk.Context, denom string, strategy WmaStrategy, value math.LegacyDec) {
	_ = k.Averages.Set(ctx, collections.Join(denom, fmt.Sprintf("wma:%s:%s", AvgTypeWMA, strategy)), value)
}

func (k Keeper) GetSMA(ctx sdk.Context, denom string) (math.LegacyDec, bool) {
	val, err := k.Averages.Get(ctx, collections.Join(denom, AvgTypeSMA))
	return val, err == nil
}

func (k Keeper) GetEMA(ctx sdk.Context, denom string) (math.LegacyDec, bool) {
	val, err := k.Averages.Get(ctx, collections.Join(denom, AvgTypeEMA))
	return val, err == nil
}

func (k Keeper) GetWMA(ctx sdk.Context, denom string, strategy string) (math.LegacyDec, bool) {
	val, err := k.Averages.Get(ctx, collections.Join(denom, AvgTypeWMA))
	return val, err == nil
}

func CalculateSMA(prices []math.LegacyDec) math.LegacyDec {
	sum := math.LegacyZeroDec()
	for _, p := range prices {
		sum = sum.Add(p)
	}
	sma := sum.QuoInt64(int64(len(prices)))
	return sma
}

func CalculateWMA(prices []math.LegacyDec, strategy string, customWeights []int64) math.LegacyDec {
	n := len(prices)
	if n == 0 {
		return math.LegacyZeroDec()
	}

	weightedSum := math.LegacyZeroDec()
	weightTotal := int64(0)

	switch strategy {
	case "OLDEST":
		// Weights: [N, N-1, ..., 1]
		for i := 0; i < n; i++ {
			weight := int64(n - i)
			weightedSum = weightedSum.Add(prices[i].MulInt64(weight))
			weightTotal += weight
		}

	case "RECENT":
		// Weights: [1, 2, ..., N]
		for i := 0; i < n; i++ {
			weight := int64(i + 1)
			weightedSum = weightedSum.Add(prices[i].MulInt64(weight))
			weightTotal += weight
		}

	case "BALANCED":
		// Weights: [1–10], then ten × 10s, then [9–1] to make 30 entries
		// Adapt to whatever len(prices) is, but assume 30 ideal entries
		weights := make([]int64, n)
		for i := 0; i < n; i++ {
			switch {
			case i < 10:
				weights[i] = int64(i + 1)
			case i < 20:
				weights[i] = 10
			default:
				weights[i] = int64(30 - i)
			}
		}

		for i := 0; i < n; i++ {
			weightedSum = weightedSum.Add(prices[i].MulInt64(weights[i]))
			weightTotal += weights[i]
		}

	case "CUSTOM":
		// Use customWeights array provided by governance param or config
		if len(customWeights) != n {
			panic(fmt.Sprintf("custom weight length %d does not match price list length %d", len(customWeights), n))
		}

		for i := 0; i < n; i++ {
			weight := customWeights[i]
			weightedSum = weightedSum.Add(prices[i].MulInt64(weight))
			weightTotal += weight
		}

	default:
		panic(fmt.Sprintf("unsupported WMA strategy: %s", strategy))
	}

	return weightedSum.QuoInt64(weightTotal)
}

func IsValidWmaStrategy(s string) bool {
	switch WmaStrategy(s) {
	case WmaStrategyOldest, WmaStrategyRecent, WmaStrategyBalanced, WmaStrategyCustom:
		return true
	default:
		return false
	}
}

func (k Keeper) GetPriceHistory(ctx sdk.Context, denom string) []math.LegacyDec {
	currentBlock := util.SafeInt64ToUint64(ctx.BlockHeight())

	prices, err := CalculateHistoricPrices(ctx, denom, currentBlock, k)
	if err != nil {
		ctx.Logger().Error("Failed to fetch price history", "denom", denom, "error", err)
		return nil
	}
	return prices
}

func (k Keeper) GetCustomWMA(ctx sdk.Context, denom string, weights []int32) (math.LegacyDec, error) {
	prices := k.GetPriceHistory(ctx, denom)
	if len(prices) == 0 {
		return math.LegacyZeroDec(), fmt.Errorf("no price history for %s", denom)
	}
	if len(prices) != len(weights) {
		return math.LegacyZeroDec(), fmt.Errorf("weights length %d ≠ prices %d", len(weights), len(prices))
	}

	wInt64 := make([]int64, len(weights))
	for i, w := range weights {
		wInt64[i] = int64(w)
	}

	return CalculateWMA(prices, "CUSTOM", wInt64), nil
}
