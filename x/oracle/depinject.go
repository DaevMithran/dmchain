package module

import (
	"github.com/cosmos/cosmos-sdk/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	"cosmossdk.io/core/address"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"

	modulev1 "github.com/DaevMithran/dmchain/api/oracle/module/v1"
	"github.com/DaevMithran/dmchain/x/oracle/keeper"
	"github.com/DaevMithran/dmchain/x/oracle/types"
)

var _ appmodule.AppModule = AppModule{}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (am AppModule) IsOnePerModuleType() {}

// IsAppModule implements the appmodule.AppModule interface.
func (am AppModule) IsAppModule() {}

func init() {
	appmodule.Register(
		&modulev1.Module{},
		appmodule.Provide(ProvideModule),
	)
}

type ModuleInputs struct {
	depinject.In

	Cdc          codec.Codec
	StoreService store.KVStoreService
	AddressCodec address.Codec

	AccountKeeper  authkeeper.AccountKeeper
	BankKeeper     bankkeeper.Keeper
	StakingKeeper  stakingkeeper.Keeper
	SlashingKeeper slashingkeeper.Keeper
	DistrKeeper    types.DistributionKeeper

	ParamSpace paramstypes.Subspace
}

type ModuleOutputs struct {
	depinject.Out

	Module appmodule.AppModule
	Keeper keeper.Keeper
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	govAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	k := keeper.NewKeeper(
		in.Cdc,
		in.StoreService,
		in.ParamSpace,
		in.AccountKeeper,
		in.BankKeeper,
		in.DistrKeeper,
		in.StakingKeeper,
		types.ModuleName,
		true,
		govAddr,
	)
	m := NewAppModule(in.Cdc, k, in.AccountKeeper, in.BankKeeper)

	return ModuleOutputs{Module: m, Keeper: k, Out: depinject.Out{}}
}
