package keeper

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/DaevMithran/dmchain/util"
	"github.com/DaevMithran/dmchain/x/oracle/types"
)

// SlashAndResetMissCounters iterates over all the current missed counters and
// calculates the "valid vote rate" as:
// (possibleWinsPerSlashWindow - missCounter)/possibleWinsPerSlashWindow.
//
// If the valid vote rate is below the minValidPerWindow, the validator will be
// slashed and jailed.
func (k Keeper) SlashAndResetMissCounters(ctx sdk.Context) {
	possibleWins := k.PossibleWinsPerSlashWindow(ctx)
	minValidRate := k.MinValidPerWindow(ctx)

	distributionHeight := ctx.BlockHeight() - sdk.ValidatorUpdateDelay - 1
	slashFraction := k.SlashFraction(ctx)
	powerReduction := k.StakingKeeper.PowerReduction(ctx)

	k.IterateMissCounters(ctx, func(operator sdk.ValAddress, missCount uint64) bool {
		k.evaluateAndSlashIfNeeded(ctx, operator, missCount, possibleWins, minValidRate, distributionHeight, slashFraction, powerReduction)
		k.DeleteMissCounter(ctx, operator)
		return false
	})
}

func (k Keeper) evaluateAndSlashIfNeeded(
	ctx sdk.Context,
	operator sdk.ValAddress,
	missCount uint64,
	possibleWinsPerSlashWindow int64,
	minValidRate math.LegacyDec,
	distributionHeight int64,
	slashFraction math.LegacyDec,
	powerReduction math.Int,
) {
	validVotes := math.NewInt(possibleWinsPerSlashWindow - util.SafeUint64ToInt64(missCount))
	validRate := math.LegacyNewDecFromInt(validVotes).QuoInt64(possibleWinsPerSlashWindow)

	if !validRate.LT(minValidRate) {
		return // Validator is safe
	}

	oracleParams := k.GetParams(ctx)

	validator, err := k.StakingKeeper.Validator(ctx, operator)
	if err != nil || !validator.IsBonded() || validator.IsJailed() || !oracleParams.SlashingEnabled {
		return // Cannot slash or jail this validator
	}

	consAddr, err := validator.GetConsAddr()
	if err != nil {
		panic(err)
	}

	if _, err := k.StakingKeeper.Slash(
		ctx,
		consAddr,
		distributionHeight,
		validator.GetConsensusPower(powerReduction),
		slashFraction,
	); err != nil {
		panic(err)
	}

	if err := k.StakingKeeper.Jail(ctx, consAddr); err != nil {
		panic(err)
	}
}

// PossibleWinsPerSlashWindow returns the total number of possible correct votes
// that a validator can have per asset multiplied by the number of vote
// periods in the slash window
func (k Keeper) PossibleWinsPerSlashWindow(ctx sdk.Context) int64 {
	slashWindow := util.SafeUint64ToInt64(k.SlashWindow(ctx))
	votePeriod := util.SafeUint64ToInt64(k.VotePeriod(ctx))

	votePeriodsPerWindow := math.LegacyNewDec(slashWindow).QuoInt64(votePeriod).TruncateInt64()
	numberOfAssets := int64(len(k.GetParams(ctx).MandatoryList))

	return (votePeriodsPerWindow * numberOfAssets)
}

// SetValidatorRewardSet will take all the current validators and store them
// in the ValidatorRewardSet to earn rewards in the current Slash Window.
func (k Keeper) SetValidatorRewardSet(ctx sdk.Context) error {
	validatorRewardSet := types.ValidatorRewardSet{
		ValidatorSet: []string{},
	}
	vals, err := k.StakingKeeper.GetBondedValidatorsByPower(ctx)
	if err != nil {
		return err
	}
	for _, v := range vals {
		addr := v.GetOperator()
		validatorRewardSet.ValidatorSet = append(validatorRewardSet.ValidatorSet, addr)
	}

	k.ValidatorRewardSet.Set(ctx, validatorRewardSet)
	return nil
}

// CurrentValidatorRewardSet returns the latest ValidatorRewardSet in the store.
func (k Keeper) GetValidatorRewardSet(ctx sdk.Context) types.ValidatorRewardSet {
	val, err := k.ValidatorRewardSet.Get(ctx)
	if err != nil {
		return types.ValidatorRewardSet{}
	}

	return val
}
