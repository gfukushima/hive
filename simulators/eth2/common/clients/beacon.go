package clients

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/beaconapi"
	"github.com/protolambda/eth2api/client/debugapi"
	"github.com/protolambda/eth2api/client/nodeapi"
	"github.com/protolambda/zrnt/eth2/beacon/bellatrix"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

const (
	PortBeaconTCP    = 9000
	PortBeaconUDP    = 9000
	PortBeaconAPI    = 4000
	PortBeaconGRPC   = 4001
	PortMetrics      = 8080
	PortValidatorAPI = 5000
)

type BeaconClient struct {
	T                *hivesim.T
	HiveClient       *hivesim.Client
	ClientType       string
	OptionsGenerator func() ([]hivesim.StartOption, error)
	API              *eth2api.Eth2HttpClient
	genesisTime      common.Timestamp
	spec             *common.Spec
	index            int
}

func NewBeaconClient(
	t *hivesim.T,
	beaconDef *hivesim.ClientDefinition,
	optionsGenerator func() ([]hivesim.StartOption, error),
	genesisTime common.Timestamp,
	spec *common.Spec,
	index int,
) *BeaconClient {
	return &BeaconClient{
		T:                t,
		ClientType:       beaconDef.Name,
		OptionsGenerator: optionsGenerator,
		genesisTime:      genesisTime,
		spec:             spec,
		index:            index,
	}
}

func (bn *BeaconClient) Start(extraOptions ...hivesim.StartOption) error {
	if bn.HiveClient != nil {
		return fmt.Errorf("client already started")
	}
	bn.T.Logf("Starting client %s", bn.ClientType)
	opts, err := bn.OptionsGenerator()
	if err != nil {
		return fmt.Errorf("unable to get start options: %v", err)
	}
	opts = append(opts, extraOptions...)

	bn.HiveClient = bn.T.StartClient(bn.ClientType, opts...)
	bn.API = &eth2api.Eth2HttpClient{
		Addr:  fmt.Sprintf("http://%s:%d", bn.HiveClient.IP, PortBeaconAPI),
		Cli:   &http.Client{},
		Codec: eth2api.JSONCodec{},
	}
	return nil
}

func (bn *BeaconClient) Shutdown() error {
	if err := bn.T.Sim.StopClient(bn.T.SuiteID, bn.T.TestID, bn.HiveClient.Container); err != nil {
		return err
	}
	bn.HiveClient = nil
	return nil
}

func (bn *BeaconClient) IsRunning() bool {
	return bn.HiveClient != nil
}

func (bn *BeaconClient) ENR(parentCtx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(parentCtx, time.Second*10)
	defer cancel()
	var out eth2api.NetworkIdentity
	if err := nodeapi.Identity(ctx, bn.API, &out); err != nil {
		return "", err
	}
	fmt.Printf("p2p addrs: %v\n", out.P2PAddresses)
	fmt.Printf("peer id: %s\n", out.PeerID)
	return out.ENR, nil
}

func (bn *BeaconClient) P2PAddr(parentCtx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(parentCtx, time.Second*10)
	defer cancel()
	var out eth2api.NetworkIdentity
	if err := nodeapi.Identity(ctx, bn.API, &out); err != nil {
		return "", err
	}
	return out.P2PAddresses[0], nil
}

func (bn *BeaconClient) EnodeURL() (string, error) {
	return "", errors.New(
		"beacon node does not have an discv4 Enode URL, use ENR or multi-address instead",
	)
}

// Beacon API wrappers
func (bn *BeaconClient) BlockV2(
	parentCtx context.Context,
	blockId eth2api.BlockId,
) (*eth2api.VersionedSignedBeaconBlock, error) {
	var (
		versionedBlock = new(eth2api.VersionedSignedBeaconBlock)
		exists         bool
		err            error
	)
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	exists, err = beaconapi.BlockV2(ctx, bn.API, blockId, versionedBlock)
	if !exists {
		return nil, fmt.Errorf("endpoint not found on beacon client")
	}
	return versionedBlock, err
}

type BlockV2OptimisticResponse struct {
	Version             string `json:"version"`
	ExecutionOptimistic bool   `json:"execution_optimistic"`
}

