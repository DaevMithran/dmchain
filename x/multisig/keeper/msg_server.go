package keeper

import (
	"bytes"
	"context"
	"encoding/binary"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"golang.org/x/crypto/blake2b"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	multisigv1 "github.com/DaevMithran/cosmos-modules/api/multisig/v1"
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

	sender, err := ms.k.ac.StringToBytes(msg.Authority)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid sender address (%s)", msg.Authority)
	}

	// validate threshold
	if msg.Threshold < 1 {
		return nil, errors.Wrapf(sdkerrors.ErrInsufficientFee, "invalid threshold")
	}

    // validate signers
    if len(msg.Signers) < 2 && len(msg.Signers) > 10 {
        return nil, errors.Wrap(sdkerrors.ErrInsufficientFunds, "Atleast two signers are required")
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

    multisig_address, err := ms.k.ac.StringToBytes(msg.MultisigAddress)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid multisig address (%s)", msg.MultisigAddress)
	}

    new_signer, err := ms.k.ac.StringToBytes(msg.Signer)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid signer address (%s)", msg.Signer)
	}

    // validate multisig_address
    multisig_account_details, err := ms.k.MultisigAccounts.Get(ctx, multisig_address)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrConflict, "Invalid multisig: Account not found")
	}

    // validate signer: Is it better to have an index table just for this; Makes deletion and updates expensive!
    if contains(multisig_account_details.Signers, new_signer) == true {
        return nil, errors.Wrap(sdkerrors.ErrConflict, "Duplicate signer")
    }

    // check if new threshold is provided
    if msg.XNewThreshold != nil {
        if msg.GetNewThreshold() < 1 {
            return nil, errors.Wrapf(sdkerrors.ErrInsufficientFee, "invalid threshold")
        }

        multisig_account_details.Threshold = msg.GetNewThreshold()
    }

    // Update the signer list
    multisig_account_details.Signers = append(multisig_account_details.Signers, new_signer);

    // Update the multisig account
    ms.k.MultisigAccounts.Set(ctx, multisig_address, multisig_account_details)

	return &types.MsgAddMultisigSignerResponse{}, nil
}

// CleanupMultisigSigner implements types.MsgServer.
func (ms msgServer) CleanupMultisigSigner(ctx context.Context, msg *types.MsgCleanupMultisigAccountParams) (*types.MsgCleanupMultisigAccountResponse, error) {
	panic("CleanupMultisigSigner is unimplemented")
	return &types.MsgCleanupMultisigAccountResponse{}, nil
}

// SetThreshold implements types.MsgServer.
func (ms msgServer) SetThreshold(ctx context.Context, msg *types.MsgSetMultisigThresholdParams) (*types.MsgSetMultisigThresholdResponse, error) {
	panic("SetThreshold is unimplemented")
	return &types.MsgSetMultisigThresholdResponse{}, nil
}

// InitializeMultisigProposal implements types.MsgServer.
func (ms msgServer) InitializeMultisigProposal(ctx context.Context, msg *types.MsgInitializeMultisigProposalParams) (*types.MsgInitializeMultisigResponse, error) {
    // validate proposer address
    proposer, err := ms.k.ac.StringToBytes(msg.Proposer)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid signer address (%s)", msg.Proposer)
	}
    
    multisig_address, err := ms.k.ac.StringToBytes(msg.MultisigAddress)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid multisig address (%s)", msg.MultisigAddress)
	}

    // validate account
    multisig_account_details, err := ms.k.MultisigAccounts.Get(ctx, multisig_address)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrConflict, "Invalid multisig: Account not found")
	}

    // validate proposer
    if contains(multisig_account_details.Signers, proposer) == false {
        return nil, errors.Wrap(sdkerrors.ErrConflict, "Invalid proposer: Permission Denied")
    }

    // Compute call hash
    call_hash := blake2b.Sum256(msg.Message.Value)

    // validate call
    existing_proposal, err := ms.k.OrmDB.ProposalTable().GetByMultisigAddressCallHash(ctx, multisig_address, call_hash[:])
    if err != nil {
        return &types.MsgInitializeMultisigResponse{
            ProposalId: existing_proposal.Id,
        }, nil
    }

    // approvals
    approvals := [][]byte{proposer};

    id, err := ms.k.OrmDB.ProposalTable().InsertReturningId(
        ctx,
        &multisigv1.Proposal{
            Depositor: proposer,
            Deposit: 10,
            MultisigAddress: multisig_address,
            Approvals: approvals,
            CallHash: call_hash[:],
        },
    )
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrConflict, "Invalid multisig: Proposal addition failed")
	}

    // collect deposit
    deposit := sdk.NewCoins(sdk.NewCoin("uom", math.NewInt(int64(10))))
    err = ms.k.BankKeeper.SendCoinsFromAccountToModule(ctx, proposer, types.ModuleName, deposit)
    if err != nil {
        return nil, err
    }

	return &types.MsgInitializeMultisigResponse{
        ProposalId: id,
    }, nil
}

