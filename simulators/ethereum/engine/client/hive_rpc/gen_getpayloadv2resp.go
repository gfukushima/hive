// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package hive_rpc

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/beacon"
)

var _ = (*getPayloadV2ResponseMarshaling)(nil)

// MarshalJSON marshals as JSON.
func (g GetPayloadV2Response) MarshalJSON() ([]byte, error) {
	type GetPayloadV2Response struct {
		ExecutableData beacon.ExecutableData `json:"executionPayload"    gencodec:"required"`
		BlockValue     *hexutil.Big          `json:"blockValue"    gencodec:"required"`
	}
	var enc GetPayloadV2Response
	enc.ExecutableData = g.ExecutableData
	enc.BlockValue = (*hexutil.Big)(g.BlockValue)
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (g *GetPayloadV2Response) UnmarshalJSON(input []byte) error {
	type GetPayloadV2Response struct {
		ExecutableData *beacon.ExecutableData `json:"executionPayload"    gencodec:"required"`
		BlockValue     *hexutil.Big           `json:"blockValue"    gencodec:"required"`
	}
	var dec GetPayloadV2Response
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.ExecutableData == nil {
		return errors.New("missing required field 'executionPayload' for GetPayloadV2Response")
	}
	g.ExecutableData = *dec.ExecutableData
	if dec.BlockValue == nil {
		return errors.New("missing required field 'blockValue' for GetPayloadV2Response")
	}
	g.BlockValue = (*big.Int)(dec.BlockValue)
	return nil
}
