package module

import (
	"context"
	"encoding/json"

	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/client/v2/autocli"
	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	"github.com/DaevMithran/dmchain/x/multisig/client/cli"
	oracleabci "github.com/DaevMithran/dmchain/x/oracle/abci"
	"github.com/DaevMithran/dmchain/x/oracle/keeper"
	simulation "github.com/DaevMithran/dmchain/x/oracle/simulations"
	"github.com/DaevMithran/dmchain/x/oracle/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

const (
	// ConsensusVersion defines the current x/oracle module consensus version.
	ConsensusVersion = 1
)

var (
	_ module.AppModuleBasic   = AppModuleBasic{}
	_ module.AppModuleGenesis = AppModule{}
	_ module.AppModule        = AppModule{}

	_ autocli.HasAutoCLIConfig = AppModule{}
)

// AppModuleBasic defines the basic application module used by the wasm module.
type AppModuleBasic struct {
	cdc codec.Codec
}

type AppModule struct {
	AppModuleBasic

	keeper        keeper.Keeper
	accountKeeper types.AccountKeeper
	bankKeeper    bankkeeper.Keeper
}

// NewAppModule constructor
func NewAppModule(
	cdc codec.Codec,
	keeper keeper.Keeper,
	accountKeeper types.AccountKeeper,
	bankKeeper bankkeeper.Keeper,
) *AppModule {
	return &AppModule{
		AppModuleBasic: AppModuleBasic{cdc: cdc},
		keeper:         keeper,
		accountKeeper:  accountKeeper,
		bankKeeper:     bankKeeper,
	}
}

func (a AppModuleBasic) Name() string {
	return types.ModuleName
}

func (a AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(&types.GenesisState{
		Params: types.DefaultParams(),
	})
}

func (a AppModuleBasic) ValidateGenesis(marshaler codec.JSONCodec, _ client.TxEncodingConfig, message json.RawMessage) error {
	var data types.GenesisState
	err := marshaler.UnmarshalJSON(message, &data)
	if err != nil {
		return err
	}
	if err := data.Params.Validate(); err != nil {
		return errorsmod.Wrap(err, "params")
	}
	return nil
}

func (a AppModuleBasic) RegisterRESTRoutes(_ client.Context, _ *mux.Router) {
}

func (a AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	err := types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx))
	if err != nil {
		// same behavior as in cosmos-sdk
		panic(err)
	}
}

// Disable in favor of autocli.go. If you wish to use these, it will override AutoCLI methods.

func (a AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.NewTxCmd()
}

func (a AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterLegacyAminoCodec(cdc)
}

func (a AppModuleBasic) RegisterInterfaces(r codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(r)
}

func (a AppModule) InitGenesis(ctx sdk.Context, marshaler codec.JSONCodec, message json.RawMessage) []abci.ValidatorUpdate {
	var genesisState types.GenesisState

	marshaler.MustUnmarshalJSON(message, &genesisState)
	InitGenesis(ctx, a.keeper, genesisState)

	return nil
}

func (a AppModule) ExportGenesis(ctx sdk.Context, marshaler codec.JSONCodec) json.RawMessage {
	genState := ExportGenesis(ctx, a.keeper)
	return marshaler.MustMarshalJSON(genState)
}

func (a AppModule) RegisterInvariants(_ sdk.InvariantRegistry) {
}

func (a AppModule) QuerierRoute() string {
	return types.QuerierRoute
}

func (a AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(a.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQuerier(a.keeper))
}

// ConsensusVersion is a sequence number for state-breaking change of the
// module. It should be incremented on each consensus-breaking change
// introduced by the module. To avoid wrong/empty versions, the initial version
// should be set to 1.
func (a AppModule) ConsensusVersion() uint64 {
	return ConsensusVersion
}

// BeginBlock executes all ABCI BeginBlock logic respective to the x/oracle module.
func (a AppModule) BeginBlock(_ context.Context) {}

// EndBlock executes all ABCI EndBlock logic respective to the x/oracle module.
// It returns no validator updates.
func (a AppModule) EndBlock(ctx context.Context) ([]abci.ValidatorUpdate, error) {
	if err := oracleabci.EndBlocker(ctx, a.keeper); err != nil {
		panic(err)
	}

	return []abci.ValidatorUpdate{}, nil
}

// GenerateGenesisState creates a randomized GenState of the distribution module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	simulation.RandomizedGenState(simState)
}

// WeightedOperations returns the all the oracle module operations with their respective weights.
func (a AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	return simulation.WeightedOperations(
		simState.AppParams, a.accountKeeper, a.bankKeeper, a.keeper,
	)
}

// ProposalContents returns all the oracle content functions used to
// simulate governance proposals.
func (am AppModule) ProposalContents(_ module.SimulationState) []simtypes.WeightedProposalMsg {
	return nil
}

// RegisterStoreDecoder registers a decoder for oracle module's types
func (am AppModule) RegisterStoreDecoder(sdr simtypes.StoreDecoderRegistry) {
	sdr[types.StoreKey] = simulation.NewDecodeStore(am.cdc)
}
