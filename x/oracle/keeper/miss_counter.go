package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetMissCounter retrieves the # of vote periods missed in this oracle slash
// window.
func (k Keeper) GetMissCounter(ctx sdk.Context, operator sdk.ValAddress) uint64 {
	value, err := k.MissCounters.Get(ctx, operator)
	if err != nil {
		return 0
	}
	return value
}

// SetMissCounter updates the # of vote periods missed in this oracle slash
// window.
func (k Keeper) SetMissCounter(ctx sdk.Context, operator sdk.ValAddress, missCounter uint64) {
	_ = k.MissCounters.Set(ctx, operator, missCounter)
}

// DeleteMissCounter removes miss counter for the validator.
func (k Keeper) DeleteMissCounter(ctx sdk.Context, operator sdk.ValAddress) {
	_ = k.MissCounters.Remove(ctx, operator)
}

// IterateMissCounters iterates over the miss counters and performs a callback
// function.
func (k Keeper) IterateMissCounters(ctx sdk.Context, handler func(sdk.ValAddress, uint64) bool) {
	err := k.MissCounters.Walk(ctx, nil, func(operator sdk.ValAddress, missCounter uint64) (stop bool, err error) {
		return handler(operator, missCounter), nil
	})
	if err != nil {
		k.Logger(ctx).Error("failed to iterate miss counters", "error", err)
	}
}
