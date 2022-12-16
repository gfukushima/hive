package clients

import (
	"fmt"

	"github.com/ethereum/hive/hivesim"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
)

type ValidatorClient struct {
	T                *hivesim.T
	HiveClient       *hivesim.Client
	ClientType       string
	OptionsGenerator func([]*consensus_config.KeyDetails) ([]hivesim.StartOption, error)
	Keys             []*consensus_config.KeyDetails
}

func NewValidatorClient(
	t *hivesim.T,
	validatorDef *hivesim.ClientDefinition,
	optionsGenerator func([]*consensus_config.KeyDetails) ([]hivesim.StartOption, error),
	keys []*consensus_config.KeyDetails,
) *ValidatorClient {
	return &ValidatorClient{
		T:                t,
		ClientType:       validatorDef.Name,
		OptionsGenerator: optionsGenerator,
		Keys:             keys,
	}
}

func (vc *ValidatorClient) Start(extraOptions ...hivesim.StartOption) error {
	if vc.HiveClient != nil {
		return fmt.Errorf("client already started")
	}
	if len(vc.Keys) == 0 {
		vc.T.Logf("Skipping validator because it has 0 validator keys")
		return nil
	}
	vc.T.Logf("Starting client %s", vc.ClientType)
	opts, err := vc.OptionsGenerator(vc.Keys)
	if err != nil {
		return fmt.Errorf("unable to get start options: %v", err)
	}
	opts = append(opts, extraOptions...)

	vc.HiveClient = vc.T.StartClient(vc.ClientType, opts...)
	return nil
}

func (vc *ValidatorClient) Shutdown() error {
	if err := vc.T.Sim.StopClient(vc.T.SuiteID, vc.T.TestID, vc.HiveClient.Container); err != nil {
		return err
	}
	vc.HiveClient = nil
	return nil
}

func (vc *ValidatorClient) IsRunning() bool {
	return vc.HiveClient != nil
}

func (v *ValidatorClient) ContainsKey(pk [48]byte) bool {
	for _, k := range v.Keys {
		if k.ValidatorPubkey == pk {
			return true
		}
	}
	return false
}

type ValidatorClients []*ValidatorClient

// Return subset of clients that are currently running
func (all ValidatorClients) Running() ValidatorClients {
	res := make(ValidatorClients, 0)
	for _, vc := range all {
		if vc.IsRunning() {
			res = append(res, vc)
		}
	}
	return res
}