func (bn *BeaconClient) BlockIsOptimistic(
	parentCtx context.Context,
	blockId eth2api.BlockId,
) (bool, error) {
	var (
		blockOptResp = new(BlockV2OptimisticResponse)
		exists       bool
		err          error
	)
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	exists, err = eth2api.SimpleRequest(
		ctx,
		bn.API,
		eth2api.FmtGET("/eth/v2/beacon/blocks/%s", blockId.BlockId()),
		blockOptResp,
	)
	if !exists {
		return false, fmt.Errorf("endpoint not found on beacon client")
	}
	return blockOptResp.ExecutionOptimistic, err
}

func (bn *BeaconClient) BlockHeader(
	parentCtx context.Context,
	blockId eth2api.BlockId,
) (*eth2api.BeaconBlockHeaderAndInfo, error) {
	var (
		headInfo = new(eth2api.BeaconBlockHeaderAndInfo)
		exists   bool
		err      error
	)
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	exists, err = beaconapi.BlockHeader(ctx, bn.API, blockId, headInfo)
	if !exists {
		return nil, fmt.Errorf("endpoint not found on beacon client")
	}
	return headInfo, err
}

func (bn *BeaconClient) StateValidator(
	parentCtx context.Context,
	stateId eth2api.StateId,
	validatorId eth2api.ValidatorId,
) (*eth2api.ValidatorResponse, error) {
	var (
		stateValidatorResponse = new(eth2api.ValidatorResponse)
		exists                 bool
		err                    error
	)
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	exists, err = beaconapi.StateValidator(
		ctx,
		bn.API,
		stateId,
		validatorId,
		stateValidatorResponse,
	)
	if !exists {
		return nil, fmt.Errorf("endpoint not found on beacon client")
	}
	return stateValidatorResponse, err
}

func (bn *BeaconClient) StateFinalityCheckpoints(
	parentCtx context.Context,
	stateId eth2api.StateId,
) (*eth2api.FinalityCheckpoints, error) {
	var (
		finalityCheckpointsResponse = new(eth2api.FinalityCheckpoints)
		exists                      bool
		err                         error
	)
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	exists, err = beaconapi.FinalityCheckpoints(
		ctx,
		bn.API,
		stateId,
		finalityCheckpointsResponse,
	)
	if !exists {
		return nil, fmt.Errorf("endpoint not found on beacon client")
	}
	return finalityCheckpointsResponse, err
}

func (bn *BeaconClient) BlockFinalityCheckpoints(
	parentCtx context.Context,
	blockId eth2api.BlockId,
) (*eth2api.FinalityCheckpoints, error) {
	var (
		headInfo                    *eth2api.BeaconBlockHeaderAndInfo
		finalityCheckpointsResponse *eth2api.FinalityCheckpoints
		err                         error
	)
	headInfo, err = bn.BlockHeader(parentCtx, blockId)
	if err != nil {
		return nil, err
	}
	finalityCheckpointsResponse, err = bn.StateFinalityCheckpoints(
		parentCtx,
		eth2api.StateIdRoot(headInfo.Header.Message.StateRoot),
	)
	if err != nil {
		// Try again using slot number
		return bn.StateFinalityCheckpoints(
			parentCtx,
			eth2api.StateIdSlot(headInfo.Header.Message.Slot),
		)
	}
	return finalityCheckpointsResponse, err
}

func (bn *BeaconClient) BeaconStateV2(
	parentCtx context.Context,
	stateId eth2api.StateId,
) (*eth2api.VersionedBeaconState, error) {
	var (
		versionedBeaconStateResponse = new(eth2api.VersionedBeaconState)
		exists                       bool
		err                          error
	)
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	exists, err = debugapi.BeaconStateV2(
		ctx,
		bn.API,
		stateId,
		versionedBeaconStateResponse,
	)
	if !exists {
		return nil, fmt.Errorf("endpoint not found on beacon client")
	}
	return versionedBeaconStateResponse, err
}

