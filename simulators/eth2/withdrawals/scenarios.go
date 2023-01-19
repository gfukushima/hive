package main

import (
	"bytes"
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

var ConsensusClientsSupportingBLSChangesBeforeCapella = []string{
	"prysm",
}

// Generic withdrawals test routine, capable of running most of the test
// scenarios.
func (ts BaseWithdrawalsTestSpec) Execute(
	t *hivesim.T,
	env *testnet.Environment,
	n []clients.NodeDefinition,
) {
	config := ts.GetTestnetConfig(n)
	ctx := context.Background()

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	blsDomain := ComputeBLSToExecutionDomain(testnet)

	// Get all validators info
	allValidators, err := ValidatorsFromBeaconState(
		testnet.GenesisBeaconState(),
		env.Keys,
		&blsDomain,
	)
	if err != nil {
		t.Fatalf("FAIL: Error parsing validators from beacon state")
	}
	genesisNonWithdrawable := allValidators.NonWithdrawable()

	// Wait for beacon chain genesis to happen
	testnet.WaitForGenesis(ctx)

	// Wait for 3 slots to pass
	<-time.After(
		3 * time.Second * time.Duration(testnet.Spec().SECONDS_PER_SLOT),
	)

	if beaconClients := testnet.FilterByCL(ConsensusClientsSupportingBLSChangesBeforeCapella).
		BeaconClients(); len(
		beaconClients,
	) > 0 &&
		ts.SubmitBLSChangesOnBellatrix {
		// If there are clients that support sending BLS to execution
		// changes in bellatrix, we send half of the changes here
		if len(genesisNonWithdrawable) > 0 {
			nonWithdrawableValidators := genesisNonWithdrawable.Chunks(2)[0]

			if len(nonWithdrawableValidators) > 0 {
				t.Logf(
					"INFO: Sending %d validators' BLS-to-exec-change on bellatrix",
					len(nonWithdrawableValidators),
				)
				for i := 0; i < len(nonWithdrawableValidators); i++ {
					b := beaconClients[i%len(beaconClients)]
					v := nonWithdrawableValidators[i]
					if err := v.SignSendBLSToExecutionChange(
						ctx,
						b,
						common.Address{byte(v.Index + 0x100)},
					); err != nil {
						t.Fatalf(
							"FAIL: Unable to submit bls-to-execution changes: %v",
							err,
						)
					}
				}

			}

		} else {
			t.Logf("INFO: no validators left on BLS credentials")
		}
	} else {
		t.Logf("INFO: No beacon clients support BLS-To-Execution-Changes on bellatrix, skipping")
	}

	// Wait for Capella
	if config.CapellaForkEpoch.Uint64() > 0 {
		slotsUntilCapella := beacon.Slot(
			config.CapellaForkEpoch.Uint64(),
		) * testnet.Spec().SLOTS_PER_EPOCH
		testnet.WaitSlots(ctx, slotsUntilCapella)
	}

	// If there are any remaining validators that cannot withdraw yet, send
	// them now
	nonWithdrawableValidators := allValidators.NonWithdrawable()
	if len(nonWithdrawableValidators) > 0 {
		beaconClients := testnet.BeaconClients()
		for i := 0; i < len(nonWithdrawableValidators); i++ {
			b := beaconClients[i%len(beaconClients)]
			v := nonWithdrawableValidators[i]
			if err := v.SignSendBLSToExecutionChange(
				ctx,
				b,
				common.Address{byte(v.Index + 0x100)},
			); err != nil {
				t.Fatalf(
					"FAIL: Unable to submit bls-to-execution changes: %v",
					err,
				)
			}
		}
	} else {
		t.Logf("INFO: no validators left on BLS credentials")
	}

	// Wait for all BLS to execution to be included
	slotsForAllBlsInclusion := beacon.Slot(
		len(genesisNonWithdrawable)/int(
			testnet.Spec().MAX_BLS_TO_EXECUTION_CHANGES,
		) + 1,
	)
	testnet.WaitSlots(ctx, slotsForAllBlsInclusion)

	// Get the beacon state and verify the credentials were updated
	var versionedBeaconState *clients.VersionedBeaconStateResponse
	for _, bn := range testnet.BeaconClients().Running() {
		versionedBeaconState, err = bn.BeaconStateV2ByBlock(
			ctx,
			eth2api.BlockHead,
		)
		if err != nil || versionedBeaconState == nil {
			t.Logf("WARN: Unable to get latest beacon state: %v", err)
		}
	}
	if versionedBeaconState == nil {
		t.Fatalf(
			"FAIL: Unable to get latest beacon state from any client: %v",
			err,
		)
	}

	validators := versionedBeaconState.Validators()
	for _, v := range genesisNonWithdrawable {
		validator := validators[v.Index]
		credentials := validator.WithdrawalCredentials
		if !bytes.Equal(
			credentials[:1],
			[]byte{beacon.ETH1_ADDRESS_WITHDRAWAL_PREFIX},
		) {
			t.Fatalf(
				"FAIL: Withdrawal credential not updated for validator %d: %v",
				v.Index,
				credentials,
			)
		}
		if v.WithdrawAddress == nil {
			t.Fatalf(
				"FAIL: BLS-to-execution change was not sent for validator %d",
				v.Index,
			)
		}
		if !bytes.Equal(v.WithdrawAddress[:], credentials[12:]) {
			t.Fatalf(
				"FAIL: Incorrect withdrawal credential for validator %d: want=%x, got=%x",
				v.Index,
				v.WithdrawAddress,
				credentials[12:],
			)
		}
		t.Logf("INFO: Successful BLS to execution change: %s", credentials)
	}

	// Wait for all validators to withdraw
	slotsForAllPartialWithdrawals := beacon.Slot(
		len(validators) / int(testnet.Spec().MAX_WITHDRAWALS_PER_PAYLOAD),
	)
	slotCtx, cancel := testnet.Spec().
		SlotTimeoutContext(ctx, slotsForAllPartialWithdrawals+5+(beacon.Slot(config.CapellaForkEpoch.Uint64())*testnet.Spec().SLOTS_PER_EPOCH))
	defer cancel()
loop:
	for {
		select {
		case <-slotCtx.Done():
			t.Fatalf("FAIL: Timeout waiting on all accounts to withdraw")
		case <-time.After(time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second):
			// Print all info
			testnet.BeaconClients().Running().PrintStatus(slotCtx, t)

			// Check all accounts
			for _, ec := range testnet.ExecutionClients().Running() {
				if allAccountsWithdrawn, err := allValidators.Withdrawable().VerifyWithdrawnBalance(ctx, ec); err != nil {
					t.Fatalf("FAIL: %v", err)
				} else if allAccountsWithdrawn {
					t.Logf("INFO: All accounts have successfully withdrawn")
					break loop
				}
			}
		}
	}

	// Lastly check all clients are on the same head
	testnet.VerifyELHeads(ctx)

	// TODO: Should we wait for finalization every time?
}
