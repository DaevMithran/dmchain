package keeper

import (
	"errors"
	"fmt"
	"strings"

	"github.com/DaevMithran/dmchain/pricefeeder"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/ojo-network/ojo/util/metrics"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"cosmossdk.io/collections"
	storetypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/DaevMithran/dmchain/x/oracle/types"
)

var ten = math.LegacyMustNewDecFromStr("10")

type Keeper struct {
	cdc        codec.BinaryCodec
	paramSpace paramstypes.Subspace

	// Collections Schema
	Schema collections.Schema

	// Map: denom -> ExchangeRate (math.LegacyDec)
	ExchangeRates collections.Map[string, math.LegacyDec]

	// Map: Validator (sdk.ValAddress) -> Feeder (sdk.AccAddress)
	FeederDelegations collections.Map[sdk.ValAddress, []byte]

	// Map: Voter (sdk.ValAddress) -> Prevote
	Prevotes collections.Map[sdk.ValAddress, types.AggregateExchangeRatePrevote]

	// Map: Voter (sdk.ValAddress) -> Vote
	Votes collections.Map[sdk.ValAddress, types.AggregateExchangeRateVote]

	// Map: Pair(Denom, BlockHeight) -> Price (math.LegacyDec)
	HistoricPrices   collections.Map[collections.Pair[string, uint64], math.LegacyDec]
	MedianPrices     collections.Map[collections.Pair[string, uint64], math.LegacyDec]
	MedianDeviations collections.Map[collections.Pair[string, uint64], math.LegacyDec]

	// Map: Denom (string) -> Last Block Height (uint64)
	LastHistoricPriceBlock collections.Map[string, uint64]

	// Map: Pair(Denom, AvgType) -> Value (math.LegacyDec)
	// AvgType example: "sma", "ema", "wma:BALANCED"
	Averages collections.Map[collections.Pair[string, string], math.LegacyDec]

	// Map: Validator Address -> Miss Counter (uint64)
	MissCounters collections.Map[sdk.ValAddress, uint64]

	// Item: ValidatorRewardSet
	ValidatorRewardSet collections.Item[types.ValidatorRewardSet]

	// Map: Block Height (uint64) -> ParamUpdatePlan
	ParamUpdatePlans collections.Map[uint64, types.ParamUpdatePlan]

	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
	distrKeeper   types.DistributionKeeper
	StakingKeeper types.StakingKeeper

	PriceFeeder *pricefeeder.PriceFeeder

	distrName        string
	telemetryEnabled bool
	// the address capable of executing a MsgUpdateParams message. Typically, this
	// should be the x/gov module account.
	authority string
}

// NewKeeper creates a new Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService storetypes.KVStoreService,
	paramspace paramstypes.Subspace,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	distrKeeper types.DistributionKeeper,
	stakingKeeper types.StakingKeeper,
	distrName string,
	telemetryEnabled bool,
	authority string,
) Keeper {

	// ensure oracle module account is set
	if addr := accountKeeper.GetModuleAddress(types.ModuleName); addr == nil {
		panic(fmt.Sprintf("%s module account has not been set", types.ModuleName))
	}

	// set KeyTable if it has not already been set
	if !paramspace.HasKeyTable() {
		paramspace = paramspace.WithKeyTable(types.ParamKeyTable())
	}

	sb := collections.NewSchemaBuilder(storeService)

	if authority == "" {
		authority = authtypes.NewModuleAddress(govtypes.ModuleName).String()
	}

	k := Keeper{
		cdc: cdc,

		authority: authority,
		ExchangeRates: collections.NewMap(
			sb,
			types.KeyPrefixExchangeRate,
			"exchange_rates",
			collections.StringKey,
			sdk.LegacyDecValue, // Built-in codec for math.LegacyDec
		),
		FeederDelegations: collections.NewMap(
			sb,
			types.KeyPrefixFeederDelegation,
			"feeder_delegations",
			sdk.ValAddressKey,
			collections.BytesValue,
		),
		Prevotes: collections.NewMap(
			sb,
			types.KeyPrefixAggregateExchangeRatePrevote,
			"prevotes",
			sdk.ValAddressKey,
			codec.CollValue[types.AggregateExchangeRatePrevote](cdc),
		),
		Votes: collections.NewMap(
			sb,
			types.KeyPrefixAggregateExchangeRateVote,
			"votes",
			sdk.ValAddressKey,
			codec.CollValue[types.AggregateExchangeRateVote](cdc),
		),
		HistoricPrices: collections.NewMap(
			sb,
			types.KeyPrefixHistoricPrice,
			"historic_prices",
			collections.PairKeyCodec(collections.StringKey, collections.Uint64Key),
			sdk.LegacyDecValue,
		),
		MedianPrices: collections.NewMap(
			sb,
			types.KeyPrefixMedian,
			"median_prices",
			collections.PairKeyCodec(collections.StringKey, collections.Uint64Key),
			sdk.LegacyDecValue,
		),
		MedianDeviations: collections.NewMap(
			sb,
			types.KeyPrefixMedianDeviation,
			"median_deviation_prices",
			collections.PairKeyCodec(collections.StringKey, collections.Uint64Key),
			sdk.LegacyDecValue,
		),
		LastHistoricPriceBlock: collections.NewMap(
			sb,
			types.KeyPrefixLastHistoricPriceBlock,
			"last_historic_block",
			collections.StringKey,
			collections.Uint64Value,
		),
		Averages: collections.NewMap(
			sb,
			types.KeyPrefixAverages,
			"averages",
			collections.PairKeyCodec(collections.StringKey, collections.StringKey),
			sdk.LegacyDecValue,
		),
		MissCounters: collections.NewMap(
			sb,
			types.KeyPrefixMissCounter,
			"miss_counters",
			sdk.ValAddressKey,
			collections.Uint64Value,
		),
		ValidatorRewardSet: collections.NewItem(
			sb,
			types.KeyPrefixValidatorRewardSet,
			"validator_reward_set",
			codec.CollValue[types.ValidatorRewardSet](cdc),
		),
		ParamUpdatePlans: collections.NewMap(
			sb,
			types.KeyPrefixParamUpdatePlan,
			"param_update_plans",
			collections.Uint64Key, // Replaces manual byte formatting for uint64
			codec.CollValue[types.ParamUpdatePlan](cdc),
		),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}

	k.Schema = schema

	return k
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// Exchange Rate

// GetExchangeRateBase gets the consensus exchange rate of an asset
// in the base denom (e.g. ATOM -> uatom)
func (k Keeper) GetExchangeRate(ctx sdk.Context, symbol string) (math.LegacyDec, error) {
	rate, err := k.ExchangeRates.Get(ctx, strings.ToUpper(symbol))
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return math.LegacyZeroDec(), types.ErrUnknownDenom.Wrap(symbol)
		}
		return math.LegacyZeroDec(), err
	}
	return rate, nil
}