func (bn *BeaconClient) StateValidatorBalances(
	parentCtx context.Context,
	stateId eth2api.StateId,
	validatorIds []eth2api.ValidatorId,
) ([]eth2api.ValidatorBalanceResponse, error) {
	var (
		stateValidatorBalanceResponse = make(
			[]eth2api.ValidatorBalanceResponse,
			0,
		)
		exists bool
		err    error
	)
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	exists, err = beaconapi.StateValidatorBalances(
		ctx,
		bn.API,
		stateId,
		validatorIds,
		&stateValidatorBalanceResponse,
	)
	if !exists {
		return nil, fmt.Errorf("endpoint not found on beacon client")
	}
	return stateValidatorBalanceResponse, err
}

type BeaconClients []*BeaconClient

// Return subset of clients that are currently running
func (all BeaconClients) Running() BeaconClients {
	res := make(BeaconClients, 0)
	for _, bc := range all {
		if bc.IsRunning() {
			res = append(res, bc)
		}
	}
	return res
}

// Returns comma-separated ENRs of all running beacon nodes
func (beacons BeaconClients) ENRs(parentCtx context.Context) (string, error) {
	if len(beacons) == 0 {
		return "", nil
	}
	enrs := make([]string, 0)
	for _, bn := range beacons {
		if bn.IsRunning() {
			enr, err := bn.ENR(parentCtx)
			if err != nil {
				return "", err
			}
			enrs = append(enrs, enr)
		}
	}
	return strings.Join(enrs, ","), nil
}

// Returns comma-separated P2PAddr of all running beacon nodes
func (beacons BeaconClients) P2PAddrs(
	parentCtx context.Context,
) (string, error) {
	if len(beacons) == 0 {
		return "", nil
	}
	staticPeers := make([]string, 0)
	for _, bn := range beacons {
		if bn.IsRunning() {
			p2p, err := bn.P2PAddr(parentCtx)
			if err != nil {
				return "", err
			}
			staticPeers = append(staticPeers, p2p)
		}
	}
	return strings.Join(staticPeers, ","), nil
}

func (b *BeaconClient) WaitForExecutionPayload(
	ctx context.Context,
) (ethcommon.Hash, error) {
	fmt.Printf("Waiting for execution payload on beacon %d\n", b.index)
	slotDuration := time.Duration(b.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)

	for {
		select {
		case <-ctx.Done():
			return ethcommon.Hash{}, ctx.Err()
		case <-timer.C:
			realTimeSlot := b.spec.TimeToSlot(
				common.Timestamp(time.Now().Unix()),
				b.genesisTime,
			)
			var headInfo eth2api.BeaconBlockHeaderAndInfo
			if exists, err := beaconapi.BlockHeader(ctx, b.API, eth2api.BlockHead, &headInfo); err != nil {
				return ethcommon.Hash{}, fmt.Errorf(
					"WaitForExecutionPayload: failed to poll head: %v",
					err,
				)
			} else if !exists {
				return ethcommon.Hash{}, fmt.Errorf("WaitForExecutionPayload: failed to poll head: !exists")
			}
			if !headInfo.Canonical {
				continue
			}

			var versionedBlock eth2api.VersionedSignedBeaconBlock
			if exists, err := beaconapi.BlockV2(ctx, b.API, eth2api.BlockIdRoot(headInfo.Root), &versionedBlock); err != nil {
				// some clients fail to return the head because the client recently re-org'd,
				// keep checking anyway...
				continue
			} else if !exists {
				return ethcommon.Hash{}, fmt.Errorf("WaitForExecutionPayload: block not found")
			}
			var execution ethcommon.Hash
			if versionedBlock.Version == "bellatrix" {
				block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
				copy(
					execution[:],
					block.Message.Body.ExecutionPayload.BlockHash[:],
				)
			}
			zero := ethcommon.Hash{}
			fmt.Printf(
				"WaitForExecutionPayload: beacon %d: slot=%d, realTimeSlot=%d, head=%s, exec=%s\n",
				b.index,
				headInfo.Header.Message.Slot,
				realTimeSlot,
				utils.Shorten(headInfo.Root.String()),
				utils.Shorten(execution.Hex()),
			)
			if !bytes.Equal(execution[:], zero[:]) {
				return execution, nil
			}
		}
	}
}

