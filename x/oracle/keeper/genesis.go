package keeper

import (
	"cosmossdk.io/collections"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/DaevMithran/dmchain/x/oracle/types"
)

// IterateAllHistoricPrices iterates over all historic prices.
// Iterator stops when exhausting the source, or when the handler returns `true`.
func (k Keeper) IterateAllHistoricPrices(
	ctx sdk.Context,
	handler func(types.PriceStamp) bool,
) {
	err := k.HistoricPrices.Walk(ctx, nil, func(key collections.Pair[string, uint64], value math.LegacyDec) (stop bool, err error) {
		denom := key.K1()
		blockNum := key.K2()

		historicPrice := types.PriceStamp{
			ExchangeRate: &sdk.DecCoin{Denom: denom, Amount: value},
			BlockNum:     blockNum,
		}
		return handler(historicPrice), nil
	})
	if err != nil {
		k.Logger(ctx).Error("failed to iterate historic prices", "error", err)
	}
}

// AllHistoricPrices collects and returns all historic prices.
func (k Keeper) AllHistoricPrices(ctx sdk.Context) types.PriceStamps {
	prices := types.PriceStamps{}
	k.IterateAllHistoricPrices(ctx, func(price types.PriceStamp) (stop bool) {
		prices = append(prices, price)
		return false
	})
	return prices
}

// IterateAllMedianPrices iterates over all median prices.
func (k Keeper) IterateAllMedianPrices(
	ctx sdk.Context,
	handler func(types.PriceStamp) bool,
) {
	err := k.MedianPrices.Walk(ctx, nil, func(key collections.Pair[string, uint64], value math.LegacyDec) (stop bool, err error) {
		median := types.PriceStamp{
			ExchangeRate: &sdk.DecCoin{Denom: key.K1(), Amount: value},
			BlockNum:     key.K2(),
		}
		return handler(median), nil
	})
	if err != nil {
		k.Logger(ctx).Error("failed to iterate median prices", "error", err)
	}
}

// AllMedianPrices collects and returns all median prices.
func (k Keeper) AllMedianPrices(ctx sdk.Context) types.PriceStamps {
	prices := types.PriceStamps{}
	k.IterateAllMedianPrices(ctx, func(median types.PriceStamp) (stop bool) {
		prices = append(prices, median)
		return false
	})
	return prices
}

// IterateAllMedianDeviationPrices iterates over all median deviation prices.
func (k Keeper) IterateAllMedianDeviationPrices(
	ctx sdk.Context,
	handler func(types.PriceStamp) bool,
) {
	err := k.MedianDeviations.Walk(ctx, nil, func(key collections.Pair[string, uint64], value math.LegacyDec) (stop bool, err error) {
		medianDeviation := types.PriceStamp{
			ExchangeRate: &sdk.DecCoin{Denom: key.K1(), Amount: value},
			BlockNum:     key.K2(),
		}
		return handler(medianDeviation), nil
	})
	if err != nil {
		k.Logger(ctx).Error("failed to iterate median deviation prices", "error", err)
	}
}

// AllMedianDeviationPrices collects and returns all median deviation prices.
func (k Keeper) AllMedianDeviationPrices(ctx sdk.Context) types.PriceStamps {
	prices := types.PriceStamps{}
	k.IterateAllMedianDeviationPrices(ctx, func(median types.PriceStamp) (stop bool) {
		prices = append(prices, median)
		return false
	})
	return prices
}
