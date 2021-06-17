package itests

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/build"
	sealing "github.com/filecoin-project/lotus/extern/storage-sealing"
	"github.com/filecoin-project/lotus/itests/kit2"
)

func TestPledgeSectors(t *testing.T) {
	kit2.QuietMiningLogs()

	blockTime := 50 * time.Millisecond

	runTest := func(t *testing.T, nSectors int) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, miner, ens := kit2.EnsembleMinimal(t, kit2.MockProofs())
		ens.InterconnectAll().BeginMining(blockTime)

		miner.PledgeSectors(ctx, nSectors, 0, nil)
	}

	t.Run("1", func(t *testing.T) {
		runTest(t, 1)
	})

	t.Run("100", func(t *testing.T) {
		runTest(t, 100)
	})

	t.Run("1000", func(t *testing.T) {
		if testing.Short() { // takes ~16s
			t.Skip("skipping test in short mode")
		}

		runTest(t, 1000)
	})
}

func TestPledgeBatching(t *testing.T) {
	blockTime := 50 * time.Millisecond

	runTest := func(t *testing.T, nSectors int) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		opts := kit2.ConstructorOpts(kit2.LatestActorsAt(-1))
		client, miner, ens := kit2.EnsembleMinimal(t, kit2.MockProofs(), opts)
		ens.InterconnectAll().BeginMining(blockTime)

		waitTillChainHeight(ctx, t, client, 10)

		toCheck := miner.StartPledge(ctx, nSectors, 0, nil)

		for len(toCheck) > 0 {
			states := map[api.SectorState]int{}

			for n := range toCheck {
				st, err := miner.SectorsStatus(ctx, n, false)
				require.NoError(t, err)
				states[st.State]++
				if st.State == api.SectorState(sealing.Proving) {
					delete(toCheck, n)
				}
				if strings.Contains(string(st.State), "Fail") {
					t.Fatal("sector in a failed state", st.State)
				}
			}
			if states[api.SectorState(sealing.SubmitPreCommitBatch)] == nSectors ||
				(states[api.SectorState(sealing.SubmitPreCommitBatch)] > 0 && states[api.SectorState(sealing.PreCommit1)] == 0 && states[api.SectorState(sealing.PreCommit2)] == 0) {
				pcb, err := miner.SectorPreCommitFlush(ctx)
				require.NoError(t, err)
				if pcb != nil {
					fmt.Printf("PRECOMMIT BATCH: %+v\n", pcb)
				}
			}

			if states[api.SectorState(sealing.SubmitCommitAggregate)] == nSectors ||
				(states[api.SectorState(sealing.SubmitCommitAggregate)] > 0 && states[api.SectorState(sealing.WaitSeed)] == 0 && states[api.SectorState(sealing.Committing)] == 0) {
				cb, err := miner.SectorCommitFlush(ctx)
				require.NoError(t, err)
				if cb != nil {
					fmt.Printf("COMMIT BATCH: %+v\n", cb)
				}
			}

			build.Clock.Sleep(100 * time.Millisecond)
			fmt.Printf("WaitSeal: %d %+v\n", len(toCheck), states)
		}
	}

	t.Run("100", func(t *testing.T) {
		runTest(t, 100)
	})
}

func TestPledgeBeforeNv13(t *testing.T) {
	blocktime := 50 * time.Millisecond

	runTest := func(t *testing.T, nSectors int) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		opts := kit2.ConstructorOpts(kit2.LatestActorsAt(1000000000))
		client, miner, ens := kit2.EnsembleMinimal(t, kit2.MockProofs(), opts)
		ens.InterconnectAll().BeginMining(blocktime)

		waitTillChainHeight(ctx, t, client, 10)

		toCheck := miner.StartPledge(ctx, nSectors, 0, nil)

		for len(toCheck) > 0 {
			states := map[api.SectorState]int{}

			for n := range toCheck {
				st, err := miner.SectorsStatus(ctx, n, false)
				require.NoError(t, err)
				states[st.State]++
				if st.State == api.SectorState(sealing.Proving) {
					delete(toCheck, n)
				}
				if strings.Contains(string(st.State), "Fail") {
					t.Fatal("sector in a failed state", st.State)
				}
			}

			build.Clock.Sleep(100 * time.Millisecond)
			fmt.Printf("WaitSeal: %d %+v\n", len(toCheck), states)
		}
	}

	t.Run("100-before-nv13", func(t *testing.T) {
		runTest(t, 100)
	})
}

func waitTillChainHeight(ctx context.Context, t *testing.T, node *kit2.TestFullNode, height int) {
	for {
		h, err := node.ChainHead(ctx)
		require.NoError(t, err)
		if h.Height() > abi.ChainEpoch(height) {
			return
		}
	}
}