// GetExchangeRateBase gets the consensus exchange rate of an asset
// in the base denom (e.g. ATOM -> uatom)
func (k Keeper) GetExchangeRateBase(ctx sdk.Context, denom string) (math.LegacyDec, error) {
	var symbol string
	var exponent uint64
	// Translate the base denom -> symbol
	params := k.GetParams(ctx)
	for _, listDenom := range params.AcceptList {
		if listDenom.BaseDenom == denom {
			symbol = listDenom.SymbolDenom
			exponent = uint64(listDenom.Exponent)
			break
		}
	}
	if len(symbol) == 0 {
		return math.LegacyZeroDec(), types.ErrUnknownDenom.Wrap(denom)
	}

	exchangeRate, err := k.GetExchangeRate(ctx, symbol)
	if err != nil {
		return math.LegacyZeroDec(), err
	}

	powerReduction := ten.Power(exponent)
	return exchangeRate.Quo(powerReduction), nil
}

// SetExchangeRate sets the consensus exchange rate of USD denominated in the
// denom asset to the store.
func (k Keeper) SetExchangeRate(ctx sdk.Context, denom string, exchangeRate math.LegacyDec) {
	_ = k.ExchangeRates.Set(ctx, strings.ToUpper(denom), exchangeRate)
	go metrics.RecordExchangeRate(denom, exchangeRate)
}

// SetExchangeRateWithEvent sets an consensus
// exchange rate to the store with ABCI event
func (k Keeper) SetExchangeRateWithEvent(ctx sdk.Context, denom string, exchangeRate math.LegacyDec) error {
	k.SetExchangeRate(ctx, denom, exchangeRate)
	return ctx.EventManager().EmitTypedEvent(&types.EventSetFxRate{
		Denom: denom, Rate: exchangeRate,
	})
}

// IterateExchangeRates iterates over USD rates in the store.
func (k Keeper) IterateExchangeRates(ctx sdk.Context, handler func(string, math.LegacyDec) bool) {
	err := k.ExchangeRates.Walk(ctx, nil, func(key string, value math.LegacyDec) (bool, error) {
		return handler(key, value), nil
	})
	if err != nil {
		k.Logger(ctx).Error("failed to iterate exchange rates", "err", err)
	}
}

func (k Keeper) ClearExchangeRates(ctx sdk.Context) {
	_ = k.ExchangeRates.Clear(ctx, nil) // Efficiently deletes all entries in prefix
}

// Feeder Delegation

// GetFeederDelegation gets the account address to which the validator operator
// delegated oracle vote rights.
func (k Keeper) GetFeederDelegation(ctx sdk.Context, vAddr sdk.ValAddress) (sdk.AccAddress, error) {
	val, err := k.StakingKeeper.Validator(ctx, vAddr)
	if err != nil {
		return nil, err
	}
	// check that the given validator exists
	if val == nil || !val.IsBonded() {
		return nil, stakingtypes.ErrNoValidatorFound.Wrapf("validator %s is not in active set", vAddr)
	}

	feeder, err := k.FeederDelegations.Get(ctx, vAddr)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return sdk.AccAddress(vAddr), nil // Default to validator address
		}
		return nil, err
	}
	return feeder, nil
}

