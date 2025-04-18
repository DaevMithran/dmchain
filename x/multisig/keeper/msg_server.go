package keeper

import (
	"context"
	"encoding/binary"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"golang.org/x/crypto/blake2b"

	"cosmossdk.io/errors"
	"github.com/DaevMithran/cosmos-modules/x/multisig/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

type msgServer struct {
	k Keeper
}

var _ types.MsgServer = msgServer{}

// NewMsgServerImpl returns an implementation of the module MsgServer interface.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{k: keeper}
}

func (ms msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.k.authority != msg.Authority {
		return nil, errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.k.authority, msg.Authority)
	}

	return nil, ms.k.Params.Set(ctx, msg.Params)
}

// CreateMultisigAccount implements types.MsgServer.
func (ms msgServer) CreateMultisigAccount(ctx context.Context, msg *types.MsgCreateMultisigAccountParams) (*types.MsgCreateMultisigAccountResponse, error) {
	// ctx := sdk.UnwrapSDKContext(goCtx)
	sender, err := ms.k.ac.StringToBytes(msg.Authority)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid sender address (%s)", msg.Authority)
	}

	// validate threshold
	if msg.Threshold < 2 {
		return nil, errors.Wrapf(sdkerrors.ErrInsufficientFee, "invalid threshold")
	}

	// derive multi account id
	multisig_address := DeriveMultisigAccountID(msg.Seed)

	// check for existing account
	_, err = ms.k.MultisigAccounts.Get(ctx, multisig_address)
	if err == nil {
		return nil, errors.Wrapf(sdkerrors.ErrUnknownAddress, "Duplicate seed: Account already exists")
	}

	// insert multisig acount
	ms.k.MultisigAccounts.Set(ctx, multisig_address, types.MultisigAccountDetails{
		Threshold: msg.Threshold,
		Signers: append(msg.Signers, sender),
		Permission: msg.Permission,
	})

	return &types.MsgCreateMultisigAccountResponse{
        MultisigAddress: string(multisig_address),
    }, nil
}

// AddMultisigSigner implements types.MsgServer.
func (ms msgServer) AddMultisigSigner(ctx context.Context, msg *types.MsgAddMultisigSignerParams) (*types.MsgAddMultisigSignerResponse, error) {
	// ctx := sdk.UnwrapSDKContext(goCtx)
    multisig_address, err := ms.k.ac.StringToBytes(msg.MultisigAccount)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid multisig address (%s)", msg.Authority)
	}

    // validate multisig_address
    multisig_account_details, err := ms.k.MultisigAccounts.Get(ctx, multisig_address)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrUnknownAddress, "Duplicate seed: Account already exists")
	}


	return &types.MsgAddMultisigSignerResponse{}, nil
}

// CleanupMultisigSigner implements types.MsgServer.
func (ms msgServer) CleanupMultisigSigner(ctx context.Context, msg *types.MsgAddMultisigSignerParams) (*types.MsgAddMultisigSignerResponse, error) {
	// ctx := sdk.UnwrapSDKContext(goCtx)
	panic("CleanupMultisigSigner is unimplemented")
	return &types.MsgAddMultisigSignerResponse{}, nil
}

// SetThreshold implements types.MsgServer.
func (ms msgServer) SetThreshold(ctx context.Context, msg *types.MsgSetMultisigThresholdParams) (*types.MsgSetMultisigThresholdResponse, error) {
	// ctx := sdk.UnwrapSDKContext(goCtx)
	panic("SetThreshold is unimplemented")
	return &types.MsgSetMultisigThresholdResponse{}, nil
}

// InitializeMultisigProposal implements types.MsgServer.
func (ms msgServer) InitializeMultisigProposal(ctx context.Context, msg *types.MsgInitializeMultisigProposalParams) (*types.MsgInitializeMultisigResponse, error) {
	// ctx := sdk.UnwrapSDKContext(goCtx)
	panic("InitializeMultisigProposal is unimplemented")
	return &types.MsgInitializeMultisigResponse{}, nil
}

// ApproveMultisigProposal implements types.MsgServer.
func (ms msgServer) ApproveMultisigProposal(ctx context.Context, msg *types.MsgApproveMultisigProposalParams) (*types.MsgApproveMultisigProposalResponse, error) {
	// ctx := sdk.UnwrapSDKContext(goCtx)
	panic("ApproveMultisigProposal is unimplemented")
	return &types.MsgApproveMultisigProposalResponse{}, nil
}

// CancelMultisigProposal implements types.MsgServer.
func (ms msgServer) CancelMultisigProposal(ctx context.Context, msg *types.MsgCancelMultisigProposalParams) (*types.MsgCancelMultisigProposalResponse, error) {
	// ctx := sdk.UnwrapSDKContext(goCtx)
	panic("CancelMultisigProposal is unimplemented")
	return &types.MsgCancelMultisigProposalResponse{}, nil
}

// CleanupMultisigProposal implements types.MsgServer.
func (ms msgServer) CleanupMultisigProposal(ctx context.Context, msg *types.MsgCleanupMultisigProposalParams) (*types.MsgCleanupMultisigProposalResponse, error) {
	// ctx := sdk.UnwrapSDKContext(goCtx)
	panic("CleanupMultisigProposal is unimplemented")
	return &types.MsgCleanupMultisigProposalResponse{}, nil
}

func DeriveMultisigAccountID(seed uint32) sdk.AccAddress {
	seedBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(seedBytes, seed)

	// prefix the seed with account addr prefix
	input := append([]byte(sdk.GetConfig().GetBech32AccountAddrPrefix()), seedBytes...)

	// Hash using blake2b 256 bit
	hash := blake2b.Sum256(input)

	// Take the first 20 bytes to form an AccAddress
	addr:= hash[:20]

	return sdk.AccAddress(addr)

}
