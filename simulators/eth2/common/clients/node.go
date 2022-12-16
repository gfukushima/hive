package clients

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
	cg "github.com/ethereum/hive/simulators/eth2/common/chain_generators"
)

// Describe a node setup, which consists of:
// - Execution Client
// - Beacon Client
// - Validator Client
type NodeDefinition struct {
	ExecutionClient      string
	ConsensusClient      string
	ValidatorClient      string
	ValidatorShares      uint64
	ExecutionClientTTD   *big.Int
	BeaconNodeTTD        *big.Int
	TestVerificationNode bool
	DisableStartup       bool
	ChainGenerator       cg.ChainGenerator
	Chain                []*types.Block
	ExecutionSubnet      string
}

func (n *NodeDefinition) String() string {
	return fmt.Sprintf("%s-%s", n.ConsensusClient, n.ExecutionClient)
}

func (n *NodeDefinition) ExecutionClientName() string {
	return n.ExecutionClient
}

func (n *NodeDefinition) ConsensusClientName() string {
	return fmt.Sprintf("%s-bn", n.ConsensusClient)
}

func (n *NodeDefinition) ValidatorClientName() string {
	if n.ValidatorClient == "" {
		return fmt.Sprintf("%s-vc", n.ConsensusClient)
	}
	return fmt.Sprintf("%s-vc", n.ValidatorClient)
}

type NodeDefinitions []NodeDefinition

func (nodes NodeDefinitions) Shares() []uint64 {
	shares := make([]uint64, len(nodes))
	for i, n := range nodes {
		shares[i] = n.ValidatorShares
	}
	return shares
}

// A node bundles together:
// - Running Execution client
// - Running Beacon client
// - Running Validator client
// Contains a flag that marks a node that can be used to query
// test verification information.
type Node struct {
	T               *hivesim.T
	Index           int
	ExecutionClient *ExecutionClient
	BeaconClient    *BeaconClient
	ValidatorClient *ValidatorClient
	Verification    bool
}

// Starts all clients included in the bundle
func (cb *Node) Start(extraOptions ...hivesim.StartOption) error {
	cb.T.Logf("Starting validator client bundle %d", cb.Index)
	if cb.ExecutionClient != nil {
		if err := cb.ExecutionClient.Start(extraOptions...); err != nil {
			return err
		}
	} else {
		cb.T.Logf("No execution client started")
	}
	if cb.BeaconClient != nil {
		if err := cb.BeaconClient.Start(extraOptions...); err != nil {
			return err
		}
	} else {
		cb.T.Logf("No beacon client started")
	}
	if cb.ValidatorClient != nil {
		if err := cb.ValidatorClient.Start(extraOptions...); err != nil {
			return err
		}
	} else {
		cb.T.Logf("No validator client started")
	}
	return nil
}

func (cb *Node) Shutdown() error {
	if err := cb.ExecutionClient.Shutdown(); err != nil {
		return err
	}
	if err := cb.BeaconClient.Shutdown(); err != nil {
		return err
	}
	if err := cb.ValidatorClient.Shutdown(); err != nil {
		return err
	}
	return nil
}

type Nodes []*Node

// Return all execution clients, even the ones not currently running
func (cbs Nodes) ExecutionClients() ExecutionClients {
	en := make(ExecutionClients, 0)
	for _, cb := range cbs {
		if cb.ExecutionClient != nil {
			en = append(en, cb.ExecutionClient)
		}
	}
	return en
}

// Return all proxy pointers, even the ones not currently running
func (cbs Nodes) Proxies() Proxies {
	ps := make(Proxies, 0)
	for _, cb := range cbs {
		if cb.ExecutionClient != nil {
			ps = append(ps, cb.ExecutionClient.proxy)
		}
	}
	return ps
}

// Return all beacon clients, even the ones not currently running
func (cbs Nodes) BeaconClients() BeaconClients {
	bn := make(BeaconClients, 0)
	for _, cb := range cbs {
		if cb.BeaconClient != nil {
			bn = append(bn, cb.BeaconClient)
		}
	}
	return bn
}

// Return all validator clients, even the ones not currently running
func (cbs Nodes) ValidatorClients() ValidatorClients {
	vc := make(ValidatorClients, 0)
	for _, cb := range cbs {
		if cb.ValidatorClient != nil {
			vc = append(vc, cb.ValidatorClient)
		}
	}
	return vc
}

// Return subset of nodes which are marked as verification nodes
func (all Nodes) VerificationNodes() Nodes {
	// If none is set as verification, then all are verification nodes
	var any bool
	for _, cb := range all {
		if cb.Verification {
			any = true
			break
		}
	}
	if !any {
		return all
	}

	res := make(Nodes, 0)
	for _, cb := range all {
		if cb.Verification {
			res = append(res, cb)
		}
	}
	return res
}

func (cbs Nodes) RemoveNodeAsVerifier(id int) error {
	if id >= len(cbs) {
		return fmt.Errorf("node %d does not exist", id)
	}
	var any bool
	for _, cb := range cbs {
		if cb.Verification {
			any = true
			break
		}
	}
	if any {
		cbs[id].Verification = false
	} else {
		// If no node is set as verifier, we will set all other nodes as verifiers then
		for i := range cbs {
			cbs[i].Verification = (i != id)
		}
	}
	return nil
}
