# Oracle Module

## Abstract

This module is a **fork of `ojo-network/x/oracle`**, refactored to utilize the **Cosmos SDK Collections** framework for state management. This ensures type-safety, improved performance through native binary codecs, and a more maintainable schema definition.

## Contents

1. **[Concepts](https://www.google.com/search?q=%23concepts)**
2. **[State & Collections](https://www.google.com/search?q=%23state--collections)**
3. **[End Block](https://www.google.com/search?q=%23end-block)**
4. **[Messages](https://www.google.com/search?q=%23messages)**
5. **[Events](https://www.google.com/search?q=%23events)**
6. **[Parameters](https://www.google.com/search?q=%23params)**

## Concepts

### Voting Procedure

The Oracle module obtains consensus via a **Commit-Reveal scheme** over a `VotePeriod`.

* **Prevote and Vote**:
* `MsgAggregateExchangeRatePrevote`: A SHA256 hash of the rates.
* `MsgAggregateExchangeRateVote`: The salt and actual rates to reveal the previous period's commitment.


* **Vote Tally**: At the end of `VotePeriod`, the module verifies hashes and calculates the **Median** exchange rate. Rates receiving less than `VoteThreshold` power are deleted.
* **Ballot Rewards**: Winners (those within the `RewardBand`) are rewarded from the reward pool. In this fork, the `ValidatorRewardSet` is cached to optimize reward distribution across the `SlashWindow`.

### Slashing

Validators must maintain a `MinValidPerWindow` (e.g., 5%) success rate. Failure to vote on **all** assets in the `AcceptList` or voting outside the `RewardBand` results in a "miss." If the threshold is not met by the end of a `SlashWindow`, the validator is slashed and jailed.

## State & Collections

This module utilizes `cosmossdk.io/collections` for all on-chain storage. This removes manual byte-prefixing and Protobuf wrapping (e.g., `gogotypes.UInt64Value`) in favor of type-safe Maps and Items.

### Exchange Rates

Stored as a `math.LegacyDec`.

* `ExchangeRates`: `Map<string, math.LegacyDec>`
* `HistoricPrices`: `Map<Pair<string, uint64>, math.LegacyDec>` (Denom + BlockHeight)

### Validator Management

* **FeederDelegation**: Maps a validator operator to a proxy "feeder" account.
* `FeederDelegations`: `Map<sdk.ValAddress, sdk.AccAddress>`


* **MissCounter**: Tracks missed vote periods.
* `MissCounters`: `Map<sdk.ValAddress, uint64>`


* **ValidatorRewardSet**: A singleton storing the active validators eligible for rewards.
* `ValidatorRewardSet`: `Item<types.ValidatorRewardSet>`



### Voting State

* **AggregateExchangeRatePrevote**: `Map<sdk.ValAddress, types.AggregateExchangeRatePrevote>`
* **AggregateExchangeRateVote**: `Map<sdk.ValAddress, types.AggregateExchangeRateVote>`

### Price Averages (dmchain specific)

The `dmchain` fork includes native support for computed averages:

* `Averages`: `Map<Pair<string, string>, math.LegacyDec>` (Denom + AvgType, e.g., "SMA", "EMA")

## End Block

At the end of every `VotePeriod`:

1. **Purge**: Expired exchange rates are cleared.
2. **Organize**: Votes are grouped into ballots by denomination.
3. **Tally**:
* Calculate the **Median** and **Standard Deviation**.
* Define the winners within the `RewardBand`.


4. **Record**: Update `ExchangeRates` and compute `Averages` (SMA/EMA/WMA).
5. **Slash/Reward**: Increment `MissCounters`, distribute rewards to `ValidatorRewardSet`, and jail underperforming validators at the end of the `SlashWindow`.
6. **Cleanup**: Clear previous period votes and prevotes.

## Messages

The module supports the standard Ojo Oracle message set, including:

* `MsgAggregateExchangeRatePrevote`
* `MsgAggregateExchangeRateVote`
* `MsgDelegateFeedConsent`

## Params

Parameters are managed via the standard `Params` struct, typically updated via governance. Keys include `VotePeriod`, `VoteThreshold`, `RewardBand`, `SlashWindow`, and the `AcceptList` (denominations to provide prices for).