// SetFeederDelegation sets the account address to which the validator operator
// delegated oracle vote rights.
func (k Keeper) SetFeederDelegation(ctx sdk.Context, operator sdk.ValAddress, delegatedFeeder sdk.AccAddress) {
	_ = k.FeederDelegations.Set(ctx, operator, delegatedFeeder)
}

type IterateFeederDelegationHandler func(delegator sdk.ValAddress, delegate sdk.AccAddress) (stop bool)

// IterateFeederDelegations iterates over the feed delegates and performs a
// callback function.
func (k Keeper) IterateFeederDelegations(ctx sdk.Context, handler IterateFeederDelegationHandler) {
	err := k.FeederDelegations.Walk(ctx, nil, func(key sdk.ValAddress, value []byte) (stop bool, err error) {
		accAddr := sdk.AccAddress(value)
		return handler(key, accAddr), nil
	})
	if err != nil {
		k.Logger(ctx).Error("failed to walk feeder delegations", "error", err)
	}
}

// Prevote

// GetAggregateExchangeRatePrevote retrieves an oracle prevote from the store.
func (k Keeper) GetAggregateExchangeRatePrevote(ctx sdk.Context, voter sdk.ValAddress) (types.AggregateExchangeRatePrevote, error) {
	prevote, err := k.Prevotes.Get(ctx, voter)
	if err != nil {
		return types.AggregateExchangeRatePrevote{}, types.ErrNoAggregatePrevote.Wrap(voter.String())
	}
	return prevote, nil
}

// HasAggregateExchangeRatePrevote checks if a validator has an existing prevote.
func (k Keeper) HasAggregateExchangeRatePrevote(ctx sdk.Context, voter sdk.ValAddress) bool {
	has, _ := k.Prevotes.Has(ctx, voter)
	return has
}

// SetAggregateExchangeRatePrevote set an oracle aggregate prevote to the store.
func (k Keeper) SetAggregateExchangeRatePrevote(ctx sdk.Context, voter sdk.ValAddress, prevote types.AggregateExchangeRatePrevote) {
	_ = k.Prevotes.Set(ctx, voter, prevote)
}

// DeleteAggregateExchangeRatePrevote deletes an oracle prevote from the store.
func (k Keeper) DeleteAggregateExchangeRatePrevote(ctx sdk.Context, voter sdk.ValAddress) {
	_ = k.Prevotes.Remove(ctx, voter)
}

// IterateAggregateExchangeRatePrevotes iterates rate over prevotes in the store
func (k Keeper) IterateAggregateExchangeRatePrevotes(ctx sdk.Context, handler func(sdk.ValAddress, types.AggregateExchangeRatePrevote) bool) {
	err := k.Prevotes.Walk(ctx, nil, func(key sdk.ValAddress, value types.AggregateExchangeRatePrevote) (stop bool, err error) {
		return handler(key, value), nil
	})
	if err != nil {
		k.Logger(ctx).Error("failed to walk prevotes", "error", err)
	}
}

// Vote

// GetAggregateExchangeRateVote retrieves an oracle prevote from the store.
func (k Keeper) GetAggregateExchangeRateVote(ctx sdk.Context, voter sdk.ValAddress) (types.AggregateExchangeRateVote, error) {
	vote, err := k.Votes.Get(ctx, voter)
	if err != nil {
		return types.AggregateExchangeRateVote{}, types.ErrNoAggregateVote.Wrap(voter.String())
	}
	return vote, nil
}

// SetAggregateExchangeRateVote adds an oracle aggregate prevote to the store.
func (k Keeper) SetAggregateExchangeRateVote(ctx sdk.Context, voter sdk.ValAddress, vote types.AggregateExchangeRateVote) {
	_ = k.Votes.Set(ctx, voter, vote)
}

// DeleteAggregateExchangeRateVote deletes an oracle prevote from the store.
func (k Keeper) DeleteAggregateExchangeRateVote(ctx sdk.Context, voter sdk.ValAddress) {
	_ = k.Votes.Remove(ctx, voter)
}

type IterateExchangeRateVote = func(voterAddr sdk.ValAddress, aggregateVote types.AggregateExchangeRateVote) (stop bool)

// IterateAggregateExchangeRateVotes iterates rate over prevotes in the store.
func (k Keeper) IterateAggregateExchangeRateVotes(ctx sdk.Context, handler IterateExchangeRateVote) {
	err := k.Votes.Walk(ctx, nil, func(key sdk.ValAddress, value types.AggregateExchangeRateVote) (stop bool, err error) {
		return handler(key, value), nil
	})
	if err != nil {
		k.Logger(ctx).Error("failed to walk votes", "error", err)
	}
}

// Validation

// ValidateFeeder returns error if the given feeder is not allowed to feed the message.
func (k Keeper) ValidateFeeder(ctx sdk.Context, feederAddr sdk.AccAddress, valAddr sdk.ValAddress) error {
	delegate, err := k.GetFeederDelegation(ctx, valAddr)
	if err != nil {
		return err
	}
	if !delegate.Equals(feederAddr) {
		return types.ErrNoVotingPermission.Wrap(feederAddr.String())
	}
	return nil
}
