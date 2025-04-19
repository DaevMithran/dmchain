package keeper_test

import (
	"testing"

	apiv1 "github.com/DaevMithran/dmchain/api/multisig/v1"
	"github.com/stretchr/testify/require"
)

func TestORM(t *testing.T) {
	f := SetupTest(t)

	dt := f.k.OrmDB.ProposalTable()
	acc := []byte("test_acc")
	amt := uint64(7)

	err := dt.Insert(f.ctx, &apiv1.Proposal{
		Id: 1,
		Depositor: acc,
		Deposit: 100,
		Approvals: [][]byte{},
		MultisigAddress: []byte("test_multisig_acc"),
		CallHash: []byte("test call"),
	})
	require.NoError(t, err)

	d, err := dt.Has(f.ctx, 1)
	require.NoError(t, err)
	require.True(t, d)

	res, err := dt.Get(f.ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.EqualValues(t, amt, res.Deposit)
}