// ApproveMultisigProposal implements types.MsgServer.
func (ms msgServer) ApproveMultisigProposal(ctx context.Context, msg *types.MsgApproveMultisigProposalParams) (*types.MsgApproveMultisigProposalResponse, error) {

    approver, err := ms.k.ac.StringToBytes(msg.Approver)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid signer address (%s)", msg.Approver)
	}
    
    multisig_address, err := ms.k.ac.StringToBytes(msg.MultisigAddress)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid multisig address (%s)", msg.MultisigAddress)
	}

    // validate account
    multisig_account_details, err := ms.k.MultisigAccounts.Get(ctx, multisig_address)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrConflict, "Invalid multisig: Account not found")
	}

    // validate approver
    if contains(multisig_account_details.Signers, approver) == false {
        return nil, errors.Wrap(sdkerrors.ErrConflict, "Invalid proposer: Permission Denied")
    }

    // validate proposal
    proposal, err := ms.k.OrmDB.ProposalTable().Get(ctx, msg.GetProposalId())
    if contains(proposal.Approvals, approver) {
       return &types.MsgApproveMultisigProposalResponse{}, nil
    }

    // Add approval if needed
    if len(proposal.Approvals) < int(multisig_account_details.Threshold) {
        // approve proposal
        proposal.Approvals = append(proposal.Approvals, approver)

        // update proposal
        ms.k.OrmDB.ProposalTable().Update(ctx, proposal)
    }

	return &types.MsgApproveMultisigProposalResponse{}, nil
}

// ApproveAndDispatchMultisigProposal implements types.MsgServer.
func (ms msgServer) ApproveAndDispatchMultisigProposal(ctx context.Context, msg *types.MsgApproveAndDispatchMultisigProposalParams) (*types.MsgApproveAndDispatchMultisigProposalResponse, error) {
    // vaidate signer
    approver, err := ms.k.ac.StringToBytes(msg.Approver)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid signer address (%s)", msg.Approver)
	}
    
    multisig_address, err := ms.k.ac.StringToBytes(msg.MultisigAddress)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid multisig address (%s)", msg.MultisigAddress)
	}

    // validate account
    multisig_account_details, err := ms.k.MultisigAccounts.Get(ctx, multisig_address)
	if err != nil {
		return nil, errors.Wrapf(sdkerrors.ErrConflict, "Invalid multisig: Account not found")
	}

    // validate approver
    if contains(multisig_account_details.Signers, approver) == false {
        return nil, errors.Wrap(sdkerrors.ErrConflict, "Invalid proposer: Permission Denied")
    }

    // Compute call hash
    call_hash := blake2b.Sum256(msg.Message.Value)

    // validate proposal
    proposal, err := ms.k.OrmDB.ProposalTable().GetByMultisigAddressCallHash(ctx, multisig_address, call_hash[:])
    if err != nil {
        return nil, err
    }

    if msg.ProposalId != proposal.Id {
        return nil, errors.Wrap(sdkerrors.ErrConflict, "Proposal Id does not match with call_hash")
    }

    // if dispatcher has approved already
    approvals_len := len(proposal.Approvals)
    if !contains(proposal.Approvals, approver) {
        approvals_len += 1;
    }

    // check threshold
    if approvals_len < int(multisig_account_details.Threshold) {
        return nil, errors.Wrap(sdkerrors.ErrInsufficientFee, "Cannot dispatch proposal, threshold not met")
    }

    // dispatch call
    res, err := ms.k.DispatchActions(ctx, multisig_address, msg)

    // remove proposal
    ms.k.OrmDB.ProposalTable().Delete(ctx, proposal)

	return &types.MsgApproveAndDispatchMultisigProposalResponse{
        TransactionHash: string(res.Data),
    }, nil
}

func (k Keeper) DispatchActions(ctx context.Context, multisig_address sdk.AccAddress, msg sdk.Msg) (*sdk.Result, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

    handler := k.router.Handler(msg)
    if handler == nil {
        return nil, sdkerrors.ErrUnknownRequest.Wrapf("unrecognized message route: %s", sdk.MsgTypeURL(msg))
    }

    msgResp, err := handler(sdkCtx, msg)
    if err != nil {
        return nil, errors.Wrapf(err, "failed to execute message; message %v", msg)
    }

	return msgResp, nil
}

// CancelMultisigProposal implements types.MsgServer.
func (ms msgServer) CancelMultisigProposal(ctx context.Context, msg *types.MsgCancelMultisigProposalParams) (*types.MsgCancelMultisigProposalResponse, error) {

	panic("CancelMultisigProposal is unimplemented")
	return &types.MsgCancelMultisigProposalResponse{}, nil
}

// CleanupMultisigProposal implements types.MsgServer.
func (ms msgServer) CleanupMultisigProposal(ctx context.Context, msg *types.MsgCleanupMultisigProposalParams) (*types.MsgCleanupMultisigProposalResponse, error) {

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

func contains(signers [][]byte, signer []byte) bool {
    for _, signer := range signers {
        if bytes.Equal(signer, signer) {
            return true
        }
    }
    return false
}
