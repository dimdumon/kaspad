package consensusstatemanager_test

import (
	"testing"

	"github.com/kaspanet/kaspad/domain/consensus/utils/transactionhelper"

	"github.com/kaspanet/kaspad/domain/consensus"
	"github.com/kaspanet/kaspad/domain/consensus/model/externalapi"
	"github.com/kaspanet/kaspad/domain/consensus/utils/multiset"
	"github.com/kaspanet/kaspad/domain/consensus/utils/testutils"
	"github.com/kaspanet/kaspad/domain/dagconfig"
)

func TestUTXOCommitment(t *testing.T) {
	testutils.ForAllNets(t, true, func(t *testing.T, params *dagconfig.Params) {
		params.BlockCoinbaseMaturity = 0
		factory := consensus.NewFactory()

		consensus, teardown, err := factory.NewTestConsensus(params, "TestUTXOCommitment")
		if err != nil {
			t.Fatalf("Error setting up consensus: %+v", err)
		}
		defer teardown()

		// Build the following DAG:
		// G <- A <- B <- C <- E
		//             <- D <-
		// Where block D has a non-coinbase transaction
		genesisHash := params.GenesisHash

		// Block A:
		blockAHash, err := consensus.AddBlock([]*externalapi.DomainHash{genesisHash}, nil, nil)
		if err != nil {
			t.Fatalf("Error creating block A: %+v", err)
		}
		// Block B:
		blockBHash, err := consensus.AddBlock([]*externalapi.DomainHash{blockAHash}, nil, nil)
		if err != nil {
			t.Fatalf("Error creating block B: %+v", err)
		}
		blockB, err := consensus.GetBlock(blockBHash)
		if err != nil {
			t.Fatalf("Error getting block B: %+v", err)
		}
		// Block C:
		blockCHash, err := consensus.AddBlock([]*externalapi.DomainHash{blockBHash}, nil, nil)
		if err != nil {
			t.Fatalf("Error creating block C: %+v", err)
		}
		// Block D:
		blockDTransaction, err := testutils.CreateTransaction(
			blockB.Transactions[transactionhelper.CoinbaseTransactionIndex])
		if err != nil {
			t.Fatalf("Error creating transaction: %+v", err)
		}
		blockDHash, err := consensus.AddBlock([]*externalapi.DomainHash{blockBHash}, nil,
			[]*externalapi.DomainTransaction{blockDTransaction})
		if err != nil {
			t.Fatalf("Error creating block D: %+v", err)
		}
		// Block E:
		blockEHash, err := consensus.AddBlock([]*externalapi.DomainHash{blockCHash, blockDHash}, nil, nil)
		if err != nil {
			t.Fatalf("Error creating block E: %+v", err)
		}
		blockE, err := consensus.GetBlock(blockEHash)
		if err != nil {
			t.Fatalf("Error getting block E: %+v", err)
		}

		// Get the past UTXO set of block E
		csm := consensus.ConsensusStateManager()
		utxoSetIterator, err := csm.RestorePastUTXOSetIterator(blockEHash)
		if err != nil {
			t.Fatalf("Error restoring past UTXO of block E: %+v", err)
		}

		// Build a Multiset for block E
		ms := multiset.New()
		for utxoSetIterator.Next() {
			outpoint, entry, err := utxoSetIterator.Get()
			if err != nil {
				t.Fatalf("Error getting from UTXOSet iterator: %+v", err)
			}
			err = consensus.ConsensusStateManager().AddUTXOToMultiset(ms, entry, outpoint)
			if err != nil {
				t.Fatalf("Error adding utxo to multiset: %+v", err)
			}
		}

		// Turn the multiset into a UTXO commitment
		utxoCommitment := ms.Hash()

		// Make sure that the two commitments are equal
		if *utxoCommitment != blockE.Header.UTXOCommitment {
			t.Fatalf("TestUTXOCommitment: calculated UTXO commitment and "+
				"actual UTXO commitment don't match. Want: %s, got: %s",
				utxoCommitment, blockE.Header.UTXOCommitment)
		}
	})
}

func TestPastUTXOMultiset(t *testing.T) {
	testutils.ForAllNets(t, true, func(t *testing.T, params *dagconfig.Params) {
		factory := consensus.NewFactory()

		consensus, teardown, err := factory.NewTestConsensus(params, "TestUTXOCommitment")
		if err != nil {
			t.Fatalf("Error setting up consensus: %+v", err)
		}
		defer teardown()

		// Build a short chain
		currentHash := params.GenesisHash
		for i := 0; i < 3; i++ {
			currentHash, err = consensus.AddBlock([]*externalapi.DomainHash{currentHash}, nil, nil)
			if err != nil {
				t.Fatalf("Error creating block A: %+v", err)
			}
		}

		// Save the current tip's hash to be used lated
		testedBlockHash := currentHash

		// Take testedBlock's multiset and hash
		firstMultiset, err := consensus.MultisetStore().Get(consensus.DatabaseContext(), testedBlockHash)
		if err != nil {
			return
		}
		firstMultisetHash := firstMultiset.Hash()

		// Add another block on top of testedBlock
		_, err = consensus.AddBlock([]*externalapi.DomainHash{testedBlockHash}, nil, nil)
		if err != nil {
			t.Fatalf("Error creating block A: %+v", err)
		}

		// Take testedBlock's multiset and hash again
		secondMultiset, err := consensus.MultisetStore().Get(consensus.DatabaseContext(), testedBlockHash)
		if err != nil {
			return
		}
		secondMultisetHash := secondMultiset.Hash()

		// Make sure the multiset hasn't changed
		if *firstMultisetHash != *secondMultisetHash {
			t.Fatalf("TestPastUTXOMultiSet: selectedParentMultiset appears to have changed!")
		}
	})
}