func (b *BeaconClient) WaitForOptimisticState(
	ctx context.Context,
	blockID eth2api.BlockId,
	optimistic bool,
) (*eth2api.BeaconBlockHeaderAndInfo, error) {
	fmt.Printf("Waiting for optimistic sync on beacon %d\n", b.index)
	slotDuration := time.Duration(b.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			var headOptStatus BlockV2OptimisticResponse
			if exists, err := eth2api.SimpleRequest(ctx, b.API, eth2api.FmtGET("/eth/v2/beacon/blocks/%s", blockID.BlockId()), &headOptStatus); err != nil {
				// Block still not synced
				continue
			} else if !exists {
				// Block still not synced
				continue
			}
			if headOptStatus.ExecutionOptimistic != optimistic {
				continue
			}
			// Return the block
			var blockInfo eth2api.BeaconBlockHeaderAndInfo
			if exists, err := beaconapi.BlockHeader(ctx, b.API, blockID, &blockInfo); err != nil {
				return nil, fmt.Errorf(
					"WaitForExecutionPayload: failed to poll block: %v",
					err,
				)
			} else if !exists {
				return nil, fmt.Errorf("WaitForExecutionPayload: failed to poll block: !exists")
			}
			return &blockInfo, nil
		}
	}
}

func (bn *BeaconClient) GetLatestExecutionBeaconBlock(
	parentCtx context.Context,
) (*bellatrix.SignedBeaconBlock, error) {
	headInfo, err := bn.BlockHeader(parentCtx, eth2api.BlockHead)
	if err != nil {
		return nil, fmt.Errorf("failed to poll head: %v", err)
	}
	for slot := headInfo.Header.Message.Slot; slot > 0; slot-- {
		versionedBlock, err := bn.BlockV2(parentCtx, eth2api.BlockIdSlot(slot))
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve block: %v", err)
		}
		if versionedBlock.Version != "bellatrix" {
			return nil, nil
		}
		block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
		payload := block.Message.Body.ExecutionPayload
		if ethcommon.BytesToHash(payload.BlockHash[:]) != (ethcommon.Hash{}) {
			return block, nil
		}
	}
	return nil, nil
}

func (bn *BeaconClient) GetFirstExecutionBeaconBlock(
	parentCtx context.Context,
) (*bellatrix.SignedBeaconBlock, error) {
	lastSlot := bn.spec.TimeToSlot(
		common.Timestamp(time.Now().Unix()),
		bn.genesisTime,
	)
	for slot := common.Slot(0); slot <= lastSlot; slot++ {
		versionedBlock, err := bn.BlockV2(parentCtx, eth2api.BlockIdSlot(slot))
		if err != nil {
			continue
		}
		if versionedBlock.Version != "bellatrix" {
			continue
		}
		block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
		payload := block.Message.Body.ExecutionPayload
		if ethcommon.BytesToHash(payload.BlockHash[:]) != (ethcommon.Hash{}) {
			return block, nil
		}
	}
	return nil, nil
}

func (bn *BeaconClient) GetBeaconBlockByExecutionHash(
	parentCtx context.Context,
	hash ethcommon.Hash,
) (*bellatrix.SignedBeaconBlock, error) {
	headInfo, err := bn.BlockHeader(parentCtx, eth2api.BlockHead)
	if err != nil {
		return nil, fmt.Errorf("failed to poll head: %v", err)
	}
	for slot := int(headInfo.Header.Message.Slot); slot > 0; slot -= 1 {
		versionedBlock, err := bn.BlockV2(parentCtx, eth2api.BlockIdSlot(slot))
		if err != nil {
			continue
		}
		if versionedBlock.Version != "bellatrix" {
			// Block can't contain an executable payload, and we are not going to find it going backwards, so return.
			return nil, nil
		}
		block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
		payload := block.Message.Body.ExecutionPayload
		if bytes.Equal(payload.BlockHash[:], hash[:]) {
			return block, nil
		}
	}
	return nil, nil
}

func (b BeaconClients) GetBeaconBlockByExecutionHash(
	parentCtx context.Context,
	hash ethcommon.Hash,
) (*bellatrix.SignedBeaconBlock, error) {
	for _, bn := range b {
		block, err := bn.GetBeaconBlockByExecutionHash(parentCtx, hash)
		if err != nil || block != nil {
			return block, err
		}
	}
	return nil, nil
